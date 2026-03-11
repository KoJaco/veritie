package http

import (
	"context"
	"net/http"
	"time"
)

// Server wires HTTP transport dependencies.
type Server struct {
	httpServer *http.Server
}

type ServerConfig struct {
	Addr            string
	Handler         http.Handler
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

func NewServer(cfg ServerConfig) *Server {
	readTimeout := cfg.ReadTimeout
	if readTimeout <= 0 {
		readTimeout = 10 * time.Second
	}
	writeTimeout := cfg.WriteTimeout
	if writeTimeout <= 0 {
		writeTimeout = 15 * time.Second
	}
	return &Server{
		httpServer: &http.Server{
			Addr:         cfg.Addr,
			Handler:      cfg.Handler,
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
		},
	}
}

func (s *Server) Start() error {
	if s == nil || s.httpServer == nil {
		return nil
	}
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}
