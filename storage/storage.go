package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"time"

	_ "modernc.org/sqlite"

	"golang.org/x/crypto/bcrypt"

	"lecture-platform/models"
)

var ErrNotFound = errors.New("not found")

// Storage — обёртка над SQLite.
type Storage struct {
	db  *sql.DB
	aead cipher.AEAD
}

// ─── Инициализация ───

func New(dbPath, keyHex string) (*Storage, error) {
	key, err := decodeKey(keyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	s := &Storage{db: db, aead: aead}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Storage) Close() error {
	return s.db.Close()
}

func (s *Storage) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id            TEXT PRIMARY KEY,
			name          TEXT NOT NULL,
			email         TEXT NOT NULL UNIQUE,
			email_enc     TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			role          TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS lectures (
			id         TEXT PRIMARY KEY,
			title      TEXT NOT NULL,
			owner_id   TEXT NOT NULL,
			pdf_path   TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			active     INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS questions (
			id         TEXT PRIMARY KEY,
			lecture_id TEXT NOT NULL,
			text       TEXT NOT NULL,
			options    TEXT NOT NULL,
			correct    INTEGER NOT NULL,
			slide      INTEGER NOT NULL,
			created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS quiz_questions (
			id         TEXT PRIMARY KEY,
			lecture_id TEXT NOT NULL,
			order_num  INTEGER NOT NULL,
			text       TEXT NOT NULL,
			options    TEXT NOT NULL,
			correct    INTEGER NOT NULL,
			created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS answers (
			id          TEXT PRIMARY KEY,
			question_id TEXT NOT NULL,
			student_id  TEXT NOT NULL,
			choice      INTEGER NOT NULL,
			correct     INTEGER NOT NULL,
			created_at  DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS friends (
			owner_id  TEXT NOT NULL,
			friend_id TEXT NOT NULL,
			PRIMARY KEY (owner_id, friend_id)
		)`,
		`CREATE TABLE IF NOT EXISTS subscriptions (
			student_id TEXT NOT NULL,
			teacher_id TEXT NOT NULL,
			PRIMARY KEY (student_id, teacher_id)
		)`,
		`CREATE TABLE IF NOT EXISTS quiz_progress (
			student_id TEXT NOT NULL,
			lecture_id TEXT NOT NULL,
			score      INTEGER NOT NULL DEFAULT 0,
			total      INTEGER NOT NULL DEFAULT 0,
			idx        INTEGER NOT NULL DEFAULT 0,
			finished   INTEGER NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL,
			PRIMARY KEY (student_id, lecture_id)
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			lecture_id TEXT PRIMARY KEY,
			active     INTEGER NOT NULL DEFAULT 0,
			started_at DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_lectures_owner ON lectures(owner_id)`,
		`CREATE INDEX IF NOT EXISTS idx_questions_lecture ON questions(lecture_id)`,
		`CREATE INDEX IF NOT EXISTS idx_quiz_questions_lecture ON quiz_questions(lecture_id)`,
		`CREATE INDEX IF NOT EXISTS idx_answers_question ON answers(question_id)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// ─── Пользователи ───

func (s *Storage) CreateUser(u *models.User) error {
	enc, err := s.encrypt(u.Email)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO users(id, name, email, email_enc, password_hash, role) VALUES(?,?,?,?,?,?)`,
		u.ID, u.Name, u.Email, enc, u.Password, u.Role,
	)
	return err
}

func (s *Storage) GetUser(id string) (*models.User, error) {
	row := s.db.QueryRow(
		`SELECT id, name, email, password_hash, role FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func (s *Storage) GetUserByEmail(email string) (*models.User, error) {
	row := s.db.QueryRow(
		`SELECT id, name, email, password_hash, role FROM users WHERE email = ?`, email)
	return scanUser(row)
}

func scanUser(row *sql.Row) (*models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// ─── Лекции ───

func (s *Storage) SaveLecture(l *models.Lecture) error {
	_, err := s.db.Exec(
		`INSERT INTO lectures(id, title, owner_id, pdf_path, created_at, active)
		 VALUES(?,?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET title=excluded.title, owner_id=excluded.owner_id,
		   pdf_path=excluded.pdf_path, active=excluded.active`,
		l.ID, l.Title, l.OwnerID, l.PDFPath, l.CreatedAt, boolToInt(l.Active),
	)
	return err
}

func (s *Storage) GetLecture(id string) (*models.Lecture, error) {
	row := s.db.QueryRow(
		`SELECT id, title, owner_id, pdf_path, created_at, active FROM lectures WHERE id = ?`, id)
	var l models.Lecture
	var active int
	err := row.Scan(&l.ID, &l.Title, &l.OwnerID, &l.PDFPath, &l.CreatedAt, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	l.Active = active != 0
	return &l, nil
}

func (s *Storage) ListLecturesByOwner(ownerID string) []*models.Lecture {
	rows, err := s.db.Query(
		`SELECT id, title, owner_id, pdf_path, created_at, active
		 FROM lectures WHERE owner_id = ? ORDER BY created_at DESC`, ownerID)
	if err != nil {
		return []*models.Lecture{}
	}
	defer rows.Close()
	out := make([]*models.Lecture, 0)
	for rows.Next() {
		var l models.Lecture
		var active int
		if err := rows.Scan(&l.ID, &l.Title, &l.OwnerID, &l.PDFPath, &l.CreatedAt, &active); err != nil {
			continue
		}
		l.Active = active != 0
		out = append(out, &l)
	}
	return out
}

func (s *Storage) DeleteLecture(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tx.Exec(`DELETE FROM questions WHERE lecture_id = ?`, id)
	tx.Exec(`DELETE FROM quiz_questions WHERE lecture_id = ?`, id)
	tx.Exec(`DELETE FROM sessions WHERE lecture_id = ?`, id)
	tx.Exec(`DELETE FROM lectures WHERE id = ?`, id)
	return tx.Commit()
}

// ─── Вопросы мастерской ───

func (s *Storage) AddQuestion(q *models.Question) error {
	opts := encodeOptions(q.Options)
	_, err := s.db.Exec(
		`INSERT INTO questions(id, lecture_id, text, options, correct, slide, created_at)
		 VALUES(?,?,?,?,?,?,?)`,
		q.ID, q.LectureID, q.Text, opts, q.Correct, q.Slide, q.CreatedAt,
	)
	return err
}

func (s *Storage) ListQuestions(lectureID string) []*models.Question {
	rows, err := s.db.Query(
		`SELECT id, lecture_id, text, options, correct, slide, created_at
		 FROM questions WHERE lecture_id = ? ORDER BY created_at ASC`, lectureID)
	if err != nil {
		return []*models.Question{}
	}
	defer rows.Close()
	out := make([]*models.Question, 0)
	for rows.Next() {
		q := &models.Question{}
		var opts string
		if err := rows.Scan(&q.ID, &q.LectureID, &q.Text, &opts, &q.Correct, &q.Slide, &q.CreatedAt); err != nil {
			continue
		}
		q.Options = decodeOptions(opts)
		out = append(out, q)
	}
	return out
}

// ─── Вопросы квиза ───

func (s *Storage) AddQuizQuestion(q *models.QuizQuestion) error {
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM quiz_questions WHERE lecture_id = ?`, q.LectureID).Scan(&count)
	q.Order = count
	opts := encodeOptions(q.Options)
	_, err := s.db.Exec(
		`INSERT INTO quiz_questions(id, lecture_id, order_num, text, options, correct, created_at)
		 VALUES(?,?,?,?,?,?,?)`,
		q.ID, q.LectureID, q.Order, q.Text, opts, q.Correct, q.CreatedAt,
	)
	return err
}

func (s *Storage) ListQuizQuestions(lectureID string) []*models.QuizQuestion {
	rows, err := s.db.Query(
		`SELECT id, lecture_id, order_num, text, options, correct, created_at
		 FROM quiz_questions WHERE lecture_id = ? ORDER BY order_num ASC`, lectureID)
	if err != nil {
		return []*models.QuizQuestion{}
	}
	defer rows.Close()
	out := make([]*models.QuizQuestion, 0)
	for rows.Next() {
		q := &models.QuizQuestion{}
		var opts string
		if err := rows.Scan(&q.ID, &q.LectureID, &q.Order, &q.Text, &opts, &q.Correct, &q.CreatedAt); err != nil {
			continue
		}
		q.Options = decodeOptions(opts)
		out = append(out, q)
	}
	return out
}

func (s *Storage) DeleteQuizQuestion(lectureID, questionID string) error {
	_, err := s.db.Exec(
		`DELETE FROM quiz_questions WHERE id = ? AND lecture_id = ?`, questionID, lectureID)
	return err
}

// ─── Ответы ───

func (s *Storage) AddAnswer(a *models.Answer) error {
	_, err := s.db.Exec(
		`INSERT INTO answers(id, question_id, student_id, choice, correct, created_at)
		 VALUES(?,?,?,?,?,?)`,
		a.ID, a.QuestionID, a.StudentID, a.Choice, boolToInt(a.Correct), a.CreatedAt,
	)
	return err
}

func (s *Storage) ListAnswers(questionID string) []*models.Answer {
	rows, err := s.db.Query(
		`SELECT id, question_id, student_id, choice, correct, created_at
		 FROM answers WHERE question_id = ?`, questionID)
	if err != nil {
		return []*models.Answer{}
	}
	defer rows.Close()
	out := make([]*models.Answer, 0)
	for rows.Next() {
		a := &models.Answer{}
		var c int
		if err := rows.Scan(&a.ID, &a.QuestionID, &a.StudentID, &a.Choice, &c, &a.CreatedAt); err != nil {
			continue
		}
		a.Correct = c != 0
		out = append(out, a)
	}
	return out
}

// ─── Друзья ───

func (s *Storage) AddFriend(ownerID, friendID string) error {
	if ownerID == friendID {
		return errors.New("cannot add yourself")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tx.Exec(`INSERT OR IGNORE INTO friends(owner_id, friend_id) VALUES(?,?)`, ownerID, friendID)
	tx.Exec(`INSERT OR IGNORE INTO friends(owner_id, friend_id) VALUES(?,?)`, friendID, ownerID)
	return tx.Commit()
}

func (s *Storage) ListFriends(ownerID string) []string {
	rows, err := s.db.Query(`SELECT friend_id FROM friends WHERE owner_id = ?`, ownerID)
	if err != nil {
		return []string{}
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var id string
		rows.Scan(&id)
		out = append(out, id)
	}
	return out
}

func (s *Storage) RemoveFriend(ownerID, friendID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tx.Exec(`DELETE FROM friends WHERE owner_id = ? AND friend_id = ?`, ownerID, friendID)
	tx.Exec(`DELETE FROM friends WHERE owner_id = ? AND friend_id = ?`, friendID, ownerID)
	return tx.Commit()
}

// ─── Подписки ───

func (s *Storage) Subscribe(studentID, teacherID string) error {
	if studentID == teacherID {
		return errors.New("cannot subscribe to yourself")
	}
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO subscriptions(student_id, teacher_id) VALUES(?,?)`,
		studentID, teacherID)
	return err
}

func (s *Storage) Unsubscribe(studentID, teacherID string) error {
	_, err := s.db.Exec(
		`DELETE FROM subscriptions WHERE student_id = ? AND teacher_id = ?`,
		studentID, teacherID)
	return err
}

func (s *Storage) ListSubscriptions(studentID string) []string {
	rows, err := s.db.Query(`SELECT teacher_id FROM subscriptions WHERE student_id = ?`, studentID)
	if err != nil {
		return []string{}
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var id string
		rows.Scan(&id)
		out = append(out, id)
	}
	return out
}

// ─── Прогресс квиза ───

func (s *Storage) GetProgress(studentID, lectureID string) *models.QuizProgress {
	row := s.db.QueryRow(
		`SELECT score, total, idx, finished, updated_at
		 FROM quiz_progress WHERE student_id = ? AND lecture_id = ?`,
		studentID, lectureID)
	p := &models.QuizProgress{StudentID: studentID, LectureID: lectureID}
	var fin int
	err := row.Scan(&p.Score, &p.Total, &p.Index, &fin, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return p
	}
	p.Finished = fin != 0
	return p
}

func (s *Storage) SaveProgress(p *models.QuizProgress) error {
	p.UpdatedAt = time.Now()
	_, err := s.db.Exec(
		`INSERT INTO quiz_progress(student_id, lecture_id, score, total, idx, finished, updated_at)
		 VALUES(?,?,?,?,?,?,?)
		 ON CONFLICT(student_id, lecture_id) DO UPDATE SET
		   score=excluded.score, total=excluded.total, idx=excluded.idx,
		   finished=excluded.finished, updated_at=excluded.updated_at`,
		p.StudentID, p.LectureID, p.Score, p.Total, p.Index, boolToInt(p.Finished), p.UpdatedAt,
	)
	return err
}

// ─── Сессии ───

func (s *Storage) GetSession(lectureID string) *models.SessionState {
	row := s.db.QueryRow(
		`SELECT active, started_at FROM sessions WHERE lecture_id = ?`, lectureID)
	st := &models.SessionState{LectureID: lectureID}
	var a int
	err := row.Scan(&a, &st.StartedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return st
	}
	st.Active = a != 0
	return st
}

func (s *Storage) SaveSession(st *models.SessionState) error {
	_, err := s.db.Exec(
		`INSERT INTO sessions(lecture_id, active, started_at) VALUES(?,?,?)
		 ON CONFLICT(lecture_id) DO UPDATE SET active=excluded.active, started_at=excluded.started_at`,
		st.LectureID, boolToInt(st.Active), st.StartedAt,
	)
	return err
}

// ─── Лидерборд ───

type LeaderboardEntry struct {
	StudentID string `json:"student_id"`
	Name      string `json:"name"`
	Score     int    `json:"score"`
}

func (s *Storage) Leaderboard(lectureID string) []LeaderboardEntry {
	rows, err := s.db.Query(
		`SELECT qp.student_id, COALESCE(u.name, 'Студент'), qp.score
		 FROM quiz_progress qp
		 LEFT JOIN users u ON u.id = qp.student_id
		 WHERE qp.lecture_id = ? AND qp.score > 0
		 ORDER BY qp.score DESC LIMIT 50`, lectureID)
	if err != nil {
		return []LeaderboardEntry{}
	}
	defer rows.Close()
	out := make([]LeaderboardEntry, 0)
	for rows.Next() {
		var e LeaderboardEntry
		rows.Scan(&e.StudentID, &e.Name, &e.Score)
		out = append(out, e)
	}
	return out
}

// ─── Пароли ───

func HashPassword(plain string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(h), err
}

func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// ─── Шифрование email ───

func (s *Storage) encrypt(plain string) (string, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := s.aead.Seal(nil, nonce, []byte(plain), nil)
	blob := append(nonce, ct...)
	return base64.StdEncoding.EncodeToString(blob), nil
}

// ─── Утилиты ───

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func encodeOptions(opts []string) string {
	// простая сериализация через \x1f (unit separator) — быстрее и без экранирования
	return joinStrings(opts, "\x1f")
}

func decodeOptions(s string) []string {
	if s == "" {
		return []string{}
	}
	return splitString(s, '\x1f')
}

func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += sep + p
	}
	return out
}

func splitString(s string, sep rune) []string {
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == sep {
			out = append(out, cur)
			cur = ""
		} else {
			cur += string(r)
		}
	}
	out = append(out, cur)
	return out
}

func decodeKey(k string) ([]byte, error) {
	if len(k) == 64 {
		b, err := hexDecode(k)
		if err == nil {
			return b, nil
		}
	}
	if b, err := base64.StdEncoding.DecodeString(k); err == nil && len(b) == 32 {
		return b, nil
	}
	if len(k) > 0 {
		sum := sha256.Sum256([]byte(k))
		return sum[:], nil
	}
	return nil, errors.New("empty key")
}

func hexDecode(s string) ([]byte, error) {
	const hextable = "0123456789abcdef"
	out := make([]byte, len(s)/2)
	for i := 0; i < len(out); i++ {
		hi := indexByte(hextable, lower(s[2*i]))
		lo := indexByte(hextable, lower(s[2*i+1]))
		if hi < 0 || lo < 0 {
			return nil, errors.New("invalid hex")
		}
		out[i] = byte(hi<<4 | lo)
	}
	return out, nil
}

func lower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 32
	}
	return b
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
