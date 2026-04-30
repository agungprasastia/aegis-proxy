package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/aegis-proxy/aegis/internal/provider/apikey"
)

func (s *APIServer) HandleAPIKeyProviders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListAPIKeyProviders(w, r)
	case http.MethodPost:
		s.handleSaveAPIKeyProvider(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *APIServer) HandleTestAPIKeyProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Provider   string `json:"provider"`
		BaseURL    string `json:"base_url"`
		AuthHeader string `json:"auth_header"`
		APIKey     string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}
	if req.Provider == "" || req.BaseURL == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "provider and base_url are required"})
		return
	}
	if req.APIKey == "" {
		key, err := apikey.GetAPIKey(s.db.SQL(), req.Provider)
		if err != nil {
			if err == sql.ErrNoRows {
				respondJSON(w, http.StatusNotFound, map[string]string{"error": "API key not configured"})
				return
			}
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		req.APIKey = key
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	provider := apikey.NewAPIKeyProvider(req.Provider, req.BaseURL, req.AuthHeader)
	if err := provider.TestConnectivity(ctx, req.APIKey); err != nil {
		respondJSON(w, http.StatusBadGateway, map[string]string{"status": "failed", "error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *APIServer) HandleDeleteAPIKeyProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	provider := strings.TrimPrefix(r.URL.Path, "/api/providers/apikey/")
	if provider == "" || provider == r.URL.Path {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "provider is required"})
		return
	}
	if err := apikey.DeleteAPIKey(s.db.SQL(), provider); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "API key deleted"})
}

func (s *APIServer) handleSaveAPIKeyProvider(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider string `json:"provider"`
		APIKey   string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}
	if req.Provider == "" || req.APIKey == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "provider and api_key are required"})
		return
	}
	if err := apikey.SaveAPIKey(s.db.SQL(), req.Provider, req.APIKey); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "API key saved"})
}

func (s *APIServer) handleListAPIKeyProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := apikey.ListAPIKeyProviders(s.db.SQL())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"providers": providers})
}
