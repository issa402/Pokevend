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
	"regexp"
	"sort"
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
	if len(payload.Cards) == 0 {
		return s.searchPokeTCGFallback(ctx, query, limit)
	}
	return payload.Cards, nil
}

func (s *CardService) searchPokeTCGFallback(ctx context.Context, query string, limit int) ([]models.PokeTCGCard, error) {
	tokens := searchTokens(query)
	if len(tokens) < 2 {
		return []models.PokeTCGCard{}, nil
	}

	if exactCards, err := s.searchOfficialCollectorCandidates(ctx, query, tokens, limit); err != nil {
		return nil, err
	} else if len(exactCards) > 0 {
		return exactCards, nil
	}

	candidateLimit := limit * 10
	if candidateLimit < 50 {
		candidateLimit = 50
	}
	if candidateLimit > 100 {
		candidateLimit = 100
	}

	seen := map[string]bool{}
	candidates := []models.PokeTCGCard{}
	for _, token := range candidateSearchTokens(tokens) {
		cards, err := s.fetchPokeTCG(ctx, token, candidateLimit)
		if err != nil {
			return nil, err
		}
		for _, card := range cards {
			if card.ID == "" || seen[card.ID] {
				continue
			}
			seen[card.ID] = true
			candidates = append(candidates, card)
		}
		if len(candidates) > 0 {
			break
		}
	}

	type scoredCard struct {
		card  models.PokeTCGCard
		score int
	}
	scored := []scoredCard{}
	for _, card := range candidates {
		score := scorePokeTCGCard(tokens, card)
		if score > 0 {
			scored = append(scored, scoredCard{card: card, score: score})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].card.Name < scored[j].card.Name
		}
		return scored[i].score > scored[j].score
	})

	results := []models.PokeTCGCard{}
	for _, item := range scored {
		results = append(results, item.card)
		if len(results) >= limit {
			break
		}
	}
	return results, nil
}

func (s *CardService) searchOfficialCollectorCandidates(ctx context.Context, query string, tokens []string, limit int) ([]models.PokeTCGCard, error) {
	nameTokens := candidateSearchTokens(tokens)
	if len(nameTokens) == 0 {
		return []models.PokeTCGCard{}, nil
	}
	number, denominator := collectorNumbers(query)
	if number == "" {
		return []models.PokeTCGCard{}, nil
	}

	cards, err := s.fetchOfficialPokemonTCG(ctx, nameTokens[0], number, 50)
	if err != nil {
		return nil, err
	}
	type scoredCard struct {
		card  models.PokeTCGCard
		score int
	}
	scored := []scoredCard{}
	for _, card := range cards {
		score := scorePokeTCGCard(tokens, card)
		if score == 0 {
			continue
		}
		if denominator != "" && card.SetPrintedTotal == denominator {
			score += 8
		}
		scored = append(scored, scoredCard{card: card, score: score})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].card.ID < scored[j].card.ID
		}
		return scored[i].score > scored[j].score
	})

	results := []models.PokeTCGCard{}
	for _, item := range scored {
		results = append(results, item.card)
		if len(results) >= limit {
			break
		}
	}
	return results, nil
}

