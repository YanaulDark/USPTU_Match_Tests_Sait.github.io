package handlers

import (
	"net/http"
	"strings"
	"time"

	"lecture-platform/storage"
)

type LectureHandler struct {
	Store *storage.Storage
}

// List — GET /api/lectures?owner_id=<id>
func (h *LectureHandler) List(w http.ResponseWriter, r *http.Request) {
	owner := r.URL.Query().Get("owner_id")
	writeJSON(w, h.Store.ListLecturesByOwner(owner))
}

// Get — GET /api/lectures/<id> — возвращает лекцию вместе с вопросами.
func (h *LectureHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/lectures/")
	l, err := h.Store.GetLecture(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	qs := h.Store.ListQuestions(id)
	writeJSON(w, map[string]any{"lecture": l, "questions": qs})
}

// Stream — SSE-поток изменений по лекции (смена слайда/активация вопроса).
func (h *LectureHandler) Stream(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/lectures/stream/")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	last := ""
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			l, err := h.Store.GetLecture(id)
			if err != nil {
				return
			}
			cur := `{"active":` + boolStr(l.Active) + `}`
			if cur != last {
				_, _ = w.Write([]byte("data: " + cur + "\n\n"))
				flusher.Flush()
				last = cur
			}
		}
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
