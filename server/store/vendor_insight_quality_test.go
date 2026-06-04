package store

import (
	"encoding/json"
	"testing"
	"time"

	"pokemontool/models"
)

func TestBuildVendorInsightRequiresMeaningfulSoldEvidenceForSourceNow(t *testing.T) {
	opportunity := models.SlabOpportunity{
		AskingPrice:          100,
		AllInCost:            114,
		EstimatedMarketValue: 1000,
		ExpectedProfit:       886,
		ExpectedMarginPct:    777,
		Evidence:             json.RawMessage(`{"signal":"BUY_CANDIDATE","needsEbayScan":false}`),
		SellerHubMetrics: json.RawMessage(`{
			"active":{"totalListings":4,"avgListingPrice":900,"avgWatchers":20,"researchedAt":"2026-06-04T12:00:00Z"},
			"sold":{"totalListings":null,"avgListingPrice":null,"researchedAt":"2026-06-04T12:00:00Z"}
		}`),
	}

	insight := buildVendorInsight(opportunity, time.Date(2026, 6, 4, 17, 0, 0, 0, time.UTC))

	if insight.HasSoldResearch {
		t.Fatal("empty sold snapshot must not count as sold research")
	}
	if insight.Action == "SOURCE_NOW" {
		t.Fatal("reference value alone must not produce SOURCE_NOW")
	}
}
