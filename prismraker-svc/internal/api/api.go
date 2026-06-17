// Package api serves the normalized color state to the React UI over REST and
// a broadcast WebSocket. Kept dependency-light (stdlib + gorilla).
package api

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/yourname/prismraker-svc/internal/ledger"
	"github.com/yourname/prismraker-svc/internal/spoolsvc"
)

type Server struct {
	led      *ledger.Ledger
	spools   *spoolsvc.Service
	upgrader websocket.Upgrader

	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
}

func New(led *ledger.Ledger, spools *spoolsvc.Service) *Server {
	return &Server{
		led:     led,
		spools:  spools,
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
	mux.HandleFunc("/api/spools", s.handleSpools)
	mux.HandleFunc("/api/assign", s.handleAssign)
	mux.HandleFunc("/api/stream", s.handleStream)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

// cors sets permissive headers and answers preflight; harmless in dev where the
// Vite proxy makes requests same-origin, useful if the UI is served elsewhere.
func cors(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}

func (s *Server) handleToolheads(w http.ResponseWriter, r *http.Request) {
	if cors(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.led.Snapshot())
}

// handleSpools lists the spools the user can load onto a toolhead.
func (s *Server) handleSpools(w http.ResponseWriter, r *http.Request) {
	if cors(w, r) {
		return
	}
	opts, err := s.spools.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(opts)
}

// handleAssign loads (or clears, with spoolId 0) a spool onto a toolhead.
func (s *Server) handleAssign(w http.ResponseWriter, r *http.Request) {
	if cors(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Toolhead int `json:"toolhead"`
		SpoolID  int `json:"spoolId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.spools.Assign(r.Context(), body.Toolhead, body.SpoolID); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.Broadcast() // push the new color/weight to every open dashboard
	w.WriteHeader(http.StatusNoContent)
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
