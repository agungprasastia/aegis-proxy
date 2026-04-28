package middleware

import (
	"fmt"
	"net/http"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		rw := newResponseWriter(w)
		
		next.ServeHTTP(rw, r)
		
		latency := time.Since(start)
		
		timestamp := start.Format("15:04:05")
		fmt.Printf("[%s] %s %s %d %dms\n",
			timestamp,
			r.Method,
			r.URL.Path,
			rw.statusCode,
			latency.Milliseconds(),
		)
	})
}
