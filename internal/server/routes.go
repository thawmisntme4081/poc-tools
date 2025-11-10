package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
)

type Message struct {
	Content   string    `json:"content"`
	SessionId uuid.UUID `json:"session_id"`
}

func (s *Server) RegisterRoutes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// SPA
	r.Handle("/*", spaHandler())

	r.Route("/v1", func(r chi.Router) {

		// Websocket
		r.Get("/ws", s.websocketHandler)
		r.Post("/chat", s.chatHandler)

		// Users
		r.Route("/users", func(r chi.Router) {
			r.Post("/", s.CreateUserHandler)
			r.Get("/", s.GetUsersHandler)
			r.Get("/{id}", s.GetUserByIDHandler)
			r.Put("/{id}", s.UpdateUserHandler)
			r.Delete("/{id}", s.DeleteUserHandler)
		})

		// Threads
		// r.Route("/threads", func(r chi.Router) {
		// 	r.Post("/", s.CreateThreadHandler)
		// 	r.Get("/", s.GetThreadsHandler)
		// 	r.Get("/{id}", s.GetThreadByIDHandler)
		// 	r.Put("/{id}", s.UpdateThreadHandler)
		// 	r.Delete("/{id}", s.DeleteThreadHandler)
		// })

		// Messages
		// r.Route("/messages", func(r chi.Router) {
		// 	r.Post("/", s.CreateMessageHandler)
		// 	r.Get("/", s.GetMessagesHandler)
		// 	r.Get("/{id}", s.GetMessageByIDHandler)
		// 	r.Put("/{id}", s.UpdateMessageHandler)
		// 	r.Delete("/{id}", s.DeleteMessageHandler)
		// })
	})

	return r
}

func spaHandler() http.HandlerFunc {
	spaFS := os.DirFS("frontend/dist")
	return func(w http.ResponseWriter, r *http.Request) {
		// Any path not ending with a file extension is served as index.html
		if path.Ext(r.URL.Path) == "" || r.URL.Path == "/" {
			http.ServeFileFS(w, r, spaFS, "index.html")
			return
		}
		fmt.Println("Serving file", "path", path.Clean(r.URL.Path))
		f, err := spaFS.Open(strings.TrimPrefix(path.Clean(r.URL.Path), "/"))
		if err == nil {
			defer f.Close()
		}
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Content not found"))
			return
		}
		http.FileServer(http.FS(spaFS)).ServeHTTP(w, r)
	}
}

func (s *Server) chatHandler(w http.ResponseWriter, r *http.Request) {
	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Parse body
	var body Message
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fmt.Println("invalid body: %w", err)
		return
	}
	if body.Content == "" {
		fmt.Println("content is required")
		return
	}
	userID := uuid.Must(uuid.Parse("123e4567-e89b-12d3-a456-426614174000"))
	agentID := uuid.Must(uuid.Parse("01993ca8-a62e-79e3-995c-a46e25a4a2a2"))
	var sessionId *uuid.UUID
	if body.SessionId != uuid.Nil {
		sessionId = &body.SessionId
	}
	// sessionId := uuid.Must(uuid.Parse("01994b58-6631-7c98-bcc0-c1e02e436a89"))
	// session, err := lm.GetOrCreateSession(&userID, &agentID, &sessionId, nil)
	session, err := s.agent.GetOrCreateSession(&userID, &agentID, sessionId, nil)
	if err != nil {
		fmt.Println("Failed to get or create session", "error", err)
		return
	}
	// Initilize session
	err = session.Initialize()
	if err != nil {
		fmt.Println("Failed to initialize session", "error", err)
		return
	}
	inThinkingBlock := false
	session.AddChatCallback(func(content string, thinking bool, endBlock bool) error {
		if thinking {
			if !inThinkingBlock {
				inThinkingBlock = true
			}
			writeSSE(w, map[string]any{"type": "thinking_delta", "data": map[string]any{"thinking": content}})
		} else {
			if inThinkingBlock {
				inThinkingBlock = false
			}
			writeSSE(w, map[string]any{"type": "text_delta", "data": map[string]any{"text": content}})
		}
		if endBlock {
			writeSSE(w, map[string]any{"type": "complete"})
		}
		return nil
	})
	err = session.HumanInput(body.Content)
	if err != nil {
		fmt.Println("Failed to send human input", "error", err)
		return
	}
	nLoop := 0
	for !session.IsHumanTurn() {
		err = session.ContinueTurn()
		if err != nil {
			fmt.Println("Failed to continue turn", "error", err)
			return
		}
		nLoop++
		if nLoop > 10 {
			fmt.Println("Too many loops, something is wrong")
			return
		}
	}
}

func writeSSE(w http.ResponseWriter, v any) {
	data, _ := json.Marshal(v)
	fmt.Fprintf(w, "data: %s\n\n", data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
