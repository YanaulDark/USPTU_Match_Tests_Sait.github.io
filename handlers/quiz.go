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

type QuizHandler struct {
	Store *storage.Storage
}

// StartQuiz:
func (h *QuizHandler) StartQuiz(w http.ResponseWriter, r *http.Request) {
    id := strings.TrimPrefix(r.URL.Path, "/api/quiz/")
    id = strings.TrimSuffix(id, "/start")
    st := h.Store.GetSession(id)
    st.Active = true
    st.StartedAt = time.Now()
    if err := h.Store.SaveSession(st); err != nil {
        http.Error(w, "cannot save session", http.StatusInternalServerError)
        return
    }
    writeJSON(w, st)
}

// StopQuiz:
func (h *QuizHandler) StopQuiz(w http.ResponseWriter, r *http.Request) {
    id := strings.TrimPrefix(r.URL.Path, "/api/quiz/")
    id = strings.TrimSuffix(id, "/stop")
    st := h.Store.GetSession(id)
    st.Active = false
    if err := h.Store.SaveSession(st); err != nil {
        http.Error(w, "cannot save session", http.StatusInternalServerError)
        return
    }
    writeJSON(w, st)
}

// GetProgress — GET /api/quiz/<lectureID>/progress
func (h *QuizHandler) GetProgress(w http.ResponseWriter, r *http.Request) {
	me := currentUserID(r)
	if me == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/quiz/")
	id = strings.TrimSuffix(id, "/progress")

	prog := h.Store.GetProgress(me, id)
	qs := h.Store.ListQuizQuestions(id)   // ← было ListQuestions
	total := len(qs)

	var current *models.QuizQuestion       // ← было *models.Question
	if prog.Index < total {
		current = qs[prog.Index]
	}

	st := h.Store.GetSession(id)

	stats := map[int]int{}
	if current != nil {
		for _, a := range h.Store.ListAnswers(current.ID) {
			stats[a.Choice]++
		}
	}

	writeJSON(w, map[string]any{
		"progress": prog,
		"question": current,
		"total":    total,
		"stats":    stats,
		"session":  st,
		"finished": prog.Finished || prog.Index >= total,
	})
}

// Submit — POST /api/quiz/<lectureID>/answer
func (h *QuizHandler) Submit(w http.ResponseWriter, r *http.Request) {
	me := currentUserID(r)
	if me == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/quiz/")
	id = strings.TrimSuffix(id, "/answer")

	var body struct {
		QuestionID string `json:"question_id"`
		Choice     int    `json:"choice"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	qs := h.Store.ListQuizQuestions(id)
	var q *models.QuizQuestion
	for _, x := range qs {
		if x.ID == body.QuestionID {
			q = x
			break
		}
	}
	if q == nil {
		http.Error(w, "question not found", http.StatusNotFound)
		return
	}

	correct := q.Correct == body.Choice

	// ─── сохранить ответ ───
	if err := h.Store.AddAnswer(&models.Answer{
		ID:         uuid.NewString(),
		QuestionID: q.ID,
		StudentID:  me,
		Choice:     body.Choice,
		Correct:    correct,
		CreatedAt:  time.Now(),
	}); err != nil {
		http.Error(w, "cannot save answer", http.StatusInternalServerError)
		return
	}

	// ─── обновить прогресс ───
	prog := h.Store.GetProgress(me, id)
	prog.Total = len(qs)
	if correct {
		prog.Score++
	}
	prog.Index++
	if prog.Index >= len(qs) {
		prog.Finished = true
	}
	if err := h.Store.SaveProgress(prog); err != nil {
		http.Error(w, "cannot save progress", http.StatusInternalServerError)
		return
	}

	// ─── статистика для диаграммы ───
	stats := map[int]int{}
	for _, a := range h.Store.ListAnswers(q.ID) {
		stats[a.Choice]++
	}

	writeJSON(w, map[string]any{
		"correct":       correct,
		"correct_index": q.Correct,
		"stats":         stats,
		"progress":      prog,
		"finished":      prog.Finished,
	})
}

// Leaderboard — GET /api/quiz/<lectureID>/leaderboard
func (h *QuizHandler) Leaderboard(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/quiz/")
	id = strings.TrimSuffix(id, "/leaderboard")
	writeJSON(w, h.Store.Leaderboard(id))
}
