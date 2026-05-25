// ============================================================
// FILE: server/services/card_service.go
// TYPE: Service Layer — Card Business Logic
//
// WHAT IS THIS?
// Business logic for card operations: search, trending, price history.
// Adds Redis caching on top of PostgreSQL queries.
//
// CACHING STRATEGY IMPLEMENTED HERE:
//
//	Search results:   5-minute TTL  (users re-search, slight staleness OK)
//	Trending cards:  30-minute TTL  (batch-computed, expensive to fetch)
//
// CACHE KEY FORMAT: "resource:qualifier"
//
//	"search:charizard"   → search for "charizard"
//	"trending:rising"    → top 10 rising cards
//	"trending:falling"   → top 10 falling cards
//
// WHY CACHE IN THE SERVICE, NOT THE STORE OR HANDLER?
// Service: RIGHT LEVEL. Business rule: "trending cards can be 30 min stale."
// Store: too low. Store's job is SQL, not caching.
// Handler: too high. Handler's job is HTTP, not caching.
// The caching strategy is a BUSINESS DECISION → belongs in service.
//
// GO CONCEPTS:
//
//	redis.Client.Get/Set, json.Marshal/Unmarshal, time.Duration,
//	errors.Is(err, redis.Nil) to detect cache miss
//
// ============================================================
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"pokemontool/models"
	"pokemontool/store"
)

// CardService handles card business logic with Redis caching.
type CardService struct {
	store          store.CardStore // PostgreSQL queries
	cache          *redis.Client   // Redis for caching
	pokeTCGBaseURL string
	apiConsumerURL string
	httpClient     *http.Client
}

// NewCardService is the constructor — injects store (DB) and cache (Redis).
func NewCardService(store store.CardStore, cache *redis.Client, pokeTCGBaseURL string, apiConsumerURL string) *CardService {
	return &CardService{
		store:          store,
		cache:          cache,
		pokeTCGBaseURL: strings.TrimRight(pokeTCGBaseURL, "/"),
		apiConsumerURL: strings.TrimRight(apiConsumerURL, "/"),
		httpClient:     &http.Client{Timeout: 15 * time.Second},
	}
}

// Search finds cards by name with a 5-minute cache.
// First checks Redis; on miss, queries PostgreSQL and caches the result.
func (s *CardService) Search(ctx context.Context, query string, limit int) ([]models.Card, error) {
	// ── Try cache first ────────────────────────────────────────
	cacheKey := "search:" + query
	// cache.Get returns the cached JSON bytes if key exists
	if cached, err := s.cache.Get(ctx, cacheKey).Bytes(); err == nil {
		// Cache HIT — deserialize JSON back into []models.Card and return immediately
		var cards []models.Card
		if json.Unmarshal(cached, &cards) == nil { // if JSON is valid
			return cards, nil // return fast (~0.1ms vs ~5ms for DB)
		}
	}
	// errors.Is(err, redis.Nil) would be true if key not found (cache miss)
	// We don't need to check because any error → fall through to DB query

	// ── Cache MISS — query PostgreSQL ─────────────────────────
	cards, err := s.store.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	// ── Cache the result ───────────────────────────────────────
	// json.Marshal converts []models.Card → JSON bytes
	if bytes, err := json.Marshal(cards); err == nil {
		// cache.Set(ctx, key, value, TTL) — expires after 5 minutes
		// After TTL expires, next request is a cache miss → fresh DB query
		s.cache.Set(ctx, cacheKey, bytes, 5*time.Minute)
	}
	return cards, nil
}

// GetTrending returns top rising and falling cards with 30-minute cache.
// These are expensive to compute (Python runs linear regression) so we cache aggressively.
func (s *CardService) GetTrending(ctx context.Context) (rising, falling []models.Card, err error) {
	// Try rising cache
	if cached, err := s.cache.Get(ctx, "trending:rising").Bytes(); err == nil {
		json.Unmarshal(cached, &rising)
	}
	// Try falling cache
	if cached, err := s.cache.Get(ctx, "trending:falling").Bytes(); err == nil {
		json.Unmarshal(cached, &falling)
	}
	// If BOTH caches hit, return immediately
	if len(rising) > 0 && len(falling) > 0 {
		return rising, falling, nil
	}

	// Cache miss — fetch from PostgreSQL
	rising, falling, err = s.store.GetTrending(ctx)
	if err != nil {
		return nil, nil, err
	}

	// Cache both results for 30 minutes
	if bytes, e := json.Marshal(rising); e == nil {
		s.cache.Set(ctx, "trending:rising", bytes, 30*time.Minute)
	}
	if bytes, e := json.Marshal(falling); e == nil {
		s.cache.Set(ctx, "trending:falling", bytes, 30*time.Minute)
	}
	return rising, falling, nil
}

