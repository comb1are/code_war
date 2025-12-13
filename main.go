package main

import (
	"bufio"
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
)

// Загрузка .env вручную (без внешних либ)
func loadEnv() {
	file, err := os.Open(".env")
	if err != nil {
		log.Println("Note: .env file not found, using defaults")
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			os.Setenv(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}
}

// Middleware для Basic Auth (Пароль)
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()

		// Берем пароль из .env
		adminPass := os.Getenv("ADMIN_PASSWORD")
		if adminPass == "" {
			adminPass = "admin"
		} // Дефолт

		// Сравниваем безопасно
		if !ok || subtle.ConstantTimeCompare([]byte(user), []byte("admin")) != 1 || subtle.ConstantTimeCompare([]byte(pass), []byte(adminPass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Teacher Area"`)
			http.Error(w, "Unauthorized", 401)
			return
		}
		next(w, r)
	}
}

func main() {
	loadEnv()

	room := newRoom("school-arena")
	go room.run()

	// 1. Публичные маршруты (для учеников)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(room, w, r)
	})

	// 2. Приватные маршруты (Админка) - защищены паролем
	http.HandleFunc("/admin", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "admin.html")
	}))

	// API тоже защищаем
	http.HandleFunc("/api/tasks", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			room.mu.RLock()
			json.NewEncoder(w).Encode(room.AllTasks)
			room.mu.RUnlock()
		} else if r.Method == "POST" {
			var task Task
			if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			room.SaveTask(task)
			w.WriteHeader(http.StatusOK)
		}
	}))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("System started on port :" + port)
	log.Println("Security Engine: Python AST (sandbox.py)")
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
