// ============================================================
// FILE: server/handlers/card_handler.go
// TYPE: Handler Layer — Card HTTP Endpoints
//
// WHAT IS THIS?
// HTTP handler for all card-related endpoints.
// Thin by design — reads request, calls service, writes response.
//
// ROUTES THIS HANDLER SERVES (from routes/routes.go):
//
//	GET /api/cards/search?q=charizard&limit=20
//	GET /api/cards/trending
//	GET /api/cards/{id}/history
//
// WHY chi.URLParam AND r.URL.Query()?
//
//	chi.URLParam(r, "id")   = extracts from the URL PATH: /api/cards/{id}
//	r.URL.Query().Get("q")  = extracts from QUERY STRING: ?q=charizard
//
// Both are URL reading — but from different parts of the URL.
//
// PATH PARAMETER:   /api/cards/abc-123   → chi.URLParam(r, "id") = "abc-123"
// QUERY PARAMETER:  /api/cards?q=char    → r.URL.Query().Get("q") = "char"
//
// Use path params for RESOURCE IDENTIFIERS (unique IDs).
// Use query params for FILTERS, PAGINATION, SEARCH TERMS.
// ============================================================
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"pokemontool/models"
	"pokemontool/pkg"
	"pokemontool/services"

	"golang.org/x/sync/errgroup"
)

// CardHandler handles all card HTTP requests.
type CardHandler struct {
	svc *services.CardService
}

// NewCardHandler is the constructor. main.go calls: handlers.NewCardHandler(cardSvc)
func NewCardHandler(svc *services.CardService) *CardHandler {
	return &CardHandler{svc: svc}
}

// Search handles GET /api/cards/search?q=charizard&limit=20
// Query params: q (required search term), limit (optional, default 20)
func (h *CardHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q") // ?q=charizard → "charizard"
	if q == "" {
		pkg.Error(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	// strconv.Atoi converts string "20" to int 20
	// Returns (int, error) — if conversion fails (not a number), use default 20
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n // only accept valid range 1-100
		}
	}

	cards, err := h.svc.Search(r.Context(), q, limit)
	if err != nil {
		pkg.Error(w, http.StatusInternalServerError, "search failed")
		return
	}
	// Even if no cards found, return empty array (not 404)
	// 404 = the ENDPOINT doesn't exist. Empty array = endpoint exists, no results.
	if cards == nil {
		cards = []models.Card{} // return [] not null in JSON
	}
	pkg.JSON(w, http.StatusOK, cards)
}

// TCGSearch handles GET /api/cards/tcg-search?q=charizard&limit=20.
// It searches the real market-data service, returning exact card/set variants.
func (h *CardHandler) TCGSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		pkg.Error(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	cards, err := h.svc.SearchPokeTCG(r.Context(), q, limit)
	if err != nil {
		pkg.Error(w, http.StatusBadGateway, "market data search failed")
		return
	}
	pkg.JSON(w, http.StatusOK, map[string]interface{}{
		"cards": cards,
		"count": len(cards),
		"query": q,
	})
}

// Trending handles GET /api/cards/trending
// Returns top 10 rising and top 10 falling cards.
func (h *CardHandler) Trending(w http.ResponseWriter, r *http.Request) {
	rising, falling, err := h.svc.GetTrending(r.Context())
	if err != nil {
		pkg.Error(w, http.StatusInternalServerError, "failed to load trending cards")
		return
	}
	// Return as an object with two arrays — frontend destructures this
	pkg.JSON(w, http.StatusOK, map[string]interface{}{
		"rising":  rising,
		"falling": falling,
	})
}

// PriceHistory handles GET /api/cards/{id}/history
// Returns 30 days of price data for charting.
func (h *CardHandler) PriceHistory(w http.ResponseWriter, r *http.Request) {
	// chi.URLParam extracts the {id} from the URL path
	// Route: /api/cards/{id}/history → URLParam(r, "id") = "ch-1"
	cardID := chi.URLParam(r, "id")
	if cardID == "" {
		pkg.Error(w, http.StatusBadRequest, "card id required")
		return
	}

	history, err := h.svc.GetPriceHistory(r.Context(), cardID)
	if err != nil {
		pkg.Error(w, http.StatusInternalServerError, "failed to load price history")
		return
	}
	pkg.JSON(w, http.StatusOK, history)
}

