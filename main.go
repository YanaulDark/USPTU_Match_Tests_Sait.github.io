package main

import (
	"log"
	"net/http"
	"os"

	"lecture-platform/handlers"
	"lecture-platform/storage"
)

func main() {
	store := storage.New()

	dataDir := "./static"
	_ = os.MkdirAll(dataDir+"/uploads", 0o755)

	auth := &handlers.AuthHandler{Store: store}
	lec := &handlers.LectureHandler{Store: store}
	pdf := &handlers.PDFHandler{Store: store, DataDir: dataDir}
	qst := &handlers.QuestionHandler{Store: store}

	mux := http.NewServeMux()

	// Страницы
	mux.HandleFunc("/", page("templates/login.html"))
	mux.HandleFunc("/teacher", page("templates/teacher.html"))
	mux.HandleFunc("/student", page("templates/student.html"))
	mux.HandleFunc("/workshop", page("templates/workshop.html"))

	// Аутентификация
	mux.HandleFunc("/api/register", auth.Register)
	mux.HandleFunc("/api/login", auth.Login)

	// Лекции: список (GET) и загрузка PDF (POST)
	mux.HandleFunc("/api/lectures", handlers.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			pdf.UploadPDF(w, r)
		case http.MethodGet:
			lec.List(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}, "teacher"))

	// Одиночная лекция: GET (с вопросами) и DELETE (удаление)
	mux.HandleFunc("/api/lectures/", handlers.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			pdf.DeleteLecture(w, r)
			return
		}
		lec.Get(w, r)
	}, "teacher"))

	// SSE-поток — без авторизации, чтобы студент мог подписаться сразу после QR
	mux.HandleFunc("/api/lectures/stream/", lec.Stream)

	// Вопросы
	mux.HandleFunc("/api/questions", handlers.AuthMiddleware(qst.Create, "teacher"))
	mux.HandleFunc("/api/questions/", handlers.AuthMiddleware(qst.ListByLecture, ""))

	// Ответы и статистика
	mux.HandleFunc("/api/answers", handlers.AuthMiddleware(qst.SubmitAnswer, "student"))
	mux.HandleFunc("/api/stats/", handlers.AuthMiddleware(qst.Stats, "teacher"))

	// Статика и PDF
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	mux.HandleFunc("/uploads/", pdf.ServePDF)

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func page(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, path)
	}
}

