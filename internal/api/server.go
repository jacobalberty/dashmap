package api

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/jacobalberty/dashmap/internal/db"
)

type Server struct {
	srv    *http.Server
	pinger db.Pinger
}

func NewServer(addr string, pinger db.Pinger) *Server {
	s := &Server{pinger: pinger}
	s.srv = &http.Server{
		Addr:              addr,
		Handler:           chain(s.routes(), withRecovery, withLogging),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
		// WriteTimeout intentionally unset: range-streaming (Story 04) needs unbounded writes.
	}
	return s
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.srv.Addr)
	if err != nil {
		return err
	}
	slog.Info("listening", "addr", ln.Addr().String())
	return s.srv.Serve(ln)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}
