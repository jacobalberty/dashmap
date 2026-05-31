package api

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/jacobalberty/dashmap/internal/config"
	"github.com/jacobalberty/dashmap/internal/db"
	"github.com/jacobalberty/dashmap/internal/media"
)

// Deps are the dependencies the HTTP server needs. Querier is an interface so
// handlers can be unit-tested with a stub; Resolver is concrete (tests use a real
// resolver over a temp directory).
type Deps struct {
	Querier  Querier
	Resolver *media.Resolver
	Pinger   db.Pinger
	Cfg      *config.Config
}

type Server struct {
	srv  *http.Server
	deps Deps
}

func NewServer(addr string, deps Deps) *Server {
	s := &Server{deps: deps}
	s.srv = &http.Server{
		Addr:              addr,
		Handler:           chain(s.routes(), withRecovery, withLogging),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
		// WriteTimeout intentionally unset: range-streaming needs unbounded writes.
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
