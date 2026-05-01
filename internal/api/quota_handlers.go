package api

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/aegis-proxy/aegis/internal/quota"
)

func (s *APIServer) HandleListQuotas(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	provider := strings.TrimPrefix(r.URL.Path, "/api/quota/")
	provider = strings.Trim(provider, "/")
	if provider == "" || strings.Contains(provider, "/") {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid provider"})
		return
	}

	quotas, err := quota.NewStore(s.db).ListQuotas(r.Context(), provider)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, quotas)
}

func (s *APIServer) HandleGetQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/quota/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid quota path"})
		return
	}

	q, err := quota.NewStore(s.db).GetQuota(r.Context(), parts[0], parts[1])
	if err != nil {
		if err == sql.ErrNoRows {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "Quota not found"})
			return
		}
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, q)
}