// GetPriceHistory returns price history — no cache (chart data needs to be fresh).
func (s *CardService) GetPriceHistory(ctx context.Context, cardID string) ([]models.PricePoint, error) {
	return s.store.GetPriceHistory(ctx, cardID)
}

func (s *CardService) GetSlabMarketSummary(ctx context.Context, externalCardID string, languagePreference string) ([]models.SlabMarketSummary, error) {
	return s.store.GetSlabMarketSummary(ctx, externalCardID, normalizeLanguagePreference(languagePreference))
}

func (s *CardService) GetByID(ctx context.Context, id string) (*models.Card, error) {
	return s.store.GetByID(ctx, id)
}

// SearchPokeTCG returns real market-data-backed card variants from the PokeTCG service.
func (s *CardService) SearchPokeTCG(ctx context.Context, query string, limit int) ([]models.PokeTCGCard, error) {
	if strings.TrimSpace(query) == "" {
		return nil, ErrCardNameRequired
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	u, err := url.Parse(s.pokeTCGBaseURL + "/search")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("q", query)
	q.Set("limit", fmt.Sprintf("%d", limit))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("poketcg search returned status %d", resp.StatusCode)
	}

	var payload struct {
		Cards []models.PokeTCGCard `json:"cards"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Cards == nil {
		payload.Cards = []models.PokeTCGCard{}
	}
	return payload.Cards, nil
}

func (s *CardService) SearchEbayListings(ctx context.Context, cardName, externalCardID, setName, assetType, slabTier, languagePreference string, pages int) ([]models.EbayLiveListing, error) {
	if strings.TrimSpace(cardName) == "" {
		return nil, ErrCardNameRequired
	}
	if assetType == "" {
		assetType = "RAW"
	}

	u, err := url.Parse(s.apiConsumerURL + "/ebay/search")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("cardName", cardName)
	q.Set("assetType", assetType)
	q.Set("publish", "false")
	q.Set("languagePreference", normalizeLanguagePreference(languagePreference))
	if pages <= 0 || pages > 5 {
		pages = 2
	}
	q.Set("pages", fmt.Sprintf("%d", pages))
	if externalCardID != "" {
		q.Set("externalCardId", externalCardID)
	}
	if setName != "" {
		q.Set("setName", setName)
	}
	if slabTier != "" {
		q.Set("slabTier", slabTier)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ebay live search returned status %d", resp.StatusCode)
	}
	var payload struct {
		Listings []models.EbayLiveListing `json:"listings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Listings == nil {
		payload.Listings = []models.EbayLiveListing{}
	}
	return payload.Listings, nil
}

// InvalidateTrendingCache clears the trending cache.
// Call this after Python updates cards (e.g., via a webhook from Python → Go admin endpoint).
func (s *CardService) InvalidateTrendingCache(ctx context.Context) {
	s.cache.Del(ctx, "trending:rising", "trending:falling")
}

// TODO #1 (Practice): Add cache invalidation for search
// When a card's price is updated by the analytics engine, cached search
// results for that card become stale. Add:
//   func (s *CardService) InvalidateSearch(ctx context.Context, cardName string)
//     s.cache.Del(ctx, "search:"+strings.ToLower(cardName))
// Call it from a new admin endpoint: POST /api/admin/invalidate-cache

// TODO #2 (Practice): Add GetCardDetail with caching
// The card detail page fetches a single card by ID + its price history.
// Add: func (s *CardService) GetDetail(ctx, cardID string) (*models.Card, []models.PricePoint, error)
// Cache the card detail: "card:"+cardID with 10-minute TTL
// Why 10 minutes (shorter than trending)? User just searched for this card —
// they expect relatively fresh price data on the detail page.
