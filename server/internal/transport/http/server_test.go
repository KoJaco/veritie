package http

import (
	"context"
	"testing"
)

func TestServerNilGuards(t *testing.T) {
	var s *Server
	if err := s.Start(); err == nil {
		t.Fatalf("expected start error on nil server")
	}
	if err := s.Shutdown(context.Background()); err == nil {
		t.Fatalf("expected shutdown error on nil server")
	}
}
