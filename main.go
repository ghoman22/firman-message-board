package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type message struct {
	ID   int64  `json:"id"`
	Text string `json:"text"`
}

type server struct {
	mu       sync.RWMutex
	messages []message
	nextID   int64
	requests atomic.Uint64
}

func newServer() *server { return &server{messages: make([]message, 0)} }

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /messages", s.listMessages)
	mux.HandleFunc("POST /messages", s.addMessage)
	mux.HandleFunc("DELETE /messages/{id}", s.deleteMessage)
	mux.HandleFunc("GET /metrics", s.metrics)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/messages" || strings.HasPrefix(r.URL.Path, "/messages/") {
			s.requests.Add(1)
		}
		mux.ServeHTTP(w, r)
	})
}

func (s *server) metrics(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	messageCount := len(s.messages)
	s.mu.RUnlock()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w,
		"# HELP message_board_api_requests_total Total requests to the message API.\n"+
			"# TYPE message_board_api_requests_total counter\n"+
			"message_board_api_requests_total "+strconv.FormatUint(s.requests.Load(), 10)+"\n"+
			"# HELP message_board_messages Current number of messages in memory.\n"+
			"# TYPE message_board_messages gauge\n"+
			"message_board_messages "+strconv.Itoa(messageCount)+"\n",
	)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (s *server) listMessages(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	items := append([]message{}, s.messages...)
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, items)
}

func (s *server) addMessage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	defer r.Body.Close()
	var input struct {
		Text string `json:"text"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		http.Error(w, "body harus JSON dengan field text", http.StatusBadRequest)
		return
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		http.Error(w, "body harus satu objek JSON", http.StatusBadRequest)
		return
	}
	input.Text = strings.TrimSpace(input.Text)
	if input.Text == "" || len(input.Text) > 140 {
		http.Error(w, "text harus 1-140 karakter", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.nextID++
	item := message{ID: s.nextID, Text: input.Text}
	s.messages = append(s.messages, item)
	s.mu.Unlock()
	writeJSON(w, http.StatusCreated, item)
}

func (s *server) deleteMessage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, item := range s.messages {
		if item.ID == id {
			s.messages = append(s.messages[:i], s.messages[i+1:]...)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	http.NotFound(w, r)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	app := &http.Server{
		Addr:              ":" + port,
		Handler:           newServer().routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("message board listening on :%s", port)
	log.Fatal(app.ListenAndServe())
}
