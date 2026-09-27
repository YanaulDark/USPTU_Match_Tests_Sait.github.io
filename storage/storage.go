package storage

import (
	"errors"
	"sort"
	"sync"
	"time"

	"lecture-platform/models"
)

// ErrNotFound — общая ошибка «нет такой записи».
var ErrNotFound = errors.New("not found")

// Storage — хранилище лекций, вопросов и ответов.
// Пользователи живут в UserStore (см. storage/users_file.go).
type Storage struct {
	mu        sync.RWMutex
	Users     *UserStore
	Lectures  map[string]*models.Lecture
	Questions map[string][]*models.Question
	Answers   map[string][]*models.Answer

	friends       map[string]map[string]bool // ownerID → set friendID
	subscriptions map[string]map[string]bool // studentID → set teacherID
	progress map[string]*models.QuizProgress // key: studentID + ":" + lectureID
	sessions map[string]*models.SessionState // key: lectureID
	quizQuestions map[string][]*models.QuizQuestion // key: lectureID
}


func (s *Storage) GetProgress(studentID, lectureID string) *models.QuizProgress {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := studentID + ":" + lectureID
	if p, ok := s.progress[key]; ok {
		return p
	}
	return &models.QuizProgress{
		StudentID: studentID,
		LectureID: lectureID,
		Index:     0,
	}
}

func (s *Storage) SaveProgress(p *models.QuizProgress) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.UpdatedAt = time.Now()
	s.progress[p.StudentID+":"+p.LectureID] = p
}

func (s *Storage) GetSession(lectureID string) *models.SessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if st, ok := s.sessions[lectureID]; ok {
		return st
	}
	return &models.SessionState{LectureID: lectureID, Active: false}
}

func (s *Storage) SaveSession(st *models.SessionState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[st.LectureID] = st
}

// Leaderboard — топ студентов по баллам для лекции.
func (s *Storage) Leaderboard(lectureID string) []map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	type entry struct {
		ID    string
		Name  string
		Score int
	}
	var entries []entry
	for _, p := range s.progress {
		if p.LectureID != lectureID || !p.Finished {
			continue
		}
		name := "Студент"
		if u, err := s.Users.GetUser(p.StudentID); err == nil {
			name = u.Name
		}
		entries = append(entries, entry{p.StudentID, name, p.Score})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Score > entries[j].Score
	})
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		out = append(out, map[string]any{
			"student_id": e.ID,
			"name":       e.Name,
			"score":      e.Score,
		})
	}
	return out
}
// ─── Друзья ───

func (s *Storage) AddFriend(ownerID, friendID string) error {
	if ownerID == friendID {
		return errors.New("cannot add yourself")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.friends[ownerID] == nil {
		s.friends[ownerID] = make(map[string]bool)
	}
	s.friends[ownerID][friendID] = true
	if s.friends[friendID] == nil {
		s.friends[friendID] = make(map[string]bool)
	}
	s.friends[friendID][ownerID] = true // взаимная дружба
	return nil
}

func (s *Storage) ListFriends(ownerID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0)
	for id := range s.friends[ownerID] {
		out = append(out, id)
	}
	return out
}

func (s *Storage) RemoveFriend(ownerID, friendID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.friends[ownerID], friendID)
	delete(s.friends[friendID], ownerID)
}

func (s *Storage) AddQuizQuestion(q *models.QuizQuestion) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.quizQuestions[q.LectureID]
	q.Order = len(list)
	s.quizQuestions[q.LectureID] = append(list, q)
}

func (s *Storage) ListQuizQuestions(lectureID string) []*models.QuizQuestion {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := s.quizQuestions[lectureID]
	if list == nil {
		return []*models.QuizQuestion{}
	}
	return list
}

func (s *Storage) DeleteQuizQuestion(lectureID, questionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.quizQuestions[lectureID]
	out := make([]*models.QuizQuestion, 0, len(list))
	for _, q := range list {
		if q.ID != questionID {
			out = append(out, q)
		}
	}
	// переиндексация order
	for i, q := range out {
		q.Order = i
	}
	s.quizQuestions[lectureID] = out
}

// ─── Подписки на преподавателей ───

func (s *Storage) Subscribe(studentID, teacherID string) error {
	if studentID == teacherID {
		return errors.New("cannot subscribe to yourself")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.subscriptions[studentID] == nil {
		s.subscriptions[studentID] = make(map[string]bool)
	}
	s.subscriptions[studentID][teacherID] = true
	return nil
}

func (s *Storage) Unsubscribe(studentID, teacherID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subscriptions[studentID], teacherID)
}

func (s *Storage) ListSubscriptions(studentID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0)
	for id := range s.subscriptions[studentID] {
		out = append(out, id)
	}
	return out
}
// New создаёт Storage и открывает файл пользователей.
// usersPath — путь к users.json, keyHex — ключ шифрования email.
func New(usersPath, keyHex string) (*Storage, error) {
	us, err := NewUserStore(usersPath, keyHex)
	if err != nil {
		return nil, err
	}
	return &Storage{
		Users:     us,
		Lectures:  make(map[string]*models.Lecture),
		Questions: make(map[string][]*models.Question),
		Answers:   make(map[string][]*models.Answer),

		friends:       make(map[string]map[string]bool),
		subscriptions: make(map[string]map[string]bool),
		progress: make(map[string]*models.QuizProgress),
		sessions: make(map[string]*models.SessionState),
		quizQuestions: make(map[string][]*models.QuizQuestion),
	}, nil
}

// ─── Лекции ───

func (s *Storage) SaveLecture(l *models.Lecture) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Lectures[l.ID] = l
}

func (s *Storage) GetLecture(id string) (*models.Lecture, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.Lectures[id]
	if !ok {
		return nil, ErrNotFound
	}
	return l, nil
}

func (s *Storage) ListLecturesByOwner(ownerID string) []*models.Lecture {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*models.Lecture, 0)
	for _, l := range s.Lectures {
		if l.OwnerID == ownerID {
			out = append(out, l)
		}
	}
	return out
}

func (s *Storage) DeleteLecture(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Lectures, id)
	delete(s.Questions, id)
}

// ─── Вопросы ───

func (s *Storage) AddQuestion(q *models.Question) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Questions[q.LectureID] = append(s.Questions[q.LectureID], q)
}

func (s *Storage) ListQuestions(lectureID string) []*models.Question {
	s.mu.RLock()
	defer s.mu.RUnlock()
	qs := s.Questions[lectureID]
	if qs == nil {
		return []*models.Question{}
	}
	return qs
}

// ─── Ответы ───

func (s *Storage) AddAnswer(a *models.Answer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Answers[a.QuestionID] = append(s.Answers[a.QuestionID], a)
}

func (s *Storage) ListAnswers(questionID string) []*models.Answer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	as := s.Answers[questionID]
	if as == nil {
		return []*models.Answer{}
	}
	return as
}
