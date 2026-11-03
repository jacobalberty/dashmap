package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// version is stamped at build time via -ldflags "-X main.version=...".
var version = "dev"

// runHealthcheck probes the local /livez endpoint and exits non-zero on failure.
// The distroless runtime image has no shell, wget or curl, so container
// healthchecks run this subcommand instead.
func runHealthcheck() {
	if err := probeLivez(os.Getenv("HTTP_ADDR")); err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck: %v\n", err)
		os.Exit(1)
	}
}

func probeLivez(addr string) error {
	if addr == "" {
		addr = ":8080"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid HTTP_ADDR %q: %w", addr, err)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort(host, port)+"/livez", nil) //nolint:gosec // G704: target is the operator-set HTTP_ADDR on loopback
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req) //nolint:gosec // G704: see above
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}
	return nil
}
