package storage

import (
	"errors"
	"sync"

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
