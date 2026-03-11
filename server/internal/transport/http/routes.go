package http

import (
	"net/http"

	"veritie.io/internal/transport/http/handlers"
)

type RouteDeps struct {
	Middleware Middleware
}

// Routes defines the HTTP routing table.
func Routes(deps RouteDeps) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	jobsHandler := http.Handler(http.HandlerFunc(handlers.Jobs))
	streamHandler := http.Handler(http.HandlerFunc(handlers.Stream))

	if deps.Middleware.Auth != nil {
		jobsHandler = deps.Middleware.Auth(jobsHandler)
		streamHandler = deps.Middleware.Auth(streamHandler)
	}

	mux.Handle("POST /v1/jobs", jobsHandler)
	mux.Handle("GET /v1/jobs", jobsHandler)
	mux.Handle("GET /v1/jobs/{job_id}", jobsHandler)
	mux.Handle("GET /v1/jobs/{job_id}/stream", streamHandler)

	return mux
}
