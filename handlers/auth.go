package handlers

import (
    "encoding/json"
    "net/http"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"
    "golang.org/x/crypto/bcrypt"

    "lecture-platform/models"
    "lecture-platform/storage"
)

var jwtKey = []byte("CHANGE_ME_SECRET_KEY")

type AuthHandler struct {
    Store *storage.Storage
}

type creds struct {
    Name     string `json:"name"`
    Email    string `json:"email"`
    Password string `json:"password"`
    Role     string `json:"role"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
    var c creds
    if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }
    if c.Role != "teacher" && c.Role != "student" {
        http.Error(w, "invalid role", http.StatusBadRequest)
        return
    }
    hash, _ := bcrypt.GenerateFromPassword([]byte(c.Password), bcrypt.DefaultCost)
    u := &models.User{
        ID:       uuid.NewString(),
        Name:     c.Name,
        Email:    c.Email,
        Password: string(hash),
        Role:     c.Role,
    }
    if err := h.Store.CreateUser(u); err != nil {
        http.Error(w, err.Error(), http.StatusConflict)
        return
    }
    writeJSON(w, map[string]any{"id": u.ID, "role": u.Role})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
    var c creds
    if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }
    u, err := h.Store.GetUserByEmail(c.Email)
    if err != nil {
        http.Error(w, "invalid credentials", http.StatusUnauthorized)
        return
    }
    if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(c.Password)) != nil {
        http.Error(w, "invalid credentials", http.StatusUnauthorized)
        return
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "sub":  u.ID,
        "role": u.Role,
        "exp":  time.Now().Add(72 * time.Hour).Unix(),
    })
    s, _ := token.SignedString(jwtKey)
    http.SetCookie(w, &http.Cookie{
        Name: "token", Value: s, Path: "/", HttpOnly: true,
    })
    writeJSON(w, map[string]any{"role": u.Role, "name": u.Name})
}

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
        claims := tok.Claims.(jwt.MapClaims)
        role, _ := claims["role"].(string)
        if requiredRole != "" && role != requiredRole {
            http.Error(w, "forbidden", http.StatusForbidden)
            return
        }
        next(w, r)
    }
}

func writeJSON(w http.ResponseWriter, v any) {
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(v)
}
