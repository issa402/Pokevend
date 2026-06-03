package models

import (
	"encoding/json"
	"time"
)

type SlabOpportunity struct {
	ID                   string          `json:"id"`
	ExternalCardID       *string         `json:"externalCardId"`
	CardName             string          `json:"cardName"`
	SetName              *string         `json:"setName"`
	Grader               *string         `json:"grader"`
	Grade                *string         `json:"grade"`
	SlabTier             *string         `json:"slabTier"`
	Marketplace          *string         `json:"marketplace"`
	ListingID            *string         `json:"listingId"`
	ListingURL           *string         `json:"listingUrl"`
	Title                *string         `json:"title"`
	AskingPrice          float64         `json:"askingPrice"`
	ShippingPrice        float64         `json:"shippingPrice"`
	EstimatedFees        float64         `json:"estimatedFees"`
	AllInCost            float64         `json:"allInCost"`
	EstimatedMarketValue float64         `json:"estimatedMarketValue"`
	ExpectedProfit       float64         `json:"expectedProfit"`
	ExpectedMarginPct    float64         `json:"expectedMarginPct"`
	LiquidityScore       int             `json:"liquidityScore"`
	ConfidenceScore      int             `json:"confidenceScore"`
	RiskScore            int             `json:"riskScore"`
	DealScore            int             `json:"dealScore"`
	Decision             string          `json:"decision"`
	Reason               *string         `json:"reason"`
	Evidence             json.RawMessage `json:"evidence"`
	SellerHubMetrics     json.RawMessage `json:"sellerHubMetrics"`
	ApprovedInventoryID  *string         `json:"approvedInventoryId"`
	CreatedAt            time.Time       `json:"createdAt"`
	UpdatedAt            time.Time       `json:"updatedAt"`
}

type SlabOpportunityFilters struct {
	Decision     string
	Grader       string
	Grade        string
	SlabTier     string
	Marketplace  string
	CardName     string
	MinMarginPct float64
	Signal       string
	Limit        int
}
