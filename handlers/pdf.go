package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lecture-platform/models"
	"lecture-platform/storage"
)

// PDFHandler инкапсулирует всё, что связано с загрузкой,
// хранением и отдачей PDF-файлов лекций.
type PDFHandler struct {
	Store   *storage.Storage
	DataDir string // корневая директория данных, например ./static
}

// UploadPDF принимает multipart/form-data с полем "pdf" и метаданными
// (title, owner_id), сохраняет файл на диск и создаёт запись Lecture.
//
// POST /api/lectures  (multipart/form-data)
//   pdf      — файл (обязательно, только .pdf)
//   title    — название лекции
//   owner_id — id преподавателя
func (h *PDFHandler) UploadPDF(w http.ResponseWriter, r *http.Request) {
	// Ограничим размер тела запроса 50 МБ.
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		http.Error(w, "file too large or bad form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("pdf")
	if err != nil {
		http.Error(w, "missing pdf file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if !isPDF(header.Filename, header.Header.Get("Content-Type")) {
		http.Error(w, "only application/pdf allowed", http.StatusBadRequest)
		return
	}

	// Проверяем, что файл действительно начинается с сигнатуры %PDF-
	// (первый байт не читаем, чтобы поток остался пригодным для копирования).
	head := make([]byte, 5)
	n, _ := io.ReadFull(file, head)
	if n < 5 || string(head) != "%PDF-" {
		http.Error(w, "invalid pdf signature", http.StatusBadRequest)
		return
	}
	// Возвращаем указатель к началу файла.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		http.Error(w, "seek error", http.StatusInternalServerError)
		return
	}

	id := newID()
	dir := filepath.Join(h.DataDir, "uploads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	destPath := filepath.Join(dir, id+".pdf")

	out, err := os.Create(destPath)
	if err != nil {
		http.Error(w, "cannot create file", http.StatusInternalServerError)
		return
	}
	defer out.Close()

	written, err := io.Copy(out, file)
	if err != nil {
		_ = os.Remove(destPath)
		http.Error(w, "cannot write file", http.StatusInternalServerError)
		return
	}
	if written == 0 {
		_ = os.Remove(destPath)
		http.Error(w, "empty file", http.StatusBadRequest)
		return
	}

	lecture := &models.Lecture{
		ID:        id,
		Title:     strings.TrimSpace(r.FormValue("title")),
		OwnerID:   r.FormValue("owner_id"),
		PDFPath:   "/uploads/" + id + ".pdf",
		CreatedAt: time.Now(),
		Active:    false,
	}
	if lecture.Title == "" {
		lecture.Title = header.Filename
	}

	h.Store.SaveLecture(lecture)
	writeJSON(w, lecture)
}

// ServePDF отдаёт сохранённый PDF по URL /uploads/<id>.pdf.
// При отдаче выставляем корректный Content-Type и inline-диспозицию,
// чтобы браузер открывал файл во встроенном вьювере, а не скачивал.
func (h *PDFHandler) ServePDF(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/uploads/")
	// Защита от path traversal.
	name = filepath.Base(name)
	if name == "" || name == "." || name == "/" {
		http.NotFound(w, r)
		return
	}
	if !strings.HasSuffix(strings.ToLower(name), ".pdf") {
		http.NotFound(w, r)
		return
	}

	fullPath := filepath.Join(h.DataDir, "uploads", name)
	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", name))
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, fullPath)
}

// DeleteLecture удаляет PDF-файл и запись о лекции.
// (Пригодится в интерфейсе преподавателя.)
//
// DELETE /api/lectures/<id>
func (h *PDFHandler) DeleteLecture(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/lectures/")
	if id == "" || strings.Contains(id, "/") {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	lec, err := h.Store.GetLecture(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	path := filepath.Join(h.DataDir, "uploads", filepath.Base(lec.PDFPath))
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		http.Error(w, "cannot delete file", http.StatusInternalServerError)
		return
	}
	h.Store.DeleteLecture(id)
	w.WriteHeader(http.StatusNoContent)
}

// ——— helpers ———

func isPDF(filename, mime string) bool {
	if strings.HasSuffix(strings.ToLower(filename), ".pdf") {
		return true
	}
	return strings.EqualFold(mime, "application/pdf")
}

// newID возвращает URL-safe hex-строку (16 байт → 32 символа).
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand на практике не падает; на всякий случай — детерминированный фолбэк.
		return hex.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	return hex.EncodeToString(b)
}
