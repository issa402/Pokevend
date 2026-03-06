// ============================================================
// FILE: server/models/card.go
// TYPE: Models Layer — Domain Types
//
// WHAT IS THE MODELS LAYER?
// Models are pure data structures — no business logic, no database queries,
// no HTTP code. They define the SHAPE of your data.
//
// FAANG PATTERN: "Anemic Domain Model" (intentional for this architecture)
// In our FAANG-level layered architecture, models are deliberately simple.
// Business logic lives in services/, SQL lives in store/.
// Models are just typed containers for moving data between layers.
//
// GO STRUCT TAGS:
// The `json:"cardId"` tag tells the JSON encoder what key to use.
// Without a tag: Go serializes "CardID" (the Go field name)
// With `json:"cardId"`: it serializes as "cardId" (camelCase for JS clients)
// With `json:"-"`: completely exclude from JSON (used for passwords!)
//
// POINTER TYPES (*string, *float64):
// A *float64 (pointer to float64) can be nil — meaning "no value."
// A float64 cannot be nil — it defaults to 0 even if DB returned NULL.
// We use pointers for nullable database columns.
// If price_ebay is NULL in DB → PriceEbay is nil in Go → "null" in JSON
// If you used float64: NULL in DB → 0.0 in Go → wrong data in JSON!
// ============================================================
package models

import "time"

// Card represents a Pokémon card in our system.
// This is populated from the cards table by store/card_store.go.
// The json tags match what the React frontend expects.
type Card struct {
	// card_id in PostgreSQL — the platform's product identifier
	CardID string `json:"cardId"`
	Name   string `json:"name"`

	// Nullable strings — use *string because set_name can be NULL in DB
	SetName *string `json:"setName"`
	SetCode *string `json:"setCode"`
	ImageURL *string `json:"imageUrl"`

	// Trend data computed by Python analytics-engine
	// trending_score: -100 (falling hard) to +100 (rising hard)
	TrendingScore int    `json:"trendingScore"`
	TrendLabel    string `json:"trendLabel"` // "RISING" | "FALLING" | "STABLE"

	// pct_change_7d = ((price_now - price_7days_ago) / price_7days_ago) * 100
	PctChange7d *float64 `json:"pctChange7d"` // nullable — new cards have no history

	// Current prices per marketplace — all nullable (not all markets have listings)
	PriceEbay     *float64 `json:"priceEbay"`
	PriceTCG      *float64 `json:"priceTcgplayer"`
	PriceFacebook *float64 `json:"priceFacebook"`
	PriceMercari  *float64 `json:"priceMercari"`

	// *time.Time = pointer so it can be nil (no update yet)
	LastUpdated *time.Time `json:"lastUpdated"`
}

// PricePoint represents one day of price history for a card.
// Used by the Chart.js chart on the card detail page.
// Stored in the price_history table and read by store/card_store.go.
type PricePoint struct {
	Date     string   `json:"date"`           // "2024-01-15" (DATE type from PostgreSQL)
	AvgPrice *float64 `json:"avgPrice"`       // average across all marketplaces
	PriceEbay *float64 `json:"priceEbay"`
	PriceTCG  *float64 `json:"priceTcgplayer"`
}

// TODO #1 (Practice): Add a CardSearchResult type
// Our Search endpoint returns the same Card struct, but search results
// often need extra fields like "relevance score" or "matched field."
// Create a CardSearchResult struct that embeds Card and adds:
//   MatchedOn  string  `json:"matchedOn"` // "name" or "setName"
//   Relevance  float64 `json:"relevance"` // 0.0-1.0
// GO CONCEPT: Struct embedding — embed Card inside CardSearchResult
//   type CardSearchResult struct {
//     Card                             // embedded = all Card fields available
//     MatchedOn string `json:"matchedOn"`
//   }

// TODO #2 (Practice): Add a MarketSummary method
// Add a method on Card that returns the best available price across all markets:
//   func (c *Card) BestPrice() *float64
// It should return the minimum of PriceEbay, PriceTCG, PriceFacebook, PriceMercari
// (ignoring nil values). This is useful business logic... but wait — should it
// really be on the model? Think about it: does this logic belong in Card (model)
// or in CardService (service)? Discuss with yourself before implementing.
