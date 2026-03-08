package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	domain_auth "schma.ai/internal/domain/auth"
	"schma.ai/internal/pkg/logger"
)

func KeyAuthMiddleware(authService domain_auth.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// Extract API key from header or query parameter (for WebSocket support)
			apiKey := r.Header.Get("x-api-key")
			logger.ServiceDebugf("AUTH", "🔐 middleware Request: %s %s", r.Method, r.URL.Path)
			logger.ServiceDebugf("AUTH", "🔐 middleware Headers: %+v", r.Header)
			logger.ServiceDebugf("AUTH", "🔐 middleware API key from header: '%s'", apiKey)

			if apiKey == "" {
				// Check query parameter for WebSocket connections
				apiKey = r.URL.Query().Get("api_key")
				logger.ServiceDebugf("AUTH", "🔐 middleware API key from query: '%s'", apiKey)
			}
			if apiKey == "" {
				logger.Errorf("❌ middleware No API key found in request")		
				http.Error(w, "Unauthorized: Missing API key", http.StatusUnauthorized)
				return
			}

			// Clean API Key
			apiKey = strings.TrimPrefix(apiKey, "Bearer")

			// Authenticate request
			logger.ServiceDebugf("AUTH", "🔐 middleware Attempting to authenticate with API key: '%s'", apiKey)
			principal, err := authService.AuthenticateRequest(r, apiKey)
			if err != nil {
				logger.Errorf("❌ middleware Authentication failed: %v", err)
				http.Error(w, "Unauthorized: Invalid API key", http.StatusUnauthorized)
				return
			}
			logger.ServiceDebugf("AUTH", "✅ middleware Authentication successful for app: %s", principal.AppName)
				
			// Check rate limit
			allowed, limitInfo, err := authService.CheckRateLimit(r.Context(), principal)

			if err != nil {
				http.Error(w, "Rate limit error", http.StatusInternalServerError)
				return
			}

			// Set rate limit headers
			if limitInfo != nil {
				w.Header().Add("X-RateLimit-Limit", strconv.FormatInt(limitInfo.Limit, 10))
				w.Header().Add("X-RateLimit-Remaining", strconv.FormatInt(limitInfo.Remaining, 10))
				w.Header().Add("X-RateLimit-Reset", strconv.FormatInt(limitInfo.Reset, 10))
			}

			if !allowed {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			// Put principal in context
			ctx := context.WithValue(r.Context(), principalKey, principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
