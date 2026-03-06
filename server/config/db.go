// ============================================================
// FILE: server/config/db.go
// TYPE: Infrastructure — PostgreSQL Connection
//
// WHAT IS THIS?
// Creates and returns a pgxpool.Pool — a pool of reusable PostgreSQL connections.
//
// WHAT IS A CONNECTION POOL?
// Opening a new database connection is expensive (TCP handshake, auth, etc.).
// A pool keeps N connections open and ready. When your handler needs the DB,
// it grabs a connection from the pool, uses it, then returns it.
//
//   Without pool: 1000 requests → 1000 new connections → slow + DB overloaded
//   With pool:    1000 requests → 10 pooled connections → fast + efficient
//
// FAANG STANDARD: Always use a connection pool for databases.
// pgxpool is the idiomatic PostgreSQL pool for Go.
// Pool size: typically (num_cpu_cores * 2) to (num_cpu_cores * 4)
//
// WHY pgx over database/sql?
// pgx/v5 is faster and has better PostgreSQL-specific features:
//   - Native UUID support
//   - Better TIMESTAMPTZ handling
//   - Batch queries
//   - CopyFrom for bulk inserts
// ============================================================
package config

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ConnectPostgres creates a PostgreSQL connection pool using the config.
// Returns *pgxpool.Pool — this is passed to all Store constructors.
// Returns error if connection fails — main.go will log.Fatalf on error.
func ConnectPostgres(cfg *Config) (*pgxpool.Pool, error) {
	// Build the DSN (Data Source Name) — the PostgreSQL connection string.
	// Format: postgres://user:password@host:port/dbname?options
	// sslmode=disable = no TLS (fine for local/Docker; use require in production)
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.PostgresUser,
		cfg.PostgresPassword,
		cfg.PostgresHost,
		cfg.PostgresPort,
		cfg.PostgresDB,
	)

	// context.Background() = a context with no deadline and no cancellation.
	// Used for long-lived operations like startup connections.
	// In handlers, you use r.Context() instead (which can be cancelled if the request ends).
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
		// %w = "wrap" the error — preserves the original error for errors.Is() checks
	}

	// Ping verifies the connection is actually alive.
	// pgxpool.New() succeeds even if the DB is unreachable —
	// it only attempts connection on the first query.
	// Ping forces an immediate connection attempt.
	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("postgres ping failed: %w", err)
	}

	return pool, nil
}

// TODO #1 (Practice): Configure pool settings
// pgxpool.New accepts a pgxpool.Config (not just a DSN string).
// Use pgxpool.ParseConfig(dsn) then configure:
//   config.MaxConns = 20              // max simultaneous connections
//   config.MinConns = 5               // keep at least 5 connections warm
//   config.MaxConnLifetime = 1 * time.Hour
//   config.MaxConnIdleTime = 30 * time.Minute
// These settings are critical for production — they control how many
// database connections your app holds. Too many = DB overloaded.

// TODO #2 (Practice): Add a health check method
// Add a function: HealthCheck(pool *pgxpool.Pool) error
// that runs: pool.QueryRow(ctx, "SELECT 1").Scan(&result)
// This can be called from the /health endpoint to verify DB connectivity.
// A proper /health endpoint returns 503 Service Unavailable if the DB is down.
