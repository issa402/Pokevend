// routes/routes.go — ONLY URL → handler mapping. Zero business logic.
// This is the single file to look at when you want to know what URL does what.
package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"pokemontool/config"
	"pokemontool/handlers"
	"pokemontool/middleware"
)

// Register wires all routes onto the provided chi router
func Register(
	r chi.Router,
	cfg *config.Config,
	sse *handlers.SSEManager,
	auth *handlers.AuthHandler,
	cards *handlers.CardHandler,
	alerts *handlers.AlertHandler,
	watchlist *handlers.WatchlistHandler,
	inventory *handlers.InventoryHandler,
	deals *handlers.DealHandler,
	shows *handlers.ShowHandler,
	apikeys *handlers.APIKeyHandler,
) {
	// Health check — unauthenticated
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"pokemontool-go"}`))
	})

	r.Route("/api", func(r chi.Router) {
		// Public: Auth
		r.Post("/auth/register", auth.Register)
		r.Post("/auth/login",    auth.Login)

		// SSE stream (JWT via query param — EventSource can't set headers)
		r.Get("/stream", handlers.Stream(sse, cfg))

		// Protected: all routes below require a valid JWT
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(cfg))

			// Cards
			r.Get("/cards/search",          cards.Search)
			r.Get("/cards/trending",         cards.Trending)
			r.Get("/cards/{id}/history",     cards.PriceHistory)

			// Alerts
			r.Get("/alerts",                 alerts.List)
			r.Put("/alerts/read-all",        alerts.MarkAllRead)
			r.Put("/alerts/{id}/read",       alerts.MarkRead)
			r.Delete("/alerts/{id}",         alerts.Delete)

			// Watchlist
			r.Get("/watchlist",              watchlist.List)
			r.Post("/watchlist",             watchlist.Add)
			r.Delete("/watchlist/{id}",      watchlist.Remove)

			// Inventory
			r.Get("/inventory",              inventory.List)
			r.Post("/inventory",             inventory.Add)
			r.Delete("/inventory/{id}",      inventory.Delete)
			r.Post("/inventory/import",      inventory.Import)
			r.Get("/inventory/export",       inventory.Export)

			// Deals
			r.Get("/deals/today",            deals.Today)

			// Shows
			r.Get("/shows/upcoming",         shows.Upcoming)

			// API Keys
			r.Get("/apikeys",                apikeys.List)
			r.Post("/apikeys",               apikeys.Save)
			r.Delete("/apikeys/{platform}",  apikeys.Delete)
		})
	})
}
