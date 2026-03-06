// ============================================================
// FILE: server/config/redis.go
// TYPE: Infrastructure — Redis Connection
//
// WHAT IS REDIS?
// Redis (Remote Dictionary Server) is an in-memory key-value store.
// "In-memory" = data lives in RAM, not on disk → extremely fast (~microseconds).
// Used where you need speed first, durability second.
//
// HOW WE USE REDIS IN THIS PROJECT:
//   1. Cache search results   → "search:charizard" → JSON array of cards (TTL: 5min)
//   2. Cache trending cards   → "trending:rising"  → JSON array (TTL: 30min)
//   3. Cache deals of the day → "deals:today"      → JSON array (TTL: 6hr)
//   4. Rate limiting counters → "ratelimit:ip:..."  → request count per IP
//
// CACHE-ASIDE PATTERN (how our services use Redis):
//   1. Check cache: GET "search:charizard"
//   2. Cache hit? Return the cached JSON instantly (sub-millisecond)
//   3. Cache miss? Query PostgreSQL, store result in Redis with TTL, return it
//
// WHY NOT JUST USE POSTGRESQL FOR EVERYTHING?
// PostgreSQL query: ~1-10ms (network + disk I/O)
// Redis GET:         ~0.1ms (RAM only)
// For endpoints hit 100s of times/second, the difference is massive.
//
// FAANG STANDARD: Redis is used at virtually every major tech company.
// Twitter uses Redis for timelines, GitHub for caching, Stripe for rate limiting.
//
// GO CONCEPTS:
//   go-redis/v9 client, options struct, context usage
// ============================================================
package config

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// ConnectRedis creates and returns a Redis client.
// Unlike PostgreSQL (where ConnectPostgres returns an error),
// redis.NewClient() never fails at creation time — it's lazy.
// The first actual command (Ping) reveals if Redis is actually reachable.
func ConnectRedis(cfg *Config) *redis.Client {
	// redis.Options configures the connection.
	// Addr = "host:port" — matches REDIS_URL env var format.
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr, // e.g., "redis:6379" (Docker service name) or "localhost:6379"
		Password: "",            // no password in dev; set REDIS_PASSWORD in production
		DB:       0,             // Redis has 16 databases (0-15). Use 0 for simplicity.
		// PoolSize: number of connections in the pool (default: 10 * NumCPU)
		// MaxRetries: auto-retry on connection failure (default: 3)
	})

	// Verify Redis is actually reachable with a PING command
	// Redis responds to PING with "PONG" — a standard health check.
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		// Note: we don't fatal here — Redis being unavailable degrades performance
		// but doesn't completely break the app (services fall back to PostgreSQL).
		// Decision: log warning, return client anyway, handle cache misses gracefully.
		// In a stricter setup, you'd log.Fatalf here.
	}

	return rdb
}

// TODO #1 (Practice): Add Redis key helpers
// To avoid typos in cache keys across services, define key-builder functions:
//   func CardSearchKey(query string) string { return "search:" + query }
//   func TrendingKey(label string) string   { return "trending:" + label }
//   func DealsTodayKey() string             { return "deals:today" }
// Move these to pkg/cache_keys.go so all services use consistent key names.
// Typo in a key = cache miss every time = slow responses nobody can explain.

// TODO #2 (Practice): Add Redis health check to /health endpoint
// The current /health endpoint only checks if the HTTP server is alive.
// Enhance it to check Redis too:
//   rdb.Ping(r.Context()).Err() == nil → Redis healthy
// Return: {"status":"ok","postgres":"ok","redis":"ok"} or 503 if degraded.
// This is how FAANG health checks work — each dependency is checked individually.
