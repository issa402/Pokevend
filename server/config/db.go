// ============================================================
// config/db.go — PostgreSQL connection pool via pgx/v5
// ============================================================
package config

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ConnectPostgres returns a pgxpool connection pool.
// pgxpool is safe for concurrent use — one pool shared across all handlers.
func ConnectPostgres(cfg *Config) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
		cfg.PostgresHost, cfg.PostgresPort,
		cfg.PostgresDB, cfg.PostgresUser, cfg.PostgresPassword,
	)

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	poolCfg.MaxConns = 20

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}
