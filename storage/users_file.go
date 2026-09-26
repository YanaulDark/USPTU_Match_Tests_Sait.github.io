package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/bcrypt"

	"lecture-platform/models"
)

// UserStore — файловое хранилище пользователей.
// Пароли хэшируются bcrypt. Поле Email шифруется AES-GCM,
// чтобы продемонстрировать симметричное шифрование данных.
//
// Формат файла: JSON-массив записей. Каждая запись — структура UserRecord.
type UserStore struct {
	mu       sync.RWMutex
	path     string
	aead     cipher.AEAD
	users    map[string]*models.User // по id
	byEmail  map[string]*models.User // по email (открытый текст, для поиска)
}

// UserRecord — то, что реально пишется в файл.
// Email хранится в зашифрованном виде (base64 от nonce+ciphertext).
// EmailPlain — НЕ в файле, а только в памяти, для быстрого поиска.
type UserRecord struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	EmailEnc     string `json:"email_enc"`      // зашифрованный email
	PasswordHash string `json:"password_hash"`  // bcrypt-хэш
	Role         string `json:"role"`
}

// NewUserStore создаёт или открывает файл users.json.
// keyHex — 32 байта в hex (64 символа) для AES-256.
// Если файл не существует — создаётся пустой.
func NewUserStore(path, keyHex string) (*UserStore, error) {
	key, err := decodeKey(keyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	s := &UserStore{
		path:    path,
		aead:    aead,
		users:   make(map[string]*models.User),
		byEmail: make(map[string]*models.User),
	}

	if err := s.load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return s, nil
}

// ─── Публичный API, аналогичный старому Storage ───

func (s *UserStore) CreateUser(u *models.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byEmail[u.Email]; exists {
		return errors.New("email already exists")
	}
	s.users[u.ID] = u
	s.byEmail[u.Email] = u
	return s.persistLocked()
}

func (s *UserStore) GetUser(id string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

func (s *UserStore) GetUserByEmail(email string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.byEmail[email]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

// ─── Внутреннее: сериализация ───

func (s *UserStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}

	var records []UserRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return fmt.Errorf("corrupt users file: %w", err)
	}

	for _, r := range records {
		email, err := s.decrypt(r.EmailEnc)
		if err != nil {
			return fmt.Errorf("cannot decrypt email for %s: %w", r.ID, err)
		}
		u := &models.User{
			ID:       r.ID,
			Name:     r.Name,
			Email:    email,
			Password: r.PasswordHash,
			Role:     r.Role,
		}
		s.users[u.ID] = u
		s.byEmail[u.Email] = u
	}
	return nil
}

func (s *UserStore) persistLocked() error {
	records := make([]UserRecord, 0, len(s.users))
	for _, u := range s.users {
		enc, err := s.encrypt(u.Email)
		if err != nil {
			return err
		}
		records = append(records, UserRecord{
			ID:           u.ID,
			Name:         u.Name,
			EmailEnc:     enc,
			PasswordHash: u.Password,
			Role:         u.Role,
		})
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}

	// Атомарная запись: сначала во временный файл, потом rename.
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// ─── Криптография ───

// encrypt возвращает base64(nonce || ciphertext).
// nonce добавляется к шифротексту, потому что GCM требует уникального nonce на каждое шифрование.
func (s *UserStore) encrypt(plain string) (string, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := s.aead.Seal(nil, nonce, []byte(plain), nil)
	blob := append(nonce, ct...)
	return base64.StdEncoding.EncodeToString(blob), nil
}

func (s *UserStore) decrypt(b64 string) (string, error) {
	blob, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", err
	}
	ns := s.aead.NonceSize()
	if len(blob) < ns {
		return "", errors.New("ciphertext too short")
	}
	nonce, ct := blob[:ns], blob[ns:]
	plain, err := s.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// decodeKey принимает hex или base64 и приводит к 32 байтам.
// Для удобства: если длина 64 и валидный hex — декодируем hex.
// Иначе пробуем base64. Иначе — паникуем с понятной ошибкой.
func decodeKey(k string) ([]byte, error) {
	if len(k) == 64 {
		if b, err := hexDecode(k); err == nil {
			return b, nil
		}
	}
	if b, err := base64.StdEncoding.DecodeString(k); err == nil && len(b) == 32 {
		return b, nil
	}
	// Фолбэк: если задали произвольный пароль — берём SHA-256 от него.
	// Это удобно для дева, но в проде используйте настоящий 32-байтный ключ.
	if len(k) > 0 {
		sum := sha256.Sum256([]byte(k))
		return sum[:], nil
	}
	return nil, errors.New("empty key")
}

// hexDecode — маленький враппер, чтобы не тащить encoding/hex в общий import block.
func hexDecode(s string) ([]byte, error) {
	const hextable = "0123456789abcdef"
	out := make([]byte, len(s)/2)
	for i := 0; i < len(out); i++ {
		hi := indexByte(hextable, lower(s[2*i]))
		lo := indexByte(hextable, lower(s[2*i+1]))
		if hi < 0 || lo < 0 {
			return nil, errors.New("invalid hex")
		}
		out[i] = byte(hi<<4 | lo)
	}
	return out, nil
}

func lower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 32
	}
	return b
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// Хэширование пароля — обёртка, чтобы весь код работал единообразно.
func HashPassword(plain string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(h), err
}

func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

var _ = filepath.Join // заглушка, чтобы filepath был импортирован при рефакторинге
