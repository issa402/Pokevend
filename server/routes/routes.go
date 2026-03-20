// ============================================================
// FILE: server/routes/routes.go
// TYPE: Routes Layer — URL → Handler Mapping
//
// WHAT IS THE ROUTES FILE?
// This is the SINGLE SOURCE OF TRUTH for every URL in your API.
// When a developer wants to know "what endpoint handles X?",
// they look HERE — not in handlers, not in main.go.
//
// FAANG PATTERN: Centralized Route Registration
// Stripe's Go codebase, Uber's Go services — they all have one file
// that lists every route. When you're on-call and need to find
// which handler fires for POST /api/alerts, you grep routes.go.
//
// WHAT THE ROUTES FILE CONTAINS:
//   - URL patterns (paths)
//   - HTTP methods (GET, POST, PUT, DELETE)
//   - Which handler method to call
//   - Which middleware applies to which groups
//   - NO logic of any kind
//
// CHI ROUTER CONCEPTS:
//
//	r.Get / r.Post / r.Put / r.Delete = HTTP method + URL → handler
//	r.Route("/api", ...) = group routes under /api prefix
//	r.Group(func(r) {...}) = sub-router with extra middleware
//	r.Use(middleware)      = middleware applies to all routes in this group
//	{id} = URL parameter   = accessible via chi.URLParam(r, "id")
//
// ============================================================
package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"pokemontool/config"
	"pokemontool/handlers"
	"pokemontool/middleware"
)

// Register wires ALL routes onto the chi router.
// Takes every handler as a parameter — this makes dependencies explicit.
// You can look at this function signature and know EXACTLY what this API has.
func Register(
	r chi.Router,
	cfg *config.Config,
	sse *handlers.SSEManager,
	auth *handlers.AuthHandler,
	cards *handlers.CardHandler,
	alerts *handlers.AlertHandler,
	priceAlerts *handlers.PriceAlertHandler,
	watchlist *handlers.WatchlistHandler,
	inventory *handlers.InventoryHandler,
	deals *handlers.DealHandler,
	shows *handlers.ShowHandler,
	apikeys *handlers.APIKeyHandler,
) {
	// ── Health Check ─────────────────────────────────────────────
	// GET /health — NOT under /api, not authenticated.
	// Used by Docker, AWS load balancers, and monitoring tools to verify
	// the server is alive. Returns immediately with no DB query.
	// At FAANG, this endpoint is called every 30 seconds by health checks.
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"pokemontool-go"}`))
	})

	// ── API Routes ───────────────────────────────────────────────
	// All API routes live under /api prefix.
	// r.Route creates a sub-router for this prefix.
	r.Route("/api", func(r chi.Router) {

		// ── Public Routes (no JWT required) ────────────────────────
		// POST /api/auth/register — create new account
		// POST /api/auth/login    — get JWT with credentials
		r.Post("/auth/register", auth.Register)
		r.Post("/auth/login", auth.Login)

		// ── SSE Stream ─────────────────────────────────────────────
		// GET /api/stream?token=<jwt>
		// WHY token in URL (not header)?
		// The browser's EventSource API does not support custom headers.
		// The JWT must be in the query string. The SSE handler validates it.
		// This is the standard workaround for SSE authentication.
		r.Get("/stream", handlers.Stream(sse, cfg))

		// ── Protected Routes ───────────────────────────────────────
		// r.Group creates a sub-router that inherits the parent's routes.
		// r.Use inside the group ONLY applies to routes inside the group.
		// RequireAuth runs BEFORE every handler in this group.
		// If JWT is invalid → 401 returned, handler never executes.
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(cfg)) // applies to all routes below

			// ── Cards ─────────────────────────────────────
			// GET /api/cards/search?q=charizard&limit=20
			// GET /api/cards/trending
			// GET /api/cards/{id}/history
			r.Get("/cards/search", cards.Search)
			r.Get("/cards/trending", cards.Trending)
			r.Get("/cards/{id}/history", cards.PriceHistory)
			r.Get("/cards/{id}", cards.Detail)
			// {id} = URL parameter. In the handler: chi.URLParam(r, "id")

			// ── Alerts ────────────────────────────────────
			// GET    /api/alerts                 list user's alerts + unread count
			// PUT    /api/alerts/read-all        mark everything read
			// PUT    /api/alerts/{id}/read       mark one alert read
			// DELETE /api/alerts/{id}            delete one alert
			r.Get("/alerts", alerts.List)
			r.Get("/alerts/unread-count", alerts.UnreadCount)
			r.Put("/alerts/read-all", alerts.MarkAllRead)
			r.Put("/alerts/{id}/read", alerts.MarkRead)
			r.Delete("/alerts/{id}", alerts.Delete)

			//________________PRICE_ALERTS_________
			r.Get("/price-alerts", priceAlerts.List)
			r.Post("/price-alerts", priceAlerts.Create)
			r.Delete("/price-alerts/{id}", priceAlerts.Delete)
			// ── Watchlist ─────────────────────────────────
			// GET    /api/watchlist       list watched cards
			// POST   /api/watchlist       add a card to watchlist
			// DELETE /api/watchlist/{id}  remove from watchlist
			r.Get("/watchlist", watchlist.List)
			r.Post("/watchlist", watchlist.Add)
			r.Delete("/watchlist/{id}", watchlist.Remove)

			// ── Inventory ─────────────────────────────────
			// GET    /api/inventory            list owned cards
			// POST   /api/inventory            add card to collection
			// DELETE /api/inventory/{id}       remove from collection
			// POST   /api/inventory/import     upload CSV file
			// GET    /api/inventory/export     download CSV
			r.Get("/inventory", inventory.List)
			r.Post("/inventory", inventory.Add)
			r.Delete("/inventory/{id}", inventory.Delete)
			r.Post("/inventory/import", inventory.Import)
			r.Get("/inventory/export", inventory.Export)

			// ── Deals ─────────────────────────────────────
			// GET /api/deals/today  today's best-value cards
			r.Get("/deals/today", deals.Today)

			// ── Shows ─────────────────────────────────────
			// GET /api/shows/upcoming  upcoming TCG events near the user
			r.Get("/shows/upcoming", shows.Upcoming)

			// ── API Keys ──────────────────────────────────
			// GET    /api/apikeys               list stored platforms
			// POST   /api/apikeys               save/update encrypted key
			// DELETE /api/apikeys/{platform}    remove key for a platform
			r.Get("/apikeys", apikeys.List)
			r.Post("/apikeys", apikeys.Save)
			r.Delete("/apikeys/{platform}", apikeys.Delete)
		})
	})

	// Catch-all: return 404 for unknown routes instead of nothing
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"endpoint not found"}`))
	})
}

// TODO #1 (Practice): Add API versioning
// FAANG APIs are versioned — POST /api/v1/auth/login vs /api/v2/auth/login.
// This lets you release breaking changes without breaking existing clients.
// Refactor: move routes under r.Route("/api/v1", ...)
// Then add r.Route("/api/v2", ...) when you add new response formats.
// Clients using v1 continue working while you build v2.

// TODO #2 (Practice): Add request logging middleware to specific route groups
// chi has a built-in RequestID middleware that adds a unique ID to each request.
// Add chimiddleware.RequestID to the protected group.
// Then log: "request_id=abc123 user=uuid method=GET path=/api/cards/trending"
// This is how FAANG traces a specific request through distributed logs.
// Research: chimiddleware.RequestID, r.Use(chimiddleware.RequestID)
