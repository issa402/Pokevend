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
	SetName  *string `json:"setName"`
	SetCode  *string `json:"setCode"`
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
	Date      string   `json:"date"`     // "2024-01-15" (DATE type from PostgreSQL)
	AvgPrice  *float64 `json:"avgPrice"` // average across all marketplaces
	PriceEbay *float64 `json:"priceEbay"`
	PriceTCG  *float64 `json:"priceTcgplayer"`
}

type ListingSnapshot struct {
	CardName           string
	ExternalCardID     *string
	Marketplace        string
	Price              float64
	ListingURL         *string
	ListingTitle       *string
	ImageURL           *string
	Condition          *string
	ListingID          *string
	SetName            *string
	IsSlab             bool
	Grader             *string
	Grade              *string
	SlabTier           string
	LanguagePreference string
}

type SlabListing struct {
	Price              float64    `json:"price"`
	ListingURL         *string    `json:"listingUrl"`
	ListingID          *string    `json:"listingId"`
	ListingTitle       *string    `json:"listingTitle"`
	ImageURL           *string    `json:"imageUrl"`
	Condition          *string    `json:"condition"`
	ObservedAt         *time.Time `json:"observedAt"`
	LanguagePreference string     `json:"languagePreference"`
}

type EbayLiveListing struct {
	CardName           string  `json:"card_name"`
	ExternalCardID     *string `json:"external_card_id"`
	Price              float64 `json:"price"`
	Marketplace        string  `json:"marketplace"`
	ListingURL         string  `json:"listing_url"`
	ListingID          *string `json:"listing_id"`
	ListingTitle       *string `json:"listing_title"`
	ImageURL           *string `json:"image_url"`
	Condition          *string `json:"condition"`
	SetName            *string `json:"set_name"`
	IsSlab             bool    `json:"is_slab"`
	Grader             *string `json:"grader"`
	Grade              *string `json:"grade"`
	SlabTier           string  `json:"slab_tier"`
	CertNumber         *string `json:"cert_number"`
	LanguagePreference string  `json:"language_preference"`
	ScrapedAt          *string `json:"scraped_at"`
}

type SlabMarketSummary struct {
	SlabTier    string        `json:"slabTier"`
	Label       string        `json:"label"`
	LowestPrice *float64      `json:"lowestPrice"`
	ListingURL  *string       `json:"listingUrl"`
	ListingID   *string       `json:"listingId"`
	ObservedAt  *time.Time    `json:"observedAt"`
	Count       int           `json:"count"`
	Listings    []SlabListing `json:"listings"`
}

// PokeTCGCard is a market-data search result from the local PokeTCG/PokeAi service.
// It represents an exact external card variant, not a row from our cards table.
type PokeTCGCard struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Set                 string   `json:"set"`
	Series              string   `json:"series"`
	Number              string   `json:"number"`
	Rarity              string   `json:"rarity"`
	Image               string   `json:"image"`
	BestVariant         string   `json:"bestVariant"`
	Market              *float64 `json:"market"`
	TCGPlayerUpdatedAt  string   `json:"tcgplayerUpdatedAt"`
	TCGPlayerURL        string   `json:"tcgplayerUrl"`
	CardmarketTrend     *float64 `json:"cardmarketTrend"`
	CardmarketUpdatedAt string   `json:"cardmarketUpdatedAt"`
	CardmarketURL       string   `json:"cardmarketUrl"`
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