func (s *CardService) fetchPokeTCG(ctx context.Context, query string, limit int) ([]models.PokeTCGCard, error) {
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
		return nil, fmt.Errorf("poketcg fallback search returned status %d", resp.StatusCode)
	}
	var payload struct {
		Cards []models.PokeTCGCard `json:"cards"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Cards == nil {
		return []models.PokeTCGCard{}, nil
	}
	return payload.Cards, nil
}

func (s *CardService) fetchOfficialPokemonTCG(ctx context.Context, cardName string, number string, limit int) ([]models.PokeTCGCard, error) {
	u, err := url.Parse("https://api.pokemontcg.io/v2/cards")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("q", fmt.Sprintf("name:\"%s\" number:\"%s\"", cardName, number))
	q.Set("pageSize", fmt.Sprintf("%d", limit))
	q.Set("orderBy", "name")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 PokeAi/1.0")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return []models.PokeTCGCard{}, nil
	}

	var payload officialPokemonTCGResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	cards := []models.PokeTCGCard{}
	for _, card := range payload.Data {
		cards = append(cards, card.toPokeTCGCard())
	}
	return cards, nil
}

func searchTokens(query string) []string {
	normalized := normalizeSearchText(query)
	normalized = strings.NewReplacer("-", " ", "/", " ", "#", " ").Replace(normalized)
	parts := strings.Fields(normalized)
	tokens := []string{}
	for _, part := range parts {
		if len(part) < 2 {
			continue
		}
		tokens = append(tokens, part)
	}
	return tokens
}

func collectorNumbers(query string) (string, string) {
	match := regexp.MustCompile(`(?i)\b(\d{1,4})\s*/\s*(\d{1,4})\b`).FindStringSubmatch(query)
	if len(match) == 3 {
		return match[1], match[2]
	}
	match = regexp.MustCompile(`(?i)\b(?:number|no\.?|#)\s*(\d{1,4})\b`).FindStringSubmatch(query)
	if len(match) == 2 {
		return match[1], ""
	}
	for _, token := range searchTokens(query) {
		if isNumericToken(token) {
			return token, ""
		}
	}
	return "", ""
}

func candidateSearchTokens(tokens []string) []string {
	searchTokens := []string{}
	seen := map[string]bool{}
	for _, token := range tokens {
		if len(token) < 3 || isNumericToken(token) || isSetOnlyToken(token) || seen[token] {
			continue
		}
		seen[token] = true
		searchTokens = append(searchTokens, token)
		if len(searchTokens) >= 2 {
			break
		}
	}
	return searchTokens
}

func isSetOnlyToken(token string) bool {
	switch token {
	case "pokemon", "pokémon", "card", "cards", "set", "series", "base", "promo", "promos":
		return true
	default:
		return false
	}
}

func normalizeSearchText(value string) string {
	return strings.NewReplacer(
		"é", "e", "É", "e", "è", "e", "È", "e",
		"—", " ", "–", " ",
	).Replace(strings.ToLower(value))
}

func isScoringStopword(token string) bool {
	switch token {
	case "pokemon", "pokémon", "card", "cards", "set", "series", "promo", "promos":
		return true
	default:
		return false
	}
}

func isNumericToken(token string) bool {
	if token == "" {
		return false
	}
	for _, ch := range token {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func scorePokeTCGCard(tokens []string, card models.PokeTCGCard) int {
	name := normalizeSearchText(card.Name)
	setName := normalizeSearchText(card.Set)
	number := normalizeSearchText(card.Number)
	score := 0
	for _, token := range tokens {
		switch {
		case isScoringStopword(token):
			continue
		case token == number:
			score += 4
		case strings.Contains(name, token):
			score += 5
		case strings.Contains(setName, token):
			score += 4
		case isNumericToken(token):
			continue
		default:
			return 0
		}
	}
	if card.Market != nil {
		score++
	}
	if card.CardmarketTrend != nil {
		score++
	}
	return score
}

func (s *CardService) SearchEbayListings(ctx context.Context, cardName, externalCardID, setName, cardNumber, assetType, slabTier, languagePreference string, pages int, publish bool) ([]models.EbayLiveListing, error) {
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
	q.Set("publish", fmt.Sprintf("%t", publish))
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
	if cardNumber != "" {
		q.Set("cardNumber", cardNumber)
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

func (s *CardService) ImportEbayListingText(ctx context.Context, request models.EbayTextImportRequest) ([]models.EbayLiveListing, error) {
	if strings.TrimSpace(request.CardName) == "" {
		return nil, ErrCardNameRequired
	}
	if strings.TrimSpace(request.Text) == "" {
		return nil, fmt.Errorf("import text is required")
	}
	if request.AssetType == "" {
		request.AssetType = "SLAB"
	}
	request.LanguagePreference = normalizeLanguagePreference(request.LanguagePreference)

	u, err := url.Parse(s.apiConsumerURL + "/ebay/import-text")
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ebay text import returned status %d", resp.StatusCode)
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

type officialPokemonTCGResponse struct {
	Data []officialPokemonTCGCard `json:"data"`
}

type officialPokemonTCGCard struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Number string `json:"number"`
	Rarity string `json:"rarity"`
	Set    struct {
		Name         string `json:"name"`
		Series       string `json:"series"`
		PrintedTotal int    `json:"printedTotal"`
	} `json:"set"`
	Images struct {
		Small string `json:"small"`
		Large string `json:"large"`
	} `json:"images"`
	TCGPlayer *struct {
		URL       string                        `json:"url"`
		UpdatedAt string                        `json:"updatedAt"`
		Prices    map[string]map[string]float64 `json:"prices"`
	} `json:"tcgplayer"`
	Cardmarket *struct {
		URL       string `json:"url"`
		UpdatedAt string `json:"updatedAt"`
		Prices    struct {
			TrendPrice *float64 `json:"trendPrice"`
		} `json:"prices"`
	} `json:"cardmarket"`
}

func (c officialPokemonTCGCard) toPokeTCGCard() models.PokeTCGCard {
	image := c.Images.Large
	if image == "" {
		image = c.Images.Small
	}
	bestVariant, market := bestOfficialMarket(c)
	result := models.PokeTCGCard{
		ID:              c.ID,
		Name:            c.Name,
		Set:             c.Set.Name,
		Series:          c.Set.Series,
		Number:          c.Number,
		Rarity:          c.Rarity,
		Image:           image,
		BestVariant:     bestVariant,
		Market:          market,
		SetPrintedTotal: fmt.Sprintf("%d", c.Set.PrintedTotal),
	}
	if c.TCGPlayer != nil {
		result.TCGPlayerURL = c.TCGPlayer.URL
		result.TCGPlayerUpdatedAt = c.TCGPlayer.UpdatedAt
	}
	if c.Cardmarket != nil {
		result.CardmarketURL = c.Cardmarket.URL
		result.CardmarketUpdatedAt = c.Cardmarket.UpdatedAt
		result.CardmarketTrend = c.Cardmarket.Prices.TrendPrice
	}
	return result
}

func bestOfficialMarket(c officialPokemonTCGCard) (string, *float64) {
	if c.TCGPlayer == nil {
		return "-", nil
	}
	bestName := "-"
	var bestValue *float64
	for variant, fields := range c.TCGPlayer.Prices {
		for _, key := range []string{"market", "mid", "low"} {
			value, ok := fields[key]
			if !ok || value <= 0 {
				continue
			}
			if bestValue == nil || value > *bestValue {
				copyValue := value
				bestValue = &copyValue
				bestName = variant
			}
			break
		}
	}
	return bestName, bestValue
}
