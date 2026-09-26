package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"lecture-platform/models"
	"lecture-platform/storage"
)

// jwtKey — секрет для подписи JWT. В продакшене — из переменной окружения.
var jwtKey = []byte("CHANGE_ME_SECRET_KEY")

// AuthHandler обслуживает регистрацию, вход и выдачу данных текущего пользователя.
type AuthHandler struct {
	Store *storage.Storage
}

type creds struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// Register — POST /api/register
// Принимает JSON {name, email, password, role}, хэширует пароль и создаёт пользователя.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var c creds
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if c.Name == "" || c.Email == "" || c.Password == "" {
		http.Error(w, "missing fields", http.StatusBadRequest)
		return
	}
	if c.Role != "teacher" && c.Role != "student" {
		http.Error(w, "invalid role", http.StatusBadRequest)
		return
	}

	hash, err := storage.HashPassword(c.Password)
	if err != nil {
		log.Printf("register: hash error: %v", err)
		http.Error(w, "hash error", http.StatusInternalServerError)
		return
	}

	u := &models.User{
		ID:       uuid.NewString(),
		Name:     c.Name,
		Email:    c.Email,
		Password: hash,
		Role:     c.Role,
	}

	if err := h.Store.Users.CreateUser(u); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	// Сразу логиним после регистрации — ставим cookie.
	issueToken(w, u)
	writeJSON(w, map[string]any{
		"id":   u.ID,
		"role": u.Role,
		"name": u.Name,
	})
}

// Login — POST /api/login
// Принимает JSON {email, password}, проверяет пароль и выдаёт JWT-cookie.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var c creds
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	u, err := h.Store.Users.GetUserByEmail(c.Email)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if !storage.CheckPassword(u.Password, c.Password) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	issueToken(w, u)
	writeJSON(w, map[string]any{
		"id":   u.ID,
		"role": u.Role,
		"name": u.Name,
	})
}

// Me — GET /api/login/me
// Возвращает данные текущего пользователя на основе cookie.
// Регистрируется БЕЗ AuthMiddleware — сам разбирает токен.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("token")
	if err != nil {
		http.Error(w, "no cookie", http.StatusUnauthorized)
		return
	}

	tok, err := jwt.Parse(c.Value, func(t *jwt.Token) (any, error) {
		return jwtKey, nil
	})
	if err != nil || !tok.Valid {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		http.Error(w, "bad claims", http.StatusUnauthorized)
		return
	}

	uid, _ := claims["sub"].(string)
	if uid == "" {
		http.Error(w, "bad sub", http.StatusUnauthorized)
		return
	}

	u, err := h.Store.Users.GetUser(uid)
	if err != nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	writeJSON(w, map[string]any{
		"id":    u.ID,
		"name":  u.Name,
		"email": u.Email,
		"role":  u.Role,
	})
}

// AuthMiddleware проверяет JWT из cookie и (опционально) роль.
// requiredRole == "" — роль не важна.
func AuthMiddleware(next http.HandlerFunc, requiredRole string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("token")
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		tok, err := jwt.Parse(c.Value, func(t *jwt.Token) (any, error) {
			return jwtKey, nil
		})
		if err != nil || !tok.Valid {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		claims, ok := tok.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if requiredRole != "" {
			role, _ := claims["role"].(string)
			if role != requiredRole {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}

		next(w, r)
	}
}

// ─── helpers ───

// issueToken подписывает JWT и ставит его в HttpOnly-cookie.
func issueToken(w http.ResponseWriter, u *models.User) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  u.ID,
		"role": u.Role,
		"exp":  time.Now().Add(72 * time.Hour).Unix(),
	})
	signed, err := token.SignedString(jwtKey)
	if err != nil {
		log.Printf("issueToken: sign error: %v", err)
		http.Error(w, "token error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    signed,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   72 * 60 * 60,
	})
}

// ParseClaims разбирает JWT и возвращает claims. Если токен невалиден — ошибка.
func ParseClaims(token string) (jwt.MapClaims, error) {
	tok, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		return jwtKey, nil
	})
	if err != nil || !tok.Valid {
		return nil, err
	}
	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return nil, err
	}
	return claims, nil
}

// writeJSON — общий хелпер, используется во всех хендлерах.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: %v", err)
	}
}
