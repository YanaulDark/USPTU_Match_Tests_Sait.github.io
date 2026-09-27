package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"lecture-platform/storage"
)

type SocialHandler struct {
	Store *storage.Storage
}

// AddFriend — POST /api/friends
// body: {"email": "friend@example.com"}
func (h *SocialHandler) AddFriend(w http.ResponseWriter, r *http.Request) {
	me := currentUserID(r)
	if me == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	target, err := h.Store.GetUserByEmail(body.Email)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if target.Role != "student" {
		http.Error(w, "not a student", http.StatusBadRequest)
		return
	}

	if err := h.Store.AddFriend(me, target.ID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{
		"id":   target.ID,
		"name": target.Name,
	})
}

// ListFriends — GET /api/friends
func (h *SocialHandler) ListFriends(w http.ResponseWriter, r *http.Request) {
	me := currentUserID(r)
	ids := h.Store.ListFriends(me)
	out := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		if u, err := h.Store.GetUser(id); err == nil {
			out = append(out, map[string]any{
				"id":   u.ID,
				"name": u.Name,
				"role": u.Role,
			})
		}
	}
	writeJSON(w, out)
}

// SubscribeTeacher — POST /api/subscriptions
// body: {"email": "teacher@example.com"}
func (h *SocialHandler) SubscribeTeacher(w http.ResponseWriter, r *http.Request) {
	me := currentUserID(r)

	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	target, err := h.Store.GetUserByEmail(body.Email)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if target.Role != "teacher" {
		http.Error(w, "not a teacher", http.StatusBadRequest)
		return
	}

	if err := h.Store.Subscribe(me, target.ID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{
		"id":   target.ID,
		"name": target.Name,
	})
}

// ListSubscriptions — GET /api/subscriptions
func (h *SocialHandler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	me := currentUserID(r)
	ids := h.Store.ListSubscriptions(me)
	out := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		if u, err := h.Store.GetUser(id); err == nil {
			out = append(out, map[string]any{
				"id":   u.ID,
				"name": u.Name,
			})
		}
	}
	writeJSON(w, out)
}

// UnsubscribeTeacher — DELETE /api/subscriptions/<teacherID>
func (h *SocialHandler) UnsubscribeTeacher(w http.ResponseWriter, r *http.Request) {
	me := currentUserID(r)
	teacherID := strings.TrimPrefix(r.URL.Path, "/api/subscriptions/")
	if teacherID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	h.Store.Unsubscribe(me, teacherID)
	w.WriteHeader(http.StatusNoContent)
}

// TeacherLectures — GET /api/teachers/<id>/lectures
// Возвращает лекции преподавателя (только просмотр для подписанных).
func (h *SocialHandler) TeacherLectures(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/teachers/")
	path = strings.TrimSuffix(path, "/lectures")
	teacherID := strings.Trim(path, "/")
	if teacherID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	writeJSON(w, h.Store.ListLecturesByOwner(teacherID))
}

// currentUserID извлекает id пользователя из cookie (без проверки роли).
func currentUserID(r *http.Request) string {
	c, err := r.Cookie("token")
	if err != nil {
		return ""
	}
	claims, err := ParseClaims(c.Value)
	if err != nil {
		return ""
	}
	uid, _ := claims["sub"].(string)
	return uid
}
// RemoveFriend — DELETE /api/friends/<friendID>
func (h *SocialHandler) RemoveFriend(w http.ResponseWriter, r *http.Request) {
	me := currentUserID(r)
	if me == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	friendID := strings.TrimPrefix(r.URL.Path, "/api/friends/")
	if friendID == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	h.Store.RemoveFriend(me, friendID)
	w.WriteHeader(http.StatusNoContent)
}
