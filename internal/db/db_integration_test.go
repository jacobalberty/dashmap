//go:build integration

package db

import (
	"context"
	"os"
	"testing"
)

func TestMigrateAndPing(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Fatal("DATABASE_URL is required for integration tests")
	}

	ctx := context.Background()

	if err := Migrate(ctx, dsn); err != nil {
		t.Fatalf("Migrate() error: %v", err)
	}

	pool, err := Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer pool.Close()

	pinger := NewPinger(pool)
	if err := pinger.Ping(ctx); err != nil {
		t.Fatalf("Ping() error: %v", err)
	}
}
