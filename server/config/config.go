// ============================================================
// config/config.go — Load all configuration from environment
// ============================================================
package config

import (
	"os"
)

type Config struct {
	Port           string
	ClientURL      string
	JWTSecret      string
	EncryptionKey  string // 32-byte hex for AES-256-GCM

	// PostgreSQL
	PostgresHost     string
	PostgresPort     string
	PostgresDB       string
	PostgresUser     string
	PostgresPassword string

	// Redis
	RedisURL string

	// RabbitMQ
	RabbitMQURL string

	// External APIs
	EventbriteToken string
}

func Load() *Config {
	return &Config{
		Port:             getEnv("PORT", "3001"),
		ClientURL:        getEnv("CLIENT_URL", "http://localhost:5173"),
		JWTSecret:        getEnv("JWT_SECRET", "dev_secret_change_in_prod"),
		EncryptionKey:    getEnv("ENCRYPTION_KEY", "a1b2c3d4e5f678901234567890123456"),

		PostgresHost:     getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort:     getEnv("POSTGRES_PORT", "5432"),
		PostgresDB:       getEnv("POSTGRES_DB", "pokemontool"),
		PostgresUser:     getEnv("POSTGRES_USER", "pokemontool_user"),
		PostgresPassword: getEnv("POSTGRES_PASSWORD", "pokemontool_pass"),

		RedisURL:        getEnv("REDIS_URL", "redis://localhost:6379"),
		RabbitMQURL:     getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672"),
		EventbriteToken: getEnv("EVENTBRITE_TOKEN", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
