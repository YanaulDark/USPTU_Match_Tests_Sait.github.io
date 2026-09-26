package main

import (
	"log"
	"net/http"
	"os"

	"lecture-platform/handlers"
	"lecture-platform/storage"
)

func main() {
	// ─── ключ шифрования пользовательских данных ───
	key := os.Getenv("USERS_KEY")
	if key == "" {
		key = "dev-insecure-key-change-me"
		log.Println("WARNING: using default USERS_KEY, set USERS_KEY env var for production")
	}

	// ─── подготовка директорий ───
	if err := os.MkdirAll("./data", 0o700); err != nil {
		log.Fatal(err)
	}
	dataDir := "./static"
	if err := os.MkdirAll(dataDir+"/uploads", 0o755); err != nil {
		log.Fatal(err)
	}

	// ─── хранилище ───
	store, err := storage.New("./data/users.json", key)
	if err != nil {
		log.Fatalf("cannot open user store: %v", err)
	}

	// ─── хендлеры ───
	auth := &handlers.AuthHandler{Store: store}
	lec := &handlers.LectureHandler{Store: store}
	pdf := &handlers.PDFHandler{Store: store, DataDir: dataDir}
	qst := &handlers.QuestionHandler{Store: store}

	// ─── маршруты ───
	mux := http.NewServeMux()

	// страницы
	mux.HandleFunc("/", page("templates/login.html"))
	mux.HandleFunc("/teacher", page("templates/teacher.html"))
	mux.HandleFunc("/student", page("templates/student.html"))
	mux.HandleFunc("/workshop", page("templates/workshop.html"))

	// аутентификация
	mux.HandleFunc("/api/register", auth.Register)
	mux.HandleFunc("/api/login", auth.Login)
	mux.HandleFunc("/api/login/me", auth.Me)

	// лекции: список (GET) и загрузка PDF (POST)
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

	// одиночная лекция: GET и DELETE
	mux.HandleFunc("/api/lectures/", handlers.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			pdf.DeleteLecture(w, r)
			return
		}
		lec.Get(w, r)
	}, "teacher"))

	// SSE-поток без авторизации
	mux.HandleFunc("/api/lectures/stream/", lec.Stream)

	// вопросы
	mux.HandleFunc("/api/questions", handlers.AuthMiddleware(qst.Create, "teacher"))
	mux.HandleFunc("/api/questions/", handlers.AuthMiddleware(qst.ListByLecture, ""))

	// ответы и статистика
	mux.HandleFunc("/api/answers", handlers.AuthMiddleware(qst.SubmitAnswer, "student"))
	mux.HandleFunc("/api/stats/", handlers.AuthMiddleware(qst.Stats, "teacher"))

	// статика и PDF
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
