// models/card.go — Card domain types
package models

import "time"

type Card struct {
	CardID        string     `json:"cardId"`
	Name          string     `json:"name"`
	SetName       *string    `json:"setName"`
	SetCode       *string    `json:"setCode"`
	ImageURL      *string    `json:"imageUrl"`
	TrendingScore int        `json:"trendingScore"`
	TrendLabel    string     `json:"trendLabel"` // RISING | FALLING | STABLE
	PctChange7d   *float64   `json:"pctChange7d"`
	PriceEbay     *float64   `json:"priceEbay"`
	PriceTCG      *float64   `json:"priceTcgplayer"`
	PriceFacebook *float64   `json:"priceFacebook"`
	PriceMercari  *float64   `json:"priceMercari"`
	LastUpdated   *time.Time `json:"lastUpdated"`
}

type PricePoint struct {
	Date      string   `json:"date"`
	AvgPrice  *float64 `json:"avgPrice"`
	PriceEbay *float64 `json:"priceEbay"`
	PriceTCG  *float64 `json:"priceTcgplayer"`
}
