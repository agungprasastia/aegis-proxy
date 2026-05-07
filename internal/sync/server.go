package sync

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type Server struct {
	mu      sync.RWMutex
	payload *CloudPayload
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/sync", s.handleSync)
	mux.HandleFunc("/sync/meta", s.handleMeta)
	return mux
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		payload := s.payload
		s.mu.RUnlock()
		if payload == nil {
			respondSyncJSON(w, http.StatusNotFound, map[string]string{"error": "no sync payload"})
			return
		}
		respondSyncJSON(w, http.StatusOK, payload)
	case http.MethodPut:
		var next CloudPayload
		if err := json.NewDecoder(r.Body).Decode(&next); err != nil {
			respondSyncJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
			return
		}
		if next.Version == 0 || len(next.Data) == 0 {
			respondSyncJSON(w, http.StatusBadRequest, map[string]string{"error": "version and data are required"})
			return
		}
		if next.UpdatedAt.IsZero() {
			next.UpdatedAt = time.Now().UTC()
		}
		s.mu.Lock()
		if s.payload != nil && next.UpdatedAt.Before(s.payload.UpdatedAt) {
			s.mu.Unlock()
			respondSyncJSON(w, http.StatusConflict, map[string]string{"error": "remote payload is newer"})
			return
		}
		s.payload = &next
		s.mu.Unlock()
		respondSyncJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleMeta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.payload == nil {
		respondSyncJSON(w, http.StatusOK, map[string]any{"exists": false})
		return
	}
	respondSyncJSON(w, http.StatusOK, map[string]any{"exists": true, "device_id": s.payload.DeviceID, "updated_at": s.payload.UpdatedAt, "version": s.payload.Version})
}

func respondSyncJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
