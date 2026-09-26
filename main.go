package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"strings"

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
	soc := &handlers.SocialHandler{Store: store}

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

	// одиночная лекция: GET (всем авторизованным) и DELETE (только преподавателю)
	mux.HandleFunc("/api/lectures/", handlers.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			// только преподаватель может удалять
			c, err := r.Cookie("token")
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			claims, err := handlers.ParseClaims(c.Value)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if role, _ := claims["role"].(string); role != "teacher" {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			pdf.DeleteLecture(w, r)
		default:
			lec.Get(w, r)
		}
	}, ""))

	// SSE-поток без авторизации
	mux.HandleFunc("/api/lectures/stream/", lec.Stream)

	// вопросы
	mux.HandleFunc("/api/questions", handlers.AuthMiddleware(qst.Create, "teacher"))
	mux.HandleFunc("/api/questions/", handlers.AuthMiddleware(qst.ListByLecture, ""))

	// ответы и статистика
	mux.HandleFunc("/api/answers", handlers.AuthMiddleware(qst.SubmitAnswer, "student"))
	mux.HandleFunc("/api/stats/", handlers.AuthMiddleware(qst.Stats, "teacher"))

	// ─── друзья ───
	mux.HandleFunc("/api/friends", handlers.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			soc.ListFriends(w, r)
		case http.MethodPost:
			soc.AddFriend(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}, "student"))

	mux.HandleFunc("/api/friends/", handlers.AuthMiddleware(soc.RemoveFriend, "student"))

	// ─── подписки на преподавателей ───
	mux.HandleFunc("/api/subscriptions", handlers.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			soc.ListSubscriptions(w, r)
		case http.MethodPost:
			soc.SubscribeTeacher(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}, "student"))

	mux.HandleFunc("/api/subscriptions/", handlers.AuthMiddleware(soc.UnsubscribeTeacher, "student"))

	// ─── лекции преподавателя (для студентов) ───
	mux.HandleFunc("/api/teachers/", handlers.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/lectures") {
			soc.TeacherLectures(w, r)
			return
		}
		http.NotFound(w, r)
	}, ""))

	// ─── статика и PDF ───
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	mux.HandleFunc("/uploads/", pdf.ServePDF)

	// ─── показ локальных IP при старте ───
	ifaces, _ := net.InterfaceAddrs()
	for _, a := range ifaces {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ip4 := ipnet.IP.To4(); ip4 != nil {
				log.Printf("Local:   http://%s:8080", ip4.String())
			}
		}
	}
	log.Println("Local:   http://localhost:8080")
	log.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func page(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, path)
	}
}
