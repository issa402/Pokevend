// main.go — Dependency injection: wire all layers, start server
// This is the only file where layers talk to each other directly.
// Everything flows: config → store → service → handler → routes
package main

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"pokemontool/config"
	"pokemontool/handlers"
	"pokemontool/middleware"
	"pokemontool/routes"
	"pokemontool/services"
	"pokemontool/store"
	"pokemontool/worker"
)

func main() {
	// ── 1. Config ──────────────────────────────────────────────
	cfg := config.Load()

	// ── 2. Infrastructure: DB + Cache + Queue ─────────────────
	db, err := config.ConnectPostgres(cfg)
	if err != nil {
		log.Fatalf("❌ PostgreSQL: %v", err)
	}
	defer db.Close()
	log.Println("  ✓ PostgreSQL connected")

	rdb := config.ConnectRedis(cfg)
	log.Println("  ✓ Redis connected")

	rmq, err := config.ConnectRabbitMQ(cfg)
	if err != nil {
		log.Printf("  ⚠️  RabbitMQ unavailable: %v", err)
	} else {
		log.Println("  ✓ RabbitMQ connected")
	}

	// ── 3. Store layer (repositories) ─────────────────────────
	userStore      := store.NewUserStore(db)
	cardStore      := store.NewCardStore(db)
	alertStore     := store.NewAlertStore(db)
	watchlistStore := store.NewWatchlistStore(db)
	inventoryStore := store.NewInventoryStore(db)
	dealStore      := store.NewDealStore(db)
	showStore      := store.NewShowStore(db)

	// ── 4. Service layer (business logic) ─────────────────────
	authSvc      := services.NewAuthService(userStore, cfg.JWTSecret)
	cardSvc      := services.NewCardService(cardStore, rdb)
	alertSvc     := services.NewAlertService(alertStore)
	watchlistSvc := services.NewWatchlistService(watchlistStore)
	inventorySvc := services.NewInventoryService(inventoryStore)
	dealSvc      := services.NewDealService(dealStore, rdb)
	showSvc      := services.NewShowService(showStore, rdb)

	// ── 5. Handlers (HTTP layer) ───────────────────────────────
	sseManager   := handlers.NewSSEManager()
	authH        := handlers.NewAuthHandler(authSvc)
	cardH        := handlers.NewCardHandler(cardSvc)
	alertH       := handlers.NewAlertHandler(alertSvc)
	watchlistH   := handlers.NewWatchlistHandler(watchlistSvc)
	inventoryH   := handlers.NewInventoryHandler(inventorySvc)
	dealH        := handlers.NewDealHandler(dealSvc)
	showH        := handlers.NewShowHandler(showSvc)
	apikeyH      := handlers.NewAPIKeyHandler(db, cfg)

	// ── 6. Background worker (RabbitMQ → SSE) ─────────────────
	if rmq != nil {
		go worker.StartNotificationWorker(rmq, db, sseManager)
	}

	// ── 7. Router + Global Middleware ─────────────────────────
	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.ClientURL, "http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))
	r.Use(middleware.RateLimit(100))

	// ── 8. Register all routes ─────────────────────────────────
	routes.Register(r, cfg, sseManager, authH, cardH, alertH, watchlistH, inventoryH, dealH, showH, apikeyH)

	log.Printf("\n🚀 PokémonTool Go API — port %s\n", cfg.Port)
	log.Printf("   Health: http://localhost:%s/health\n", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
