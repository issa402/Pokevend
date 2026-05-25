// ============================================================
// FILE: server/main.go
// TYPE: Go Application Entry Point
//
// WHAT IS THIS?
// main.go is where the Go program starts. Go always begins at func main().
// Think of it as the "brain" that wires all layers together.
// This file ONLY does dependency injection (DI) — it creates instances
// of each layer and passes them into the next layer.
//
// FAANG PATTERN: Composition Root / Dependency Injection
// All dependencies (DB, Redis, services, handlers) are created HERE
// in one place, and passed into the things that need them.
// No global variables. No hidden dependencies.
//
// REQUEST FLOW (top to bottom):
//
//	HTTP Request
//	  → chi Router (routes/routes.go)
//	  → Middleware (JWT auth, rate limit)
//	  → Handler (handlers/*_handler.go)  — parse HTTP
//	  → Service (services/*_service.go)  — business logic
//	  → Store (store/*_store.go)         — SQL queries
//	  → PostgreSQL / Redis
//
// GO CONCEPTS DEMONSTRATED:
//
//	func main(), package imports, short variable declaration :=,
//	defer, goroutines (go keyword), chi router, cors middleware
//
// ============================================================
package main

import (
	"log"
	"net/http"
	"time"

	// chi = lightweight HTTP router. Chosen over Gorilla Mux because:
	// - composable middleware (r.Use, r.Group)
	// - lightweight (~500KB binary overhead)
	// - net/http compatible (no lock-in)
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware" // aliased to avoid conflict with our middleware pkg
	"github.com/go-chi/cors"

	// Our internal packages — each is a separate folder in this project
	"pokemontool/config"
	"pokemontool/handlers"
	"pokemontool/middleware"
	"pokemontool/routes"
	"pokemontool/services"
	"pokemontool/store"
	"pokemontool/worker"
)

