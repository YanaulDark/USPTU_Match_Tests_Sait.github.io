package models

import "time"

type User struct {
    ID       string `json:"id"`
    Name     string `json:"name"`
    Email    string `json:"email"`
    Password string `json:"-"`
    Role     string `json:"role"` // "teacher" | "student"
}

type Lecture struct {
    ID        string    `json:"id"`
    Title     string    `json:"title"`
    OwnerID   string    `json:"owner_id"`
    PDFPath   string    `json:"pdf_path"`
    CreatedAt time.Time `json:"created_at"`
    Active    bool      `json:"active"`
}

type Question struct {
    ID        string    `json:"id"`
    LectureID string    `json:"lecture_id"`
    Text      string    `json:"text"`
    Options   []string  `json:"options"`
    Correct   int       `json:"correct"`
    Slide     int       `json:"slide"`
    CreatedAt time.Time `json:"created_at"`
}

type Answer struct {
    ID         string    `json:"id"`
    QuestionID string    `json:"question_id"`
    StudentID  string    `json:"student_id"`
    Choice     int       `json:"choice"`
    Correct    bool      `json:"correct"`
    CreatedAt  time.Time `json:"created_at"`
}
// Friendship — связь между двумя студентами.
// Храним как пару (ownerID, friendID) — ownerID добавил friendID.
type Friendship struct {
	OwnerID  string    `json:"owner_id"`
	FriendID string    `json:"friend_id"`
	Created  time.Time `json:"created_at"`
}

// Subscription — подписка студента на преподавателя.
type Subscription struct {
	StudentID string    `json:"student_id"`
	TeacherID string    `json:"teacher_id"`
	Created   time.Time `json:"created_at"`
}

// QuizProgress — текущее состояние прохождения квиза конкретным студентом.
type QuizProgress struct {
	StudentID  string    `json:"student_id"`
	LectureID  string    `json:"lecture_id"`
	Score      int       `json:"score"`
	Total      int       `json:"total"`
	Index      int       `json:"index"`       // текущий вопрос (0-based)
	Finished   bool      `json:"finished"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// SessionState — глобальное состояние квиз-сессии для лекции.
// Храним активный ли квиз, на каком вопросе сейчас все, и т.п.
type SessionState struct {
	LectureID    string    `json:"lecture_id"`
	Active       bool      `json:"active"`
	CurrentSlide int       `json:"current_slide"`  // вопрос привязан к слайду
	StartedAt    time.Time `json:"started_at"`
}

// QuizQuestion — вопрос для квиза. Не привязан к слайду, идёт строго по порядку.
type QuizQuestion struct {
	ID        string    `json:"id"`
	LectureID string    `json:"lecture_id"`
	Order     int       `json:"order"`     // порядковый номер (0, 1, 2, ...)
	Text      string    `json:"text"`
	Options   []string  `json:"options"`
	Correct   int       `json:"correct"`
	CreatedAt time.Time `json:"created_at"`
}
