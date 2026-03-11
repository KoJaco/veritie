package handlers

import (
	"fmt"
	"net/http"

	"veritie.io/internal/app/auth"
)

// Stream handles job SSE endpoints.
func Stream(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		http.Error(w, "missing principal context", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	_, _ = fmt.Fprintf(w, "event: job.snapshot\ndata: {\"app_id\":\"%s\",\"account_id\":\"%s\"}\n\n", principal.AppID, principal.AccountID)
	_, _ = fmt.Fprint(w, ": keepalive\n\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
