// transport/http/auth_token.go (you can keep the same type/filename if you want)
package http

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	domain_auth "schma.ai/internal/domain/auth"
	"schma.ai/internal/pkg/auth"
	"schma.ai/internal/pkg/logger"
	"schma.ai/internal/transport/http/middleware"
)

type AuthHandler struct {
	sessionManager domain_auth.WSSessionManager
}

func NewAuthHandler(sessionManager domain_auth.WSSessionManager) *AuthHandler {
	return &AuthHandler{sessionManager: sessionManager}
}

func (h *AuthHandler) HandleAuth(w http.ResponseWriter, r *http.Request) {
	logger.ServiceDebugf("AUTH", "🔑 [AUTH_HANDLER] Received %s %s", r.Method, r.URL.Path)
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Principal from API-key middleware (unchanged)
	principal, ok := middleware.GetPrincipal(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	logger.ServiceDebugf("AUTH", "🔑 [AUTH_HANDLER] Principal: %s (%s)", principal.AppName, principal.AppID)

	// Create a server-side session (unchanged)
	session, err := h.sessionManager.CreateSession(r.Context(), principal)
	if err != nil {
		logger.Errorf("create session: %v", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}
	logger.ServiceDebugf("AUTH", "✅ [AUTH_HANDLER] Session %s exp %s", session.ID, session.ExpiresAt)

	// ⬇️ NEW: mint short-lived JWT the WS server will verify

	now := time.Now()
	exp := now.Add(90 * time.Second) // keep it short

	iss := os.Getenv("JWT_ISSUER") // or JWT_ISS — just be consistent
	if iss == "" {
		http.Error(w, "JWT issuer not configured", http.StatusInternalServerError); return
	}

	claims := jwt.MapClaims{
		"iss": iss,
		"sub": principal.AppID.String(), // app_id (tenant)
		"sid": session.ID,               // your server session id
		"typ": "ws-client",
		"iat": now.Unix(),
		"exp": exp.Unix(),
	}

	keyBytes, err := auth.LoadSigningKeyFromEnv("JWT_SIGNING_KEY")
	if err != nil {
		http.Error(w, "JWT not configured", http.StatusInternalServerError); return
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(keyBytes)

	logger.ServiceDebugf("AUTH", "🔐 [AUTH_HANDLER] claims=%+v, err=%v", claims, err)

	if err != nil {
		logger.Errorf("sign jwt: %v", err)
		http.Error(w, "Failed to mint token", http.StatusInternalServerError)
		return
	}

	// Return JSON the SDK expects
	resp := MintResponse{
		Token:     signed,
		ExpiresIn: int64(time.Until(exp).Seconds()),
		SID:       session.ID,
	}

	logger.ServiceDebugf("AUTH", "✅ [AUTH_HANDLER] Response: %+v", resp)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
	logger.ServiceDebugf("AUTH", "✅ [AUTH_HANDLER] Response encoded")
}