func (h *CardHandler) SlabSummary(w http.ResponseWriter, r *http.Request) {
	cardID := chi.URLParam(r, "id")
	if cardID == "" {
		pkg.Error(w, http.StatusBadRequest, "card id required")
		return
	}

	summary, err := h.svc.GetSlabMarketSummary(r.Context(), cardID, r.URL.Query().Get("languagePreference"))
	if err != nil {
		pkg.Error(w, http.StatusInternalServerError, "failed to load slab summary")
		return
	}
	pkg.JSON(w, http.StatusOK, map[string]interface{}{
		"cardId":  cardID,
		"summary": summary,
	})
}

func (h *CardHandler) EbayListings(w http.ResponseWriter, r *http.Request) {
	cardName := r.URL.Query().Get("cardName")
	if cardName == "" {
		pkg.Error(w, http.StatusBadRequest, "query parameter 'cardName' is required")
		return
	}

	listings, err := h.svc.SearchEbayListings(
		r.Context(),
		cardName,
		r.URL.Query().Get("externalCardId"),
		r.URL.Query().Get("setName"),
		r.URL.Query().Get("cardNumber"),
		r.URL.Query().Get("assetType"),
		r.URL.Query().Get("slabTier"),
		r.URL.Query().Get("languagePreference"),
		intQuery(r, "pages", 2, 1, 5),
		r.URL.Query().Get("publish") == "true",
	)
	if err != nil {
		pkg.Error(w, http.StatusBadGateway, "ebay listing search failed")
		return
	}
	pkg.JSON(w, http.StatusOK, map[string]interface{}{
		"count":    len(listings),
		"listings": listings,
	})
}

func (h *CardHandler) ImportEbayText(w http.ResponseWriter, r *http.Request) {
	var request models.EbayTextImportRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		pkg.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if request.CardName == "" {
		pkg.Error(w, http.StatusBadRequest, "cardName is required")
		return
	}
	if request.Text == "" {
		pkg.Error(w, http.StatusBadRequest, "text is required")
		return
	}
	listings, err := h.svc.ImportEbayListingText(r.Context(), request)
	if err != nil {
		pkg.Error(w, http.StatusBadGateway, "ebay text import failed")
		return
	}
	pkg.JSON(w, http.StatusOK, map[string]interface{}{
		"count":    len(listings),
		"listings": listings,
	})
}

func intQuery(r *http.Request, key string, defaultValue int, minValue int, maxValue int) int {
	value := defaultValue
	if raw := r.URL.Query().Get(key); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			value = parsed
		}
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

// TODO #1 (Practice): Add GetCardDetail handler
// Add: GET /api/cards/{id} → returns full card info for the detail page
// Route registers as: r.Get("/cards/{id}", cards.Detail)
// Handler: fetch card + price_history in parallel using goroutines
// Research: "Go errgroup" — concurrent requests with shared error handling
// import golang.org/x/sync/errgroup → runs multiple goroutines, collects errors
func (h *CardHandler) Detail(w http.ResponseWriter, r *http.Request) {
	cardID := chi.URLParam(r, "id")

	g, ctx := errgroup.WithContext(r.Context())
	var card *models.Card
	var history []models.PricePoint

	g.Go(func() error {
		var err error
		// We call the SERVICE, not the store.
		card, err = h.svc.GetByID(ctx, cardID)
		return err // If this fails, the whole group stops
	})

	g.Go(func() error {
		var err error
		history, err = h.svc.GetPriceHistory(ctx, cardID)
		return err
	})

	if err := g.Wait(); err != nil {
		pkg.Error(w, http.StatusNotFound, "Card not found")
		return
	}
	pkg.JSON(w, http.StatusOK, map[string]interface{}{
		"info":    card,
		"history": history,
	})

}

// TODO #2 (Practice): Add pagination to Search
// Currently limit=100 is the max — for large results, implement cursor pagination:
//   GET /api/cards/search?q=charizard&limit=20&cursor=eyJpZCI6Ii4uLiJ9
// cursor = base64-encoded last seen card_id from previous page
// SQL: WHERE name ILIKE $1 AND card_id > $2 ORDER BY card_id LIMIT $3
// This "keyset pagination" is more efficient than OFFSET at large page numbers.
// Research: "keyset pagination vs offset pagination" — FAANG standard
