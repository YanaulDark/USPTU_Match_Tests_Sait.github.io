package storage

import (
    "errors"
    "sync"

    "lecture-platform/models"
)

type Storage struct {
    mu        sync.RWMutex
    Users     map[string]*models.User
    UsersByEmail map[string]*models.User
    Lectures  map[string]*models.Lecture
    Questions map[string][]*models.Question
    Answers   map[string][]*models.Answer
}

func New() *Storage {
    return &Storage{
        Users:        make(map[string]*models.User),
        UsersByEmail: make(map[string]*models.User),
        Lectures:     make(map[string]*models.Lecture),
        Questions:    make(map[string][]*models.Question),
        Answers:      make(map[string][]*models.Answer),
    }
}

var ErrNotFound = errors.New("not found")

func (s *Storage) CreateUser(u *models.User) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    if _, ok := s.UsersByEmail[u.Email]; ok {
        return errors.New("email already exists")
    }
    s.Users[u.ID] = u
    s.UsersByEmail[u.Email] = u
    return nil
}

func (s *Storage) GetUserByEmail(email string) (*models.User, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    u, ok := s.UsersByEmail[email]
    if !ok {
        return nil, ErrNotFound
    }
    return u, nil
}

func (s *Storage) GetUser(id string) (*models.User, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    u, ok := s.Users[id]
    if !ok {
        return nil, ErrNotFound
    }
    return u, nil
}

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

func (s *Storage) DeleteLecture(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Lectures, id)
	delete(s.Questions, id)
}
