package handlers

import (
	"encoding/json"
	"net/http"

	"veritie.io/internal/app/auth"
)

// Jobs handles job REST endpoints.
func Jobs(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		http.Error(w, "missing principal context", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":         true,
		"route":      "jobs",
		"app_id":     principal.AppID.String(),
		"account_id": principal.AccountID.String(),
	})
}
