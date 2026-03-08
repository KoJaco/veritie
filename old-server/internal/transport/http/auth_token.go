package http

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	domain_auth "schma.ai/internal/domain/auth"
	"schma.ai/internal/pkg/logger"
	"schma.ai/internal/transport/http/middleware"

	"github.com/golang-jwt/jwt/v5"
)

type MintResponse struct {
	Token     string    `json:"token"`
	ExpiresIn int64 `json:"expires_in"`
	SID       string    `json:"sid"`
}

type AuthTokenHandler struct {
	sessionManager domain_auth.WSSessionManager
	issuer string
	signingKey []byte // HS256; for RS256 store private key
}

func NewAuthTokenHandler(sm domain_auth.WSSessionManager) *AuthTokenHandler {
	return &AuthTokenHandler{
		sessionManager: sm, 
		issuer: os.Getenv("JWT_ISSUER"),
		signingKey: []byte(os.Getenv("JWT_SIGNING_KEY")), // HS256; for RS256 store private key
	}
}




func (h *AuthTokenHandler) HandleMint(w http.ResponseWriter, r *http.Request) {
	logger.ServiceDebugf("AUTH", "🔑[MINT] %s %s", r.Method, r.URL.Path)
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 🚫 Enforce server-to-server: reject browser/cross-origin
	if origin := r.Header.Get("Origin"); origin != "" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Principal injected by API-key middleware
	principal, ok := middleware.GetPrincipal(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Create a short server session (optional but useful)
	session, err := h.sessionManager.CreateSession(r.Context(), principal)
	if err != nil {
		logger.Errorf("create session: %v", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Mint short-lived JWT (60-120s)
	now := time.Now()
	exp := now.Add(90 * time.Second)


	claims := jwt.MapClaims{
		"iss": h.issuer,
		"sub": principal.AppID, // tenant/app
		"sid": session.ID,      // ws_session_id
		"typ": "ws-client",
		"iat": now.Unix(),
		"exp": exp.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(h.signingKey)
	if err != nil {
		logger.Errorf("sign jwt: %v", err)
		http.Error(w, "Failed to mint token", http.StatusInternalServerError)
		return
	}
	resp := MintResponse{
		Token:     signed,
		ExpiresIn: int64(time.Until(exp).Seconds()),
		SID:       session.ID,
	}

	logger.ServiceDebugf("AUTH", "🔑[MINT] resp=%+v", resp)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)

}