// models/inventory.go — InventoryItem domain type
package models

import "time"


type InventoryItem struct {
	ID            string     `json:"id"`
	UserID        string     `json:"userId"`
	CardName      string     `json:"cardName"`
	SetName       *string    `json:"setName"`
	CardNumber    *string    `json:"cardNumber"`
	Condition     *string    `json:"condition"` // NM, LP, MP, HP, Damaged
	Quantity      int        `json:"quantity"`
	PurchasePrice *float64   `json:"purchasePrice"`
	CurrentValue  *float64   `json:"currentValue"`
	Notes         *string    `json:"notes"`
	AcquiredAt    time.Time  `json:"acquiredAt"`
}
