// ============================================================
// FILE: server/config/config.go
// TYPE: Configuration Layer
//
// WHAT IS THIS?
// This file loads all environment variables into a typed Config struct.
// Instead of calling os.Getenv("JWT_SECRET") everywhere in your code,
// you call it ONCE here, put it in a struct, and pass that struct around.
//
// FAANG PATTERN: "Configuration as a Struct"
// - All env var names are in ONE place (easy to audit)
// - Compile-time type safety (JWT_SECRET is always a string)
// - Default values in one place (easy to find what defaults exist)
// - Testable: you can create a Config{} with test values in tests
//
// WHY NOT Global Variables?
// Global state makes code hard to test and reason about.
// cfg := config.Load() is explicit — you can see exactly where config comes from.
//
// GO CONCEPTS:
//
//	os.Getenv(), struct definition, getEnv helper function
//
// ============================================================
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application configuration.
// It's created ONCE in main.go and passed to every layer that needs it.
// The pointer (*Config) is passed so all layers share the same struct.
type Config struct {
	// Server
	Port      string // Which port to listen on (default: "3001")
	Env       string // "development" or "production"
	ClientURL string // React app URL for CORS whitelist

	// PostgreSQL — all fields needed to build the connection string
	PostgresHost     string
	PostgresPort     string
	PostgresDB       string
	PostgresUser     string
	PostgresPassword string

	// Redis — used for caching search results, trending cards, deals
	RedisAddr string

	// RabbitMQ — used for receiving listing messages from Python
	RabbitMQURL string

	// Security
	JWTSecret     string // MUST be at least 32 chars in production — signs JWTs
	EncryptionKey string // MUST be exactly 64 hex chars (32 bytes) — for AES-256-GCM

	// Scraping configuration
	ScrapingIntervalMinutes int
}

// Load reads all environment variables and returns a Config.
// Called once in main.go: cfg := config.Load()
// Environment variables come from: .env file (dev) or EC2 environment (prod)

func Load() *Config {
	_ = godotenv.Load("../.env")
	return &Config{
		// getEnv(key, default) — if KEY is not set, use the default
		// This means the server works with zero config in development
		Port:      getEnv("PORT", "3001"),
		Env:       getEnv("NODE_ENV", "development"),
		ClientURL: getEnv("CLIENT_URL", "http://localhost:5173"),

		// PostgreSQL connection details — must match docker-compose.yml
		PostgresHost:     getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort:     getEnv("POSTGRES_PORT", "5432"),
		PostgresDB:       getEnv("POSTGRES_DB", "pokemontool"),
		PostgresUser:     getEnv("POSTGRES_USER", "pokemontool_user"),
		PostgresPassword: getEnv("POSTGRES_PASSWORD", "pokemontool_pass"),

		// Redis — default is the Docker service name "redis:6379"
		RedisAddr: getEnv("REDIS_URL", "redis:6379"),

		// RabbitMQ — default is guest:guest (dev only; use strong creds in prod)
		RabbitMQURL: getEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672"),

		// CRITICAL SECURITY: These MUST be overridden in production via .env or EC2 secrets
		// JWT_SECRET: any string; longer = more secure (use openssl rand -hex 32)
		// ENCRYPTION_KEY: must be exactly 64 hex chars = 32 bytes = AES-256
		JWTSecret:     getEnv("JWT_SECRET", "change-me-in-production"),
		EncryptionKey: getEnv("ENCRYPTION_KEY", "0000000000000000000000000000000000000000000000000000000000000000"),
	}
}

// getEnv returns the environment variable value, or defaultVal if not set.
// This is a helper function — private to this package (lowercase first letter).
//
// GO NAMING CONVENTION:
//   Exported (public) = UpperCamelCase: Config, Load
//   Unexported (private) = lowerCamelCase: getEnv
//   Private functions can only be called within the same package.

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// TODO #1 (Practice): Add configuration validation
// After Load() reads all env vars, validate the critical ones.
// Add a Validate() method on Config that returns an error if:
//   - JWT_SECRET is still the default "change-me-in-production" in production
//   - ENCRYPTION_KEY is all zeros in production
//   - PORT is not a valid port number (1-65535)
//
// Call cfg.Validate() in main.go and log.Fatalf if it returns an error.
// HINT: strings.HasPrefix, strconv.Atoi for port validation
func (c *Config) Validate() error {
	port, err := strconv.Atoi(c.Port)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("Invalid Port: %s( must be 1- 65535)", c.Port)
	}
	if c.Env == "production" {
		if c.JWTSecret == "change-me-in-production" {
			return errors.New("security risk: EJWT Key")
		}
	}
	return nil
}

// TODO #2 (Practice): Add configuration for feature flags
// Feature flags let you enable/disable features without redeployment.
// Add fields like: EnableRateLimit bool, EnableSSE bool, MaxSearchResults int
// Load them from env vars with sensible defaults.
// This is how FAANG companies do gradual rollouts: deploy code, then
// enable the feature flag for 1% of users, then 10%, then 100%.
