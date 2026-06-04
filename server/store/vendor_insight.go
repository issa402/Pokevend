package store

import (
	"encoding/json"
	"math"
	"time"

	"pokemontool/models"
)

func buildVendorInsight(opportunity models.SlabOpportunity, now time.Time) models.VendorInsight {
	var metrics sellerHubMetricsPayload
	_ = json.Unmarshal(opportunity.SellerHubMetrics, &metrics)

	var evidence struct {
		NeedsEbayScan bool     `json:"needsEbayScan"`
		TrendChange   *float64 `json:"trendChange"`
	}
	_ = json.Unmarshal(opportunity.Evidence, &evidence)

	insight := models.VendorInsight{
		HasExactListing:   !evidence.NeedsEbayScan && opportunity.AskingPrice > 0,
		HasActiveResearch: hasMeaningfulActiveResearch(metrics.Active),
		HasSoldResearch:   hasMeaningfulSoldResearch(metrics.Sold),
		TargetListPrice:   opportunity.EstimatedMarketValue,
		BenchmarkSource:   "reference market value",
	}
	if metrics.Sold != nil && metrics.Sold.AvgListingPrice != nil && *metrics.Sold.AvgListingPrice > 0 {
		insight.TargetListPrice = *metrics.Sold.AvgListingPrice
		insight.BenchmarkSource = "eBay sold average"
	}

	insight.SellThroughRate = sellThroughProxy(metrics.Active, metrics.Sold)
	insight.DemandScore = sellerHubDemandScore(metrics.Active, metrics.Sold, insight.SellThroughRate)
	insight.PriceEdgePct = priceEdgePct(opportunity.AskingPrice, insight.TargetListPrice)
	insight.PriceScore = vendorPriceScore(insight.PriceEdgePct)
	insight.SellerHubStale = sellerHubEvidenceStale(metrics.Active, metrics.Sold, now)
	insight.EvidenceScore = vendorEvidenceScore(insight)

	economicsScore := minInt(20, int(math.Max(opportunity.ExpectedMarginPct, 0)/2)) +
		minInt(20, int(math.Max(opportunity.ExpectedProfit, 0)/25))
	trendScore := 0
	if evidence.TrendChange != nil && *evidence.TrendChange > 0 {
		trendScore = minInt(5, int(*evidence.TrendChange/20))
	}
	insight.OpportunityScore = clampInt(
		economicsScore+insight.DemandScore+insight.PriceScore+insight.EvidenceScore+trendScore,
		0,
		100,
	)
	if !insight.HasSoldResearch {
		insight.OpportunityScore = minInt(insight.OpportunityScore, 69)
	}

	switch {
	case !insight.HasExactListing:
		insight.Action = "FIND_EXACT_LISTING"
		insight.ActionReason = "Research is promising, but no exact purchasable listing is attached yet."
	case opportunity.ExpectedProfit <= 0 || opportunity.ExpectedMarginPct < 0:
		insight.Action = "AVOID"
		insight.ActionReason = "The exact listing is above the evidence-backed resale value after estimated costs."
	case insight.SellerHubStale:
		insight.Action = "REFRESH_RESEARCH"
		insight.ActionReason = "The listing has positive economics, but Seller Hub demand evidence is stale."
	case insight.OpportunityScore >= 70 && opportunity.ExpectedMarginPct >= 20 && insight.EvidenceScore >= 10 && insight.HasSoldResearch:
		insight.Action = "SOURCE_NOW"
		insight.ActionReason = "Exact listing, strong margin, and fresh marketplace demand support a sourcing decision."
	case insight.OpportunityScore >= 55:
		insight.Action = "CONSIDER_BUY"
		insight.ActionReason = "The listing has potential, but demand or pricing evidence is not strong enough for an urgent buy."
	default:
		insight.Action = "WATCH"
		insight.ActionReason = "Keep monitoring until price, demand, or evidence quality improves."
	}
	return insight
}

func hasMeaningfulActiveResearch(metric *sellerHubMetric) bool {
	return metric != nil &&
		((metric.TotalListings != nil && *metric.TotalListings > 0) ||
			(metric.AvgWatchers != nil && *metric.AvgWatchers > 0) ||
			(metric.AvgListingPrice != nil && *metric.AvgListingPrice > 0))
}

func hasMeaningfulSoldResearch(metric *sellerHubMetric) bool {
	return metric != nil &&
		((metric.TotalListings != nil && *metric.TotalListings > 0) ||
			(metric.AvgBids != nil && *metric.AvgBids > 0) ||
			(metric.AvgListingPrice != nil && *metric.AvgListingPrice > 0))
}

func sellThroughProxy(active, sold *sellerHubMetric) *float64 {
	if active == nil || sold == nil || active.TotalListings == nil || sold.TotalListings == nil {
		return nil
	}
	total := *active.TotalListings + *sold.TotalListings
	if total <= 0 {
		return nil
	}
	value := math.Round((float64(*sold.TotalListings)/float64(total))*10000) / 100
	return &value
}

func sellerHubDemandScore(active, sold *sellerHubMetric, sellThrough *float64) int {
	score := 0
	if sellThrough != nil {
		score += minInt(12, int(*sellThrough/8))
	}
	if active != nil && active.AvgWatchers != nil {
		score += minInt(8, int(*active.AvgWatchers/2))
	}
	if sold != nil && sold.AvgBids != nil {
		score += minInt(5, int(*sold.AvgBids*2))
	}
	return clampInt(score, 0, 25)
}

func priceEdgePct(askingPrice, benchmark float64) *float64 {
	if askingPrice <= 0 || benchmark <= 0 {
		return nil
	}
	value := math.Round(((benchmark-askingPrice)/benchmark)*10000) / 100
	return &value
}

func vendorPriceScore(edge *float64) int {
	if edge == nil {
		return 0
	}
	if *edge >= 0 {
		return minInt(20, 8+int(*edge/2))
	}
	return maxInt(0, 8+int(*edge/2))
}

func vendorEvidenceScore(insight models.VendorInsight) int {
	score := 0
	if insight.HasExactListing {
		score += 5
	}
	if insight.HasActiveResearch {
		score += 4
	}
	if insight.HasSoldResearch {
		score += 6
	}
	if insight.SellerHubStale {
		score -= 5
	}
	return clampInt(score, 0, 15)
}

func sellerHubEvidenceStale(active, sold *sellerHubMetric, now time.Time) bool {
	var latest time.Time
	for _, metric := range []*sellerHubMetric{active, sold} {
		if metric != nil && metric.ResearchedAt != nil && metric.ResearchedAt.After(latest) {
			latest = *metric.ResearchedAt
		}
	}
	return !latest.IsZero() && now.Sub(latest) > 7*24*time.Hour
}
