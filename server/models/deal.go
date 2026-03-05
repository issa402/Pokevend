// models/deal.go — Deal domain type
package models

type Deal struct {
	CardName    string   `json:"cardName"`
	SetName     *string  `json:"setName"`
	ImageURL    *string  `json:"imageUrl"`
	MarketPrice float64  `json:"marketPrice"`
	BestPrice   float64  `json:"bestPrice"`
	Savings     float64  `json:"savings"`
	SavingsPct  float64  `json:"savingsPct"`
	ListingURL  *string  `json:"listingUrl"`
	Marketplace string   `json:"marketplace"`
	Reason      *string  `json:"reason"`
}
