// Package store is the PostgreSQL/TimescaleDB data-access layer.
package store

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/southcorner/systemcheck/server/migrations"
)

// ErrNotFound is returned when a lookup finds no row.
var ErrNotFound = errors.New("not found")

// Store wraps a pgx connection pool.
type Store struct {
	Pool *pgxpool.Pool
}

// Open connects to Postgres and verifies connectivity.
func Open(ctx context.Context, dbURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &Store{Pool: pool}, nil
}

// Close releases the pool.
func (s *Store) Close() { s.Pool.Close() }

// Migrate applies embedded migrations. It is idempotent: each file is recorded
// in schema_migrations and skipped if already applied.
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.Pool.Exec(ctx,
		`CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
	); err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && len(e.Name()) > 4 && e.Name()[len(e.Name())-4:] == ".sql" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var exists bool
		if err := s.Pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name=$1)`, name,
		).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		sqlBytes, err := migrations.FS.ReadFile(name)
		if err != nil {
			return err
		}
		if _, err := s.Pool.Exec(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := s.Pool.Exec(ctx,
			`INSERT INTO schema_migrations(name) VALUES($1)`, name,
		); err != nil {
			return err
		}
	}
	return nil
}

// noRows maps pgx.ErrNoRows to ErrNotFound.
func noRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
