package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"lecture-platform/models"
	"lecture-platform/storage"
)

type QuizEditorHandler struct {
	Store *storage.Storage
}

// CreateQuizQuestion — POST /api/quiz-questions
func (h *QuizEditorHandler) CreateQuizQuestion(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LectureID string   `json:"lecture_id"`
		Text      string   `json:"text"`
		Options   []string `json:"options"`
		Correct   int      `json:"correct"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.LectureID == "" || body.Text == "" || len(body.Options) < 2 {
		http.Error(w, "missing fields", http.StatusBadRequest)
		return
	}
	if body.Correct < 0 || body.Correct >= len(body.Options) {
		http.Error(w, "bad correct index", http.StatusBadRequest)
		return
	}

	q := &models.QuizQuestion{
		ID:        uuid.NewString(),
		LectureID: body.LectureID,
		Text:      body.Text,
		Options:   body.Options,
		Correct:   body.Correct,
		CreatedAt: time.Now(),
	}
	h.Store.AddQuizQuestion(q)
	writeJSON(w, q)
}

// ListQuizQuestions — GET /api/quiz-questions/<lectureID>
func (h *QuizEditorHandler) ListQuizQuestions(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/quiz-questions/")
	if id == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	writeJSON(w, h.Store.ListQuizQuestions(id))
}

// DeleteQuizQuestion — DELETE /api/quiz-questions/<lectureID>/<questionID>
func (h *QuizEditorHandler) DeleteQuizQuestion(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/quiz-questions/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	h.Store.DeleteQuizQuestion(parts[0], parts[1])
	w.WriteHeader(http.StatusNoContent)
}
