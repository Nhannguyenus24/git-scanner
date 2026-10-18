// Package store owns Postgres access.
//
// It lives in internal/ rather than libs/ because the schema is private to this
// system; nothing outside it should depend on these tables.
package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed all:migrations
var migrationsFS embed.FS

// DB is the shared connection pool.
type DB struct {
	pool *pgxpool.Pool
}

// Open connects and verifies the pool eagerly.
func Open(ctx context.Context, url string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("store: parse url: %w", err)
	}

	// Workers hold a connection for the length of a batch, not a request, so the
	// default pool size is too small under fan-out. Bounded so a scaled-out
	// deployment cannot exhaust Postgres max_connections.
	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 10 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("store: create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}

	return &DB{pool: pool}, nil
}

// Pool exposes the driver for queries this package does not wrap.
func (db *DB) Pool() *pgxpool.Pool { return db.pool }

// Ping is the readiness check.
func (db *DB) Ping(ctx context.Context) error { return db.pool.Ping(ctx) }

// Close releases the pool.
func (db *DB) Close() error {
	db.pool.Close()
	return nil
}

// InTx runs fn inside a transaction, rolling back on error or panic.
func (db *DB) InTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			return errors.Join(err, fmt.Errorf("store: rollback: %w", rbErr))
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit: %w", err)
	}
	return nil
}

// Migrate applies pending migrations under an advisory lock.
//
// The lock is what makes this safe to call from more than one place. It normally
// runs as a Helm pre-upgrade hook (plan.md §13.6), but a developer running a
// service directly should not have to remember a separate step, and two of them
// starting at once must not race.
func (db *DB) Migrate(ctx context.Context) error {
	const lockID = 0x6749_5343 // "giSC"

	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("store: acquire for migrate: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lockID); err != nil {
		return fmt.Errorf("store: acquire migration lock: %w", err)
	}
	defer func() {
		_, _ = conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", lockID)
	}()

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("store: create schema_migrations: %w", err)
	}

	applied := map[string]bool{}
	rows, err := conn.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("store: read schema_migrations: %w", err)
	}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return fmt.Errorf("store: scan schema_migrations: %w", err)
		}
		applied[v] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("store: iterate schema_migrations: %w", err)
	}

	files, err := upMigrations()
	if err != nil {
		return err
	}

	for _, f := range files {
		version := strings.TrimSuffix(f, ".up.sql")
		if applied[version] {
			continue
		}

		body, err := migrationsFS.ReadFile("migrations/" + f)
		if err != nil {
			return fmt.Errorf("store: read migration %s: %w", f, err)
		}

		// Each migration runs in its own transaction, so a failure leaves the
		// database at the last complete version rather than half-applied.
		err = db.InTx(ctx, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, string(body)); err != nil {
				return fmt.Errorf("store: apply %s: %w", f, err)
			}
			_, err := tx.Exec(ctx,
				"INSERT INTO schema_migrations (version) VALUES ($1)", version)
			return err
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func upMigrations() ([]string, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("store: list migrations: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}
