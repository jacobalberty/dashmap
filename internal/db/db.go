package db

import (
	"context"
	"database/sql"
	"embed"
	"io/fs"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed all:migrations
var migrationsFS embed.FS

// Pinger abstracts DB reachability so callers can be tested with a stub.
type Pinger interface {
	Ping(ctx context.Context) error
}

func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, dsn)
}

// Migrate applies all pending migrations using goose.
// Returns nil immediately when no .sql files exist (empty migrations dir is valid).
func Migrate(ctx context.Context, dsn string) error {
	if !hasSQLMigrations(migrationsFS) {
		return nil
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if err := db.PingContext(ctx); err != nil {
		return err
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrationsFS,
		goose.WithTableName("goose_db_version"),
	)
	if err != nil {
		return err
	}

	_, err = provider.Up(ctx)
	return err
}

// hasSQLMigrations reports whether the embedded FS contains any *.sql files.
func hasSQLMigrations(fsys embed.FS) bool {
	entries, _ := fs.ReadDir(fsys, "migrations")
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			return true
		}
	}
	return false
}

// pgxPoolPinger wraps pgxpool.Pool to satisfy the Pinger interface.
type pgxPoolPinger struct{ pool *pgxpool.Pool }

func NewPinger(pool *pgxpool.Pool) Pinger { return &pgxPoolPinger{pool} }

func (p *pgxPoolPinger) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }
