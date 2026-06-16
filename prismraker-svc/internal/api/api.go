// Package api serves the normalized color state to the React UI over REST and
// a broadcast WebSocket. Kept dependency-light (stdlib + gorilla).
package api

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/yourname/prismraker-svc/internal/ledger"
)

type Server struct {
	led      *ledger.Ledger
	upgrader websocket.Upgrader

	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
}

func New(led *ledger.Ledger) *Server {
	return &Server{
		led:     led,
		clients: make(map[*websocket.Conn]struct{}),
		upgrader: websocket.Upgrader{
			// Dev-friendly; lock this down for production deployments.
			CheckOrigin: func(*http.Request) bool { return true },
		},
	}
}

func (s *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/toolheads", s.handleToolheads)
	mux.HandleFunc("/api/stream", s.handleStream)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

func (s *Server) handleToolheads(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(s.led.Snapshot())
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.clients[conn] = struct{}{}
	s.mu.Unlock()

	// Push the current snapshot immediately on connect.
	_ = conn.WriteJSON(s.led.Snapshot())
}

// Broadcast pushes the latest snapshot to all connected UI clients. Call this
// whenever the ledger changes.
func (s *Server) Broadcast() {
	snap := s.led.Snapshot()
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.clients {
		if err := c.WriteJSON(snap); err != nil {
			_ = c.Close()
			delete(s.clients, c)
		}
	}
}
