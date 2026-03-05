// models/alert.go — Alert domain type
package models

import "time"

type Alert struct {
	ID          string     `json:"id"`
	UserID      string     `json:"userId"`
	CardName    *string    `json:"cardName"`
	AlertType   string     `json:"alertType"` // PRICE_DROP | PRICE_SPIKE | TREND_CHANGE | DEAL_OF_DAY
	Message     string     `json:"message"`
	Marketplace *string    `json:"marketplace"`
	Price       *float64   `json:"price"`
	ListingURL  *string    `json:"listingUrl"`
	IsRead      bool       `json:"isRead"`
	CreatedAt   time.Time  `json:"createdAt"`
}
