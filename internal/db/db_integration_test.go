//go:build integration

package db

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
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

func TestSchemaCoreQuery(t *testing.T) {
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

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin() error: %v", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Force the planner to use indexes.
	if _, err := tx.Exec(ctx, "SET LOCAL enable_seqscan = off"); err != nil {
		t.Fatalf("SET LOCAL enable_seqscan: %v", err)
	}

	now := time.Now().UTC()

	// Insert car.
	var carID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO cars (key) VALUES ($1) RETURNING id`,
		"test-car",
	).Scan(&carID); err != nil {
		t.Fatalf("insert car: %v", err)
	}

	// Insert event.
	var eventID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO events (car_id, event_ts, folder_ts, kind, folder_rel_path, raw)
		 VALUES ($1, $2, $3, $4, $5, $6::jsonb) RETURNING id`,
		carID, now, now, "saved", "test/2026-05-30_12-00-00", `{"test":true}`,
	).Scan(&eventID); err != nil {
		t.Fatalf("insert event: %v", err)
	}

	// Insert clip.
	var clipID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO clips (car_id, event_id, clip_ts, kind)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		carID, eventID, now, "saved",
	).Scan(&clipID); err != nil {
		t.Fatalf("insert clip: %v", err)
	}

	// Insert segment.
	var segmentID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO segments (clip_id, camera, rel_path)
		 VALUES ($1, $2, $3) RETURNING id`,
		clipID, "front", "test/front.mp4",
	).Scan(&segmentID); err != nil {
		t.Fatalf("insert segment: %v", err)
	}

	// Insert track_points spanning 2 hours at 1-second intervals so the
	// 2-second query window is selective enough for the planner to prefer
	// the ts index over the segment_id index.
	for i := range 7200 {
		ts := now.Add(time.Duration(i-3600) * time.Second)
		if _, err := tx.Exec(ctx,
			`INSERT INTO track_points (car_id, clip_id, segment_id, ts, lat, lon)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			carID, clipID, segmentID, ts, 37.4221, -122.0841,
		); err != nil {
			t.Fatalf("insert track_point: %v", err)
		}
	}

	// Run the core query and assert the inserted rel_path is returned.
	start := now.Add(-time.Second)
	end := now.Add(time.Second)
	var relPath string
	if err := tx.QueryRow(ctx,
		`SELECT s.rel_path
		 FROM track_points tp
		 JOIN segments s ON s.id = tp.segment_id
		 WHERE tp.ts BETWEEN $1 AND $2`,
		start, end,
	).Scan(&relPath); err != nil {
		t.Fatalf("core query: %v", err)
	}
	if relPath != "test/front.mp4" {
		t.Fatalf("core query: got rel_path %q, want %q", relPath, "test/front.mp4")
	}

	// Run EXPLAIN and assert index usage.
	rows, err := tx.Query(ctx,
		`EXPLAIN (FORMAT TEXT)
		 SELECT s.rel_path
		 FROM track_points tp
		 JOIN segments s ON s.id = tp.segment_id
		 WHERE tp.ts BETWEEN $1 AND $2`,
		start, end,
	)
	if err != nil {
		t.Fatalf("EXPLAIN query: %v", err)
	}
	var planLines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("EXPLAIN scan: %v", err)
		}
		planLines = append(planLines, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("EXPLAIN rows: %v", err)
	}
	plan := strings.Join(planLines, "\n")

	if !strings.Contains(plan, "track_points_ts_idx") {
		t.Fatalf("EXPLAIN plan does not reference track_points_ts_idx:\n%s", plan)
	}
	if strings.Contains(plan, "Seq Scan on track_points") {
		t.Fatalf("EXPLAIN plan contains sequential scan on track_points:\n%s", plan)
	}

	// Assert that a duplicate rel_path violates the unique constraint.
	var seg2ID int64
	err = tx.QueryRow(ctx,
		`INSERT INTO segments (clip_id, camera, rel_path)
		 VALUES ($1, $2, $3) RETURNING id`,
		clipID, "front", "test/front.mp4",
	).Scan(&seg2ID)
	if err == nil {
		t.Fatal("expected unique constraint violation, got nil error")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("expected pgError code 23505, got: %v", err)
	}
}
