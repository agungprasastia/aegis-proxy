package middleware

import (
	"net/http"
	"strings"
)

func AuthMiddleware(apiKeyGetter func() string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}
			
			expectedKey := apiKeyGetter()
			if expectedKey == "" {
				next.ServeHTTP(w, r)
				return
			}
			
			var providedKey string
			
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				if strings.HasPrefix(authHeader, "Bearer ") {
					providedKey = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}
			
			if providedKey == "" {
				providedKey = r.Header.Get("x-api-key")
			}
			
			if providedKey != expectedKey {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":{"message":"Invalid API key","type":"invalid_request_error"}}`))
				return
			}
			
			next.ServeHTTP(w, r)
		})
	}
}
