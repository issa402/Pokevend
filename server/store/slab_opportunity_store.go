package store

import (
	"context"
	"encoding/json"
	"fmt"

	"pokemontool/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SlabOpportunityStore interface {
	List(ctx context.Context, filters models.SlabOpportunityFilters) ([]models.SlabOpportunity, error)
	Approve(ctx context.Context, opportunityID, userID string) (string, error)
	Reject(ctx context.Context, opportunityID string) error
}

type postgresSlabOpportunityStore struct{ db *pgxpool.Pool }

func NewSlabOpportunityStore(db *pgxpool.Pool) SlabOpportunityStore {
	return &postgresSlabOpportunityStore{db: db}
}

func (s *postgresSlabOpportunityStore) List(ctx context.Context, filters models.SlabOpportunityFilters) ([]models.SlabOpportunity, error) {
	if filters.Limit <= 0 || filters.Limit > 100 {
		filters.Limit = 50
	}
	rows, err := s.db.Query(ctx, `
		SELECT o.id::text, o.external_card_id, o.card_name, o.set_name, o.grader, o.grade, o.slab_tier,
		       o.marketplace, o.listing_id, o.listing_url, o.title, o.asking_price::float,
		       o.shipping_price::float, o.estimated_fees::float, o.all_in_cost::float,
		       o.estimated_market_value::float, o.expected_profit::float,
		       o.expected_margin_pct::float, o.liquidity_score, o.confidence_score, o.risk_score,
		       o.deal_score, o.decision, o.reason, o.evidence,
		       jsonb_build_object(
		         'active', seller_hub_snapshot(active_metric),
		         'sold', seller_hub_snapshot(sold_metric)
		       ) AS seller_hub_metrics,
		       o.approved_inventory_id::text, o.created_at, o.updated_at
		FROM slab_opportunities o
		LEFT JOIN LATERAL latest_seller_hub_metric(o.external_card_id, o.card_name, o.slab_tier, 'ACTIVE') active_metric ON TRUE
		LEFT JOIN LATERAL latest_seller_hub_metric(o.external_card_id, o.card_name, o.slab_tier, 'SOLD') sold_metric ON TRUE
		WHERE ($1 = '' OR o.decision = $1)
		  AND ($2 = '' OR o.grader = $2)
		  AND ($3 = '' OR o.grade = $3)
		  AND ($4 = '' OR o.slab_tier = $4)
		  AND ($5 = '' OR o.marketplace = $5)
		  AND ($6 = '' OR o.card_name ILIKE '%' || $6 || '%')
		  AND o.expected_margin_pct >= $7
		  AND ($8 = '' OR o.evidence->>'signal' = $8)
		ORDER BY o.decision = 'candidate' DESC, o.deal_score DESC, o.expected_margin_pct DESC, o.created_at DESC
		LIMIT $9`,
		filters.Decision, filters.Grader, filters.Grade, filters.SlabTier,
		filters.Marketplace, filters.CardName, filters.MinMarginPct, filters.Signal, filters.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	opportunities := []models.SlabOpportunity{}
	for rows.Next() {
		opportunity, scanErr := scanSlabOpportunity(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		opportunities = append(opportunities, opportunity)
	}
	return opportunities, rows.Err()
}

func (s *postgresSlabOpportunityStore) Approve(ctx context.Context, opportunityID, userID string) (string, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var opportunity models.SlabOpportunity
	row := tx.QueryRow(ctx, `
		SELECT o.id::text, o.external_card_id, o.card_name, o.set_name, o.grader, o.grade, o.slab_tier,
		       o.marketplace, o.listing_id, o.listing_url, o.title, o.asking_price::float,
		       o.shipping_price::float, o.estimated_fees::float, o.all_in_cost::float,
		       o.estimated_market_value::float, o.expected_profit::float,
		       o.expected_margin_pct::float, o.liquidity_score, o.confidence_score, o.risk_score,
		       o.deal_score, o.decision, o.reason, o.evidence,
		       jsonb_build_object(
		         'active', seller_hub_snapshot(active_metric),
		         'sold', seller_hub_snapshot(sold_metric)
		       ) AS seller_hub_metrics,
		       o.approved_inventory_id::text, o.created_at, o.updated_at
		FROM slab_opportunities o
		LEFT JOIN LATERAL latest_seller_hub_metric(o.external_card_id, o.card_name, o.slab_tier, 'ACTIVE') active_metric ON TRUE
		LEFT JOIN LATERAL latest_seller_hub_metric(o.external_card_id, o.card_name, o.slab_tier, 'SOLD') sold_metric ON TRUE
		WHERE o.id = $1
		FOR UPDATE`, opportunityID)
	var scanErr error
	opportunity, scanErr = scanSlabOpportunity(row)
	if scanErr != nil {
		return "", scanErr
	}
	if opportunity.Decision == "approved" && opportunity.ApprovedInventoryID != nil {
		return *opportunity.ApprovedInventoryID, tx.Commit(ctx)
	}

	notes := fmt.Sprintf("Approved slab opportunity. %s", stringPtrValue(opportunity.Reason))
	if opportunity.ListingURL != nil && *opportunity.ListingURL != "" {
		notes += " Listing: " + *opportunity.ListingURL
	}
	var inventoryID string
	err = tx.QueryRow(ctx, `
		INSERT INTO inventory
		    (user_id, card_name, set_name, external_card_id, condition, quantity,
		     purchase_price, current_value, notes, asset_type, grader, grade,
		     slab_tier, cert_number, target_sale_price)
		VALUES ($1, $2, $3, $4, 'GRADED', 1, $5, $6, $7, 'SLAB', $8, $9, $10, NULL, $6)
		RETURNING id::text`,
		userID, opportunity.CardName, opportunity.SetName, opportunity.ExternalCardID,
		opportunity.AllInCost, opportunity.EstimatedMarketValue, notes,
		opportunity.Grader, opportunity.Grade, opportunity.SlabTier,
	).Scan(&inventoryID)
	if err != nil {
		return "", err
	}

	_, err = tx.Exec(ctx, `
		UPDATE slab_opportunities
		SET decision = 'approved', approved_inventory_id = $2, updated_at = NOW()
		WHERE id = $1`, opportunityID, inventoryID)
	if err != nil {
		return "", err
	}
	return inventoryID, tx.Commit(ctx)
}

func (s *postgresSlabOpportunityStore) Reject(ctx context.Context, opportunityID string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE slab_opportunities
		SET decision = 'rejected', updated_at = NOW()
		WHERE id = $1`, opportunityID)
	return err
}

type slabOpportunityRow interface {
	Scan(dest ...any) error
}

func scanSlabOpportunity(row slabOpportunityRow) (models.SlabOpportunity, error) {
	var opportunity models.SlabOpportunity
	var evidence []byte
	var sellerHubMetrics []byte
	err := row.Scan(
		&opportunity.ID, &opportunity.ExternalCardID, &opportunity.CardName, &opportunity.SetName,
		&opportunity.Grader, &opportunity.Grade, &opportunity.SlabTier, &opportunity.Marketplace,
		&opportunity.ListingID, &opportunity.ListingURL, &opportunity.Title, &opportunity.AskingPrice,
		&opportunity.ShippingPrice, &opportunity.EstimatedFees, &opportunity.AllInCost,
		&opportunity.EstimatedMarketValue, &opportunity.ExpectedProfit, &opportunity.ExpectedMarginPct,
		&opportunity.LiquidityScore, &opportunity.ConfidenceScore, &opportunity.RiskScore,
		&opportunity.DealScore, &opportunity.Decision, &opportunity.Reason, &evidence,
		&sellerHubMetrics, &opportunity.ApprovedInventoryID, &opportunity.CreatedAt, &opportunity.UpdatedAt,
	)
	if err != nil {
		return opportunity, err
	}
	if len(evidence) == 0 {
		evidence = []byte(`{}`)
	}
	if len(sellerHubMetrics) == 0 {
		sellerHubMetrics = []byte(`{"active":null,"sold":null}`)
	}
	opportunity.Evidence = evidence
	opportunity.SellerHubMetrics = sellerHubMetrics
	applySellerHubScoreAdjustments(&opportunity)
	return opportunity, nil
}

type sellerHubMetricsPayload struct {
	Active *sellerHubMetric `json:"active"`
	Sold   *sellerHubMetric `json:"sold"`
}

type sellerHubMetric struct {
	TotalListings   *int     `json:"totalListings"`
	AvgWatchers     *float64 `json:"avgWatchers"`
	MaxWatchers     *int     `json:"maxWatchers"`
	AvgBids         *float64 `json:"avgBids"`
	MaxBids         *int     `json:"maxBids"`
	AvgListingPrice *float64 `json:"avgListingPrice"`
}

func applySellerHubScoreAdjustments(opportunity *models.SlabOpportunity) {
	var payload sellerHubMetricsPayload
	if len(opportunity.SellerHubMetrics) == 0 || json.Unmarshal(opportunity.SellerHubMetrics, &payload) != nil {
		return
	}
	liquidity := sellerHubLiquidityScore(payload.Active, payload.Sold)
	if liquidity > opportunity.LiquidityScore {
		opportunity.LiquidityScore = liquidity
	}
	confidenceBoost := sellerHubConfidenceBoost(payload.Active, payload.Sold)
	if confidenceBoost > 0 {
		opportunity.ConfidenceScore = clampInt(opportunity.ConfidenceScore+confidenceBoost, 0, 100)
		opportunity.RiskScore = clampInt(opportunity.RiskScore-confidenceBoost, 0, 100)
		opportunity.DealScore = clampInt(opportunity.DealScore+(liquidity/5)+confidenceBoost, 0, 300)
	}
}

func sellerHubLiquidityScore(active, sold *sellerHubMetric) int {
	score := 0
	if active != nil {
		if active.TotalListings != nil {
			score += minInt(*active.TotalListings*3, 30)
		}
		if active.AvgWatchers != nil {
			score += minInt(int(*active.AvgWatchers*2), 35)
		}
		if active.MaxWatchers != nil {
			score += minInt(*active.MaxWatchers/10, 25)
		}
	}
	if sold != nil {
		if sold.TotalListings != nil {
			score += minInt(*sold.TotalListings*5, 40)
		}
		if sold.AvgBids != nil {
			score += minInt(int(*sold.AvgBids*4), 20)
		}
	}
	return clampInt(score, 0, 100)
}

func sellerHubConfidenceBoost(active, sold *sellerHubMetric) int {
	boost := 0
	if active != nil && active.TotalListings != nil && *active.TotalListings > 0 {
		boost += 8
		if active.AvgWatchers != nil && *active.AvgWatchers >= 5 {
			boost += 4
		}
	}
	if sold != nil && sold.TotalListings != nil && *sold.TotalListings > 0 {
		boost += 10
		if sold.AvgBids != nil && *sold.AvgBids >= 2 {
			boost += 3
		}
	}
	return minInt(boost, 20)
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

var _ slabOpportunityRow = pgx.Row(nil)
