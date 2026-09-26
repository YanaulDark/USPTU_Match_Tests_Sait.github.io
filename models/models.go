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
