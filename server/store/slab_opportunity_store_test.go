package store

import (
	"encoding/json"
	"testing"
	"time"

	"pokemontool/models"
)

func TestBuildVendorInsightRanksProfitableDemandBackedExactListing(t *testing.T) {
	opportunity := models.SlabOpportunity{
		AskingPrice:          120,
		AllInCost:            136.80,
		EstimatedMarketValue: 220,
		ExpectedProfit:       83.20,
		ExpectedMarginPct:    60.82,
		ConfidenceScore:      70,
		RiskScore:            15,
		Evidence:             json.RawMessage(`{"signal":"BUY_CANDIDATE","needsEbayScan":false,"trendChange":25}`),
		SellerHubMetrics: json.RawMessage(`{
			"active":{"totalListings":10,"avgListingPrice":210,"avgWatchers":12,"maxWatchers":35,"researchedAt":"2026-06-04T12:00:00Z"},
			"sold":{"totalListings":20,"avgListingPrice":225,"avgBids":3,"maxBids":8,"researchedAt":"2026-06-04T12:00:00Z"}
		}`),
	}

	insight := buildVendorInsight(opportunity, time.Date(2026, 6, 4, 17, 0, 0, 0, time.UTC))

	if insight.Action != "SOURCE_NOW" {
		t.Fatalf("expected SOURCE_NOW, got %s", insight.Action)
	}
	if insight.OpportunityScore < 70 {
		t.Fatalf("expected high opportunity score, got %d", insight.OpportunityScore)
	}
	if insight.SellThroughRate == nil || *insight.SellThroughRate < 60 {
		t.Fatalf("expected sell-through proxy above 60, got %#v", insight.SellThroughRate)
	}
	if insight.TargetListPrice != 225 {
		t.Fatalf("expected sold average as target list price, got %.2f", insight.TargetListPrice)
	}
	if insight.BenchmarkSource != "eBay sold average" {
		t.Fatalf("expected sold benchmark, got %s", insight.BenchmarkSource)
	}
}

func TestBuildVendorInsightDoesNotRecommendGenericResearchTarget(t *testing.T) {
	opportunity := models.SlabOpportunity{
		EstimatedMarketValue: 500,
		ExpectedProfit:       0,
		ExpectedMarginPct:    0,
		Evidence:             json.RawMessage(`{"signal":"RESEARCH_TARGET","needsEbayScan":true,"trendChange":100}`),
		SellerHubMetrics:     json.RawMessage(`{"active":null,"sold":null}`),
	}

	insight := buildVendorInsight(opportunity, time.Now())

	if insight.Action != "FIND_EXACT_LISTING" {
		t.Fatalf("expected FIND_EXACT_LISTING, got %s", insight.Action)
	}
	if insight.OpportunityScore >= 50 {
		t.Fatalf("generic research target should not rank as a buy, got %d", insight.OpportunityScore)
	}
}

func TestBuildVendorInsightPenalizesOverpricedListing(t *testing.T) {
	opportunity := models.SlabOpportunity{
		AskingPrice:          300,
		AllInCost:            342,
		EstimatedMarketValue: 200,
		ExpectedProfit:       -142,
		ExpectedMarginPct:    -41.52,
		Evidence:             json.RawMessage(`{"signal":"SELL_RESEARCH","needsEbayScan":false}`),
		SellerHubMetrics: json.RawMessage(`{
			"active":{"totalListings":20,"avgListingPrice":210,"avgWatchers":1,"researchedAt":"2026-06-04T12:00:00Z"},
			"sold":{"totalListings":2,"avgListingPrice":195,"avgBids":0,"researchedAt":"2026-06-04T12:00:00Z"}
		}`),
	}

	insight := buildVendorInsight(opportunity, time.Date(2026, 6, 4, 17, 0, 0, 0, time.UTC))

	if insight.Action != "AVOID" {
		t.Fatalf("expected AVOID, got %s", insight.Action)
	}
	if insight.PriceEdgePct == nil || *insight.PriceEdgePct >= 0 {
		t.Fatalf("expected negative price edge, got %#v", insight.PriceEdgePct)
	}
}

func TestBuildVendorInsightMarksStaleEvidence(t *testing.T) {
	opportunity := models.SlabOpportunity{
		AskingPrice:          100,
		AllInCost:            114,
		EstimatedMarketValue: 200,
		ExpectedProfit:       86,
		ExpectedMarginPct:    75,
		Evidence:             json.RawMessage(`{"signal":"BUY_CANDIDATE","needsEbayScan":false}`),
		SellerHubMetrics: json.RawMessage(`{
			"active":{"totalListings":5,"avgListingPrice":190,"avgWatchers":10,"researchedAt":"2026-05-01T12:00:00Z"},
			"sold":null
		}`),
	}

	insight := buildVendorInsight(opportunity, time.Date(2026, 6, 4, 17, 0, 0, 0, time.UTC))

	if !insight.SellerHubStale {
		t.Fatal("expected stale Seller Hub evidence")
	}
	if insight.Action == "SOURCE_NOW" {
		t.Fatal("stale evidence must not produce SOURCE_NOW")
	}
}
