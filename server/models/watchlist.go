// models/watchlist.go — WatchlistItem domain type
package models

import "time"

type WatchlistItem struct {
	ID              string     `json:"id"`
	UserID          string     `json:"userId"`
	CardName        string     `json:"cardName"`
	SetName         *string    `json:"setName"`
	TargetBuyPrice  *float64   `json:"targetBuyPrice"`
	TargetSellPrice *float64   `json:"targetSellPrice"`
	Notes           *string    `json:"notes"`
	AddedAt         time.Time  `json:"addedAt"`
}
