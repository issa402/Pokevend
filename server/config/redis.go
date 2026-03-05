// ============================================================
// config/redis.go — Redis client via go-redis/v9
// ============================================================
package config

import (
	"github.com/redis/go-redis/v9"
)

func ConnectRedis(cfg *Config) *redis.Client {
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		// Fallback to default if URL is malformed
		opts = &redis.Options{Addr: "localhost:6379"}
	}
	return redis.NewClient(opts)
}
