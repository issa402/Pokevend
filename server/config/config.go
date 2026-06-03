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
	"errors"  // Provides functions to create and manipulate new error messages (e.g., errors.New)
	"fmt"     // Formatted I/O package used for printing text to the console, formatting strings, and scanning input
	"os"      // Provides a platform-independent interface to operating system functions like accessing environment variables (os.Getenv) or file system interaction
	"strconv" // Converts string representations of basic data types to actual types (e.g., string to int)

	"github.com/joho/godotenv" // Third-party library to load environment variables from a .env file into the system's environment variables
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

	// PokeTCG / PokeAi market data service
	PokeTCGBaseURL     string
	APIConsumerBaseURL string

	// Odoo account/product sync. Password is read from env only; never hardcode it.
	OdooURL      string
	OdooDB       string
	OdooUsername string
	OdooPassword string
}

// Load reads all environment variables and returns a Config.
// Called once in main.go: cfg := config.Load()
// Environment variables come from: .env file (dev) or EC2 environment (prod)

// the pointer in front of config returns a memory address not the actual data, instead of copying the entire Config struct everytime we use it we can just pass a tiny map to where the data lives in memory
func Load() *Config {
	// the reason theres a "_" is because if env returns nothing so error we ignore it so we can use the default values
	_ = godotenv.Load("../.env")
	return &Config{ // the & symbol takes a struct and find its location in memory since the function signature says it returns a pointer *Config we must use the & to point to the struct
		// getEnv(key, default) — if KEY is not set, use the default
		// This means the server works with zero config in development
		Port:      getEnv("PORT", "3001"),
		Env:       getEnv("NODE_ENV", "development"),
		ClientURL: getEnv("CLIENT_URL", "http://localhost:5173"),

		// PostgreSQL connection details — default to the Docker service name.
		// If you run the Go server directly on your laptop, switch POSTGRES_HOST to "localhost".
		PostgresHost:     getEnv("POSTGRES_HOST", "postgres"),
		PostgresPort:     getEnv("POSTGRES_PORT", "5432"),
		PostgresDB:       getEnv("POSTGRES_DB", "pokemontool"),
		PostgresUser:     getEnv("POSTGRES_USER", "pokemontool_user"),
		PostgresPassword: getEnv("POSTGRES_PASSWORD", "pokemontool_pass"),

		// Redis — default is the Docker service name "redis:6379".
		// If you run the Go server directly on your laptop, use "localhost:6379".
		RedisAddr: getEnv("REDIS_URL", "redis:6379"),

		// RabbitMQ — default is the Docker service name "rabbitmq".
		// If you run the Go server directly on your laptop, use
		// amqp://guest:guest@localhost:5672
		RabbitMQURL: getEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672"),

		// CRITICAL SECURITY: These MUST be overridden in production via .env or EC2 secrets
		// JWT_SECRET: any string; longer = more secure (use openssl rand -hex 32)
		// ENCRYPTION_KEY: must be exactly 64 hex chars = 32 bytes = AES-256
		JWTSecret:     getEnv("JWT_SECRET", "change-me-in-production"),
		EncryptionKey: getEnv("ENCRYPTION_KEY", "0000000000000000000000000000000000000000000000000000000000000000"),

		// PokeTCG market data API. Local development uses the host-published port; Docker Compose overrides this to http://poketcg:8765.
		PokeTCGBaseURL:     getEnv("POKETCG_BASE_URL", "http://127.0.0.1:8765"),
		APIConsumerBaseURL: getEnv("API_CONSUMER_BASE_URL", "http://api-consumer:8001"),

		OdooURL:      getEnv("ODOO_URL", ""),
		OdooDB:       getEnv("ODOO_DB", ""),
		OdooUsername: getEnv("ODOO_USERNAME", ""),
		OdooPassword: getEnv("ODOO_PASSWORD", ""),
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
