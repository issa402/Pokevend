// models/watchlist.go — WatchlistItem domain type
package models

import "time"

type WatchlistItem struct {
	ID                 string    `json:"id"`
	UserID             string    `json:"userId"`
	CardName           string    `json:"cardName"`
	SetName            *string   `json:"setName"`
	ExternalCardID     *string   `json:"externalCardId"`
	CardNumber         *string   `json:"cardNumber"`
	Rarity             *string   `json:"rarity"`
	ImageURL           *string   `json:"imageUrl"`
	MarketPrice        *float64  `json:"marketPrice"`
	MarketUpdatedAt    *string   `json:"marketUpdatedAt"`
	TargetDiscountPct  *float64  `json:"targetDiscountPct"`
	PriceSource        *string   `json:"priceSource"`
	TargetBuyPrice     *float64  `json:"targetBuyPrice"`
	TargetSellPrice    *float64  `json:"targetSellPrice"`
	Notes              *string   `json:"notes"`
	AssetType          string    `json:"assetType"`
	Grader             *string   `json:"grader"`
	Grade              *string   `json:"grade"`
	SlabTier           *string   `json:"slabTier"`
	LanguagePreference string    `json:"languagePreference"`
	AddedAt            time.Time `json:"addedAt"`
}

type WatchlistScanTarget struct {
	CardName           string  `json:"cardName"`
	SetName            *string `json:"setName"`
	ExternalCardID     *string `json:"externalCardId"`
	AssetType          string  `json:"assetType"`
	SlabTier           *string `json:"slabTier"`
	LanguagePreference string  `json:"languagePreference"`
}
