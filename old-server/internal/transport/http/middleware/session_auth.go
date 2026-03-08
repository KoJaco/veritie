package middleware

import (
	"context"
	"net/http"

	domain_auth "schma.ai/internal/domain/auth"
)

// SessionAuthMiddleware creates HTTP middleware using session tokens
func SessionAuthMiddleware(sessionManager domain_auth.WSSessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract session token from query parameter (for WebSocket)
			sessionToken := r.URL.Query().Get("session")
			if sessionToken == "" {
				http.Error(w, "Unauthorized: Missing session token", http.StatusUnauthorized)
				return
			}

			// Validate session token
			session, err := sessionManager.ValidateSession(r.Context(), sessionToken)
			if err != nil {
				http.Error(w, "Unauthorized: Invalid session token", http.StatusUnauthorized)
				return
			}

			// Put principal in context (same as KeyAuthMiddleware)
			ctx := context.WithValue(r.Context(), principalKey, session.Principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