func main() {
	// ── LAYER 1: Configuration ────────────────────────────────
	// Load all environment variables from .env into a Config struct.
	// config.Load() reads os.Getenv() with defaults for missing values.
	// cfg is passed to everything that needs config — no global config.
	cfg := config.Load()

	// ── LAYER 2: Infrastructure Connections ───────────────────
	// Connect to all external systems (PostgreSQL, Redis, RabbitMQ).
	// Fail fast: if PostgreSQL isn't available, panic immediately with a
	// clear error message. Don't start serving traffic with no DB.
	//
	// log.Fatalf = print message + call os.Exit(1) (non-zero = error)
	// This is intentional: a server without a database is useless.
	db, err := config.ConnectPostgres(cfg)
	if err != nil {
		log.Fatalf("❌ PostgreSQL: %v", err) // %v formats the error as a string
	}
	// defer runs when main() exits — closes the DB connection cleanly.
	// GO CONCEPT: defer executes LIFO (last in, first out) order.
	// Multiple defers: the last defer added runs first.
	defer db.Close()
	log.Println("  ✓ PostgreSQL connected")

	rdb := config.ConnectRedis(cfg)
	log.Println("  ✓ Redis connected")

	// RabbitMQ is optional — if not running, the API still works.
	// Alerts just won't fire in real-time (they'd be missed).
	rmq, err := config.ConnectRabbitMQ(cfg)
	if err != nil {
		log.Printf("  ⚠️  RabbitMQ unavailable: %v", err)
	} else {
		log.Println("  ✓ RabbitMQ connected")
	}

	// ── LAYER 3: Store (Repository) ───────────────────────────
	// Stores wrap the database — they execute SQL and return models.
	// Each store gets injected with the shared DB pool.
	// db is a *pgxpool.Pool — it manages multiple concurrent DB connections.
	// Passing the pool (not a connection) means stores share connections efficiently.
	userStore := store.NewUserStore(db)
	cardStore := store.NewCardStore(db)
	alertStore := store.NewAlertStore(db)
	priceAlertStore := store.NewPriceAlertStore(db)
	watchlistStore := store.NewWatchlistStore(db)
	inventoryStore := store.NewInventoryStore(db)
	dealStore := store.NewDealStore(db)
	showStore := store.NewShowStore(db)

	// ── LAYER 4: Services (Business Logic) ────────────────────
	// Services receive INTERFACES (not concrete types).
	// Example: authSvc receives UserStore interface, not *postgresUserStore.
	// This means: in tests, you can pass a fake UserStore that returns mock data.
	// Services know WHAT to do (business rules) but not HOW to store (that's stores).
	authSvc := services.NewAuthService(userStore, cfg.JWTSecret)
	cardSvc := services.NewCardService(cardStore, rdb, cfg.PokeTCGBaseURL, cfg.APIConsumerBaseURL) // rdb = Redis for caching
	alertSvc := services.NewAlertService(alertStore)
	watchlistSvc := services.NewWatchlistService(watchlistStore, cfg.APIConsumerBaseURL, cardStore)
	inventorySvc := services.NewInventoryService(inventoryStore, cardStore)
	dealSvc := services.NewDealService(dealStore, rdb)
	showSvc := services.NewShowService(showStore, rdb)

	// ── LAYER 5: Handlers (HTTP Layer) ────────────────────────
	// Handlers receive SERVICE instances (not store instances).
	// Handlers handle HTTP: parse request, call service, write response.
	// They contain NO business logic and NO SQL.
	healthH := handlers.NewHealthHandler(db, rdb)
	sseManager := handlers.NewSSEManager() // SSE connection registry
	authH := handlers.NewAuthHandler(authSvc)
	cardH := handlers.NewCardHandler(cardSvc)
	alertH := handlers.NewAlertHandler(alertSvc)
	priceAlertH := handlers.NewPriceAlertHandler(priceAlertStore)
	watchlistH := handlers.NewWatchlistHandler(watchlistSvc)
	inventoryH := handlers.NewInventoryHandler(inventorySvc)
	dealH := handlers.NewDealHandler(dealSvc)
	showH := handlers.NewShowHandler(showSvc)
	apikeyH := handlers.NewAPIKeyHandler(db, cfg) // apikey doesn't have a service yet

	// ── LAYER 6: Background Worker ────────────────────────────
	// The notification worker runs as a goroutine — independent of HTTP.
	// "go" keyword launches it concurrently alongside the HTTP server.
	// It consumes RabbitMQ messages and pushes SSE events to browsers.
	if rmq != nil {
		go worker.StartNotificationWorker(rmq, db, sseManager)
	}

	// ── LAYER 7: HTTP Router + Middleware ─────────────────────
	// chi.NewRouter() creates the HTTP multiplexer (router).
	// Middleware is applied with r.Use() — it runs for EVERY request.
	r := chi.NewRouter()

	// Built-in chi middleware:
	r.Use(chimiddleware.Logger)                    // log every request: method, path, status, duration
	r.Use(chimiddleware.Recoverer)                 // catch panics, return 500 instead of crashing server
	r.Use(chimiddleware.Timeout(30 * time.Second)) // cancel requests that take too long

	// CORS (Cross-Origin Resource Sharing):
	// Browsers block requests from different origins (e.g., localhost:5173 → localhost:3001)
	// unless the server sends the right CORS headers.
	// Our React app at :5173 needs to call this Go server at :3001.
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.ClientURL, "http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	// Rate limiting: max 100 requests/second per IP.
	// This protects against brute force attacks and DoS.
	r.Use(middleware.RateLimit(100))

	// ── LAYER 8: Route Registration ───────────────────────────
	// All URL → handler mappings live in routes/routes.go (single source of truth).
	// We pass all handler instances to routes.Register so it can wire them.
	routes.Register(r, cfg, sseManager, authH, cardH, alertH, priceAlertH, watchlistH, inventoryH, dealH, showH, apikeyH, healthH)

	// ── Start Server ──────────────────────────────────────────
	// http.ListenAndServe blocks forever, serving requests.
	// It returns only if it encounters an unrecoverable error (e.g., port in use).
	log.Printf("\n🚀 PokémonTool Go API — port %s\n", cfg.Port)
	log.Printf("   Health: http://localhost:%s/health\n", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatalf("Server error: %v", err)
	}

	// TODO #1 (Practice): Add graceful shutdown
	// Currently, when the server receives SIGTERM (e.g., Docker stop),
	// it kills immediately — in-flight requests are dropped.
	// Fix: use signal.NotifyContext(context.Background(), os.Interrupt)
	// and http.Server{}.Shutdown(ctx) to wait for requests to complete.
	// Research: "Go graceful shutdown" — this is required at FAANG.

	// TODO #2 (Practice): Add structured logging
	// log.Println outputs unstructured text — hard to query in production.
	// At FAANG, you use structured JSON logging (every log = a JSON object).
	// Add the "log/slog" package (standard library since Go 1.21):
	//   logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	//   logger.Info("server started", "port", cfg.Port, "env", cfg.Env)
	// This makes logs searchable in CloudWatch, Datadog, etc.
}
