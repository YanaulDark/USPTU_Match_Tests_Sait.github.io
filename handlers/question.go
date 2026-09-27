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

type QuestionHandler struct {
    Store *storage.Storage
}

func (h *QuestionHandler) Create(w http.ResponseWriter, r *http.Request) {
    var q models.Question
    if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }
    q.ID = uuid.NewString()
    q.CreatedAt = time.Now()
    if err := h.Store.AddQuestion(&q); err != nil {
    http.Error(w, "cannot save question", http.StatusInternalServerError)
    return
		}
		writeJSON(w, q)
}

func (h *QuestionHandler) ListByLecture(w http.ResponseWriter, r *http.Request) {
    id := strings.TrimPrefix(r.URL.Path, "/api/questions/")
    writeJSON(w, h.Store.ListQuestions(id))
}

func (h *QuestionHandler) SubmitAnswer(w http.ResponseWriter, r *http.Request) {
    var a models.Answer
    if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }
    // найти вопрос и оценить
    lectureID := r.URL.Query().Get("lecture_id")
    qs := h.Store.ListQuestions(lectureID)
    for _, q := range qs {
        if q.ID == a.QuestionID {
            a.Correct = (q.Correct == a.Choice)
            break
        }
    }
    a.ID = uuid.NewString()
		a.CreatedAt = time.Now()
		if err := h.Store.AddAnswer(&a); err != nil {
    		http.Error(w, "cannot save answer", http.StatusInternalServerError)
    		return
		}
		writeJSON(w, map[string]any{"correct": a.Correct})
}

func (h *QuestionHandler) Stats(w http.ResponseWriter, r *http.Request) {
    qID := strings.TrimPrefix(r.URL.Path, "/api/stats/")
    ans := h.Store.ListAnswers(qID)
    counts := map[int]int{}
    for _, a := range ans {
        counts[a.Choice]++
    }
    writeJSON(w, map[string]any{"total": len(ans), "counts": counts})
}
