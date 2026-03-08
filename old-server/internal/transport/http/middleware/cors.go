package middleware

import (
	"net/http"
	"strings"

	"schma.ai/internal/pkg/logger"
)

// CORSMiddleware adds CORS headers to allow cross-origin requests
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// short circuit for websocket upgrade, skip cors.
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			next.ServeHTTP(w, r)
			return
		}
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*") // In production, specify your domain
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, x-api-key")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Continue to next handler
		next.ServeHTTP(w, r)
	})
}

// DevelopmentCORS is a more permissive CORS middleware for development
func DevelopmentCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// short circuit for websocket upgrade, skip cors.
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")

		logger.Debugf("🌐 [CORS] %s %s from origin: %s", r.Method, r.URL.Path, origin)

		// Allow localhost origins for development
		if origin == "http://localhost:3000" || origin == "http://localhost:3001" ||
			origin == "https://localhost:3000" || origin == "https://localhost:3001" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			logger.ServiceDebugf("CORS", "🌐 Allowing origin: %s", origin)
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			logger.ServiceDebugf("CORS", "🌐 Using wildcard origin for: %s", origin)
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, x-api-key")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			logger.ServiceDebugf("CORS", "🌐 Handling OPTIONS preflight request")
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
