// ============================================================
// FILE: server/store/card_store.go
// TYPE: Repository Layer — Card SQL Queries
//
// WHAT IS THE REPOSITORY PATTERN?
// The Repository pattern is one of the most important FAANG backend patterns.
// The idea: ALL database queries for a domain (e.g. cards) live in ONE place.
//
// BEFORE Repository pattern: SQL scattered everywhere
//
//	handlers/cards.go       → has SQL
//	services/card_service.go → also has SQL
//	worker/notification.go  → also has SQL
//
// AFTER Repository pattern: SQL in ONE place
//
//	store/card_store.go     → ALL card SQL is here and ONLY here
//	handlers, services, worker → call the INTERFACE, never write SQL
//
// WHY THIS IS FAANG STANDARD:
//  1. Want to switch from PostgreSQL to DynamoDB? Change the store, nothing else.
//  2. Want to add query caching? Add it in the store, once.
//  3. Want to test your service? Mock the interface — no real DB needed.
//
// GO CONCEPT: Interface-based Repository
//   - Define an interface (CardStore) with the methods you need
//   - Write a concrete implementation (postgresCardStore)
//   - Services accept the INTERFACE, not the concrete type
//   - In tests, you can provide a testCardStore that returns fake data
//
// ============================================================
package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"pokemontool/models"
)

// CardStore is the interface — the CONTRACT.
// Any type that has these four methods is a CardStore.
// Your service only depends on this interface, not the implementation.
//
// This is Go's "duck typing" for interfaces:
// You don't explicitly say "I implement CardStore."
// If you have all the methods, you implement it automatically.
type CardStore interface {
	Search(ctx context.Context, query string, limit int) ([]models.Card, error)
	GetTrending(ctx context.Context) (rising, falling []models.Card, err error)
	GetPriceHistory(ctx context.Context, cardID string) ([]models.PricePoint, error)
	GetByID(ctx context.Context, cardID string) (*models.Card, error)
	UpdatePrice(ctx context.Context, name string, marketplace string, price float64) error
	UpsertPriceSnapshot(ctx context.Context, cardID string, marketplace string, price float64) error
	UpsertListingSnapshot(ctx context.Context, listing models.ListingSnapshot) error
	GetSlabMarketSummary(ctx context.Context, externalCardID string, languagePreference string) ([]models.SlabMarketSummary, error)
}

// postgresCardStore is the CONCRETE implementation using PostgreSQL.
// It's unexported (lowercase 'p') — nobody outside this package
// should create it directly. They use NewCardStore() instead.
type postgresCardStore struct {
	db *pgxpool.Pool // the shared connection pool from config/db.go
}

// NewCardStore is the constructor function.
// FAANG PATTERN: Constructor functions named "New<Type>"
// Returns the INTERFACE (CardStore), not the concrete type (*postgresCardStore).
// This enforces that callers only use the interface methods.
func NewCardStore(db *pgxpool.Pool) CardStore {
	return &postgresCardStore{db: db}
}

// Search finds cards by name or set name (case-insensitive).
// ILIKE = case-insensitive LIKE in PostgreSQL.
// $1, $2 = parameterized query placeholders (prevents SQL injection).
// NEVER use fmt.Sprintf to build SQL — that's a SQL injection vulnerability.
func (s *postgresCardStore) Search(ctx context.Context, query string, limit int) ([]models.Card, error) {
	rows, err := s.db.Query(ctx,
		`SELECT card_id,name,set_name,set_code,image_url,trending_score,trend_label,
		        pct_change_7d,price_ebay,price_tcgplayer,price_facebook,price_mercari,last_updated
		 FROM cards WHERE name ILIKE $1 OR set_name ILIKE $1
		 ORDER BY trending_score DESC LIMIT $2`,
		// $1 = "%charizard%" (the % wildcards match any characters)
		fmt.Sprintf("%%%s%%", query), limit,
	)
	if err != nil {
		return nil, err
	}
	// defer rows.Close() — ALWAYS close rows after querying.
	// If you forget, the connection is held open and the pool starves.
	defer rows.Close()
	return scanCards(rows)
}

// GetTrending returns top 10 rising and top 10 falling cards.
// These are the cards shown on the dashboard "trending" section.
func (s *postgresCardStore) GetTrending(ctx context.Context) ([]models.Card, []models.Card, error) {
	rising, err := s.queryByLabel(ctx, "RISING", "trending_score DESC")
	if err != nil {
		return nil, nil, err
	}
	falling, err := s.queryByLabel(ctx, "FALLING", "trending_score ASC")
	return rising, falling, err
}

func (s *postgresCardStore) UpdatePrice(ctx context.Context, name string, marketplace string, price float64) error {
	query := fmt.Sprintf(
		`UPDATE cards
		SET price_%s = $1, last_updated = NOW()
		WHERE name = $2`, marketplace)
	_, err := s.db.Exec(ctx, query, price, name)
	return err
}

func (s *postgresCardStore) UpsertPriceSnapshot(ctx context.Context, cardID string, marketplace string, price float64) error {
	column := "price_tcgplayer"
	if marketplace == "ebay" {
		column = "price_ebay"
	}
	query := fmt.Sprintf(
		`INSERT INTO price_history (card_id, date, avg_price, %s, sale_count)
		 VALUES ($1, CURRENT_DATE, $2, $2, 1)
		 ON CONFLICT (card_id, date)
		 DO UPDATE SET avg_price = EXCLUDED.avg_price, %s = EXCLUDED.%s`,
		column, column, column,
	)
	_, err := s.db.Exec(ctx, query, cardID, price)
	return err
}

func (s *postgresCardStore) UpsertListingSnapshot(ctx context.Context, listing models.ListingSnapshot) error {
	if listing.ListingID == nil || *listing.ListingID == "" {
		return nil
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO card_listings (
			card_name, marketplace, price, listing_url, image_url, condition, listing_id,
			set_name, external_card_id, is_slab, grader, grade, slab_tier, listing_title, language_preference, discovered_at
		 )
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,NOW())
		 ON CONFLICT (marketplace, listing_id)
		 DO UPDATE SET
			price = EXCLUDED.price,
			listing_url = EXCLUDED.listing_url,
			image_url = EXCLUDED.image_url,
			condition = EXCLUDED.condition,
			set_name = EXCLUDED.set_name,
			external_card_id = EXCLUDED.external_card_id,
			is_slab = EXCLUDED.is_slab,
			grader = EXCLUDED.grader,
			grade = EXCLUDED.grade,
			slab_tier = EXCLUDED.slab_tier,
			listing_title = EXCLUDED.listing_title,
			language_preference = EXCLUDED.language_preference,
			discovered_at = NOW()`,
		listing.CardName, listing.Marketplace, listing.Price, listing.ListingURL, listing.ImageURL,
		listing.Condition, listing.ListingID, listing.SetName, listing.ExternalCardID, listing.IsSlab,
		listing.Grader, listing.Grade, listing.SlabTier, listing.ListingTitle, listing.LanguagePreference,
	)
	return err
}

func (s *postgresCardStore) GetSlabMarketSummary(ctx context.Context, externalCardID string, languagePreference string) ([]models.SlabMarketSummary, error) {
	rows, err := s.db.Query(ctx,
		`WITH ranked AS (
			SELECT
				COALESCE(NULLIF(slab_tier, ''), 'RAW') AS slab_tier,
				price::float8,
				listing_url,
				listing_id,
				listing_title,
				image_url,
				condition,
				COALESCE(language_preference, 'BOTH') AS language_preference,
				discovered_at,
				COUNT(*) OVER (PARTITION BY COALESCE(NULLIF(slab_tier, ''), 'RAW')) AS listing_count,
				ROW_NUMBER() OVER (
					PARTITION BY COALESCE(NULLIF(slab_tier, ''), 'RAW')
					ORDER BY price ASC, discovered_at DESC
				) AS rn
			FROM card_listings
			WHERE external_card_id = $1
			  AND listing_title IS NOT NULL
			  AND ($2 = 'BOTH' OR COALESCE(language_preference, 'BOTH') IN ($2, 'BOTH'))
			  AND discovered_at >= NOW() - INTERVAL '7 days' 
		)
		SELECT slab_tier, price, listing_url, listing_id, listing_title, image_url, condition, language_preference, discovered_at, listing_count
		FROM ranked
		ORDER BY CASE slab_tier
			WHEN 'RAW' THEN 0
			WHEN 'PSA_10' THEN 1 WHEN 'PSA_9' THEN 2 WHEN 'PSA_8' THEN 3 WHEN 'PSA_7' THEN 4
			WHEN 'CGC_10' THEN 5 WHEN 'CGC_9_5' THEN 6 WHEN 'CGC_9' THEN 7 WHEN 'CGC_8_5' THEN 8 WHEN 'CGC_8' THEN 9 WHEN 'CGC_7_5' THEN 10 WHEN 'CGC_7' THEN 11
			WHEN 'BGS_10' THEN 12 WHEN 'BGS_9_5' THEN 13 WHEN 'BGS_9' THEN 14 WHEN 'BGS_8_5' THEN 15 WHEN 'BGS_8' THEN 16 WHEN 'BGS_7_5' THEN 17 WHEN 'BGS_7' THEN 18
			ELSE 99
		END, price ASC, discovered_at DESC`,
		externalCardID, normalizeLanguagePreference(languagePreference),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summaries := []models.SlabMarketSummary{}
	byTier := map[string]int{}
	for rows.Next() {
		var tier string
		var listing models.SlabListing
		var count int
		if err := rows.Scan(&tier, &listing.Price, &listing.ListingURL, &listing.ListingID, &listing.ListingTitle, &listing.ImageURL, &listing.Condition, &listing.LanguagePreference, &listing.ObservedAt, &count); err != nil {
			return nil, err
		}
		index, ok := byTier[tier]
		if !ok {
			price := listing.Price
			summaries = append(summaries, models.SlabMarketSummary{
				SlabTier:    tier,
				Label:       slabTierLabel(tier),
				LowestPrice: &price,
				ListingURL:  listing.ListingURL,
				ListingID:   listing.ListingID,
				ObservedAt:  listing.ObservedAt,
				Count:       count,
				Listings:    []models.SlabListing{},
			})
			index = len(summaries) - 1
			byTier[tier] = index
		}
		summaries[index].Listings = append(summaries[index].Listings, listing)
	}
	return summaries, nil
}

func slabTierLabel(tier string) string {
	switch tier {
	case "RAW":
		return "Raw"
	case "PSA_10":
		return "PSA 10"
	case "PSA_9":
		return "PSA 9"
	case "PSA_8":
		return "PSA 8"
	case "PSA_7":
		return "PSA 7"
	case "CGC_10":
		return "CGC 10"
	case "CGC_9_5":
		return "CGC 9.5"
	case "CGC_9":
		return "CGC 9"
	case "BGS_10":
		return "BGS 10"
	case "BGS_9_5":
		return "BGS 9.5"
	case "CGC_8_5":
		return "CGC 8.5"
	case "CGC_8":
		return "CGC 8"
	case "CGC_7_5":
		return "CGC 7.5"
	case "CGC_7":
		return "CGC 7"
	case "BGS_8_5":
		return "BGS 8.5"
	case "BGS_8":
		return "BGS 8"
	case "BGS_7_5":
		return "BGS 7.5"
	case "BGS_7":
		return "BGS 7"
	case "BGS_9":
		return "BGS 9"
	default:
		parts := strings.Split(tier, "_")
		if len(parts) >= 2 && (parts[0] == "PSA" || parts[0] == "CGC" || parts[0] == "BGS") {
			return parts[0] + " " + strings.Join(parts[1:], ".")
		}
		return tier
	}
}

// queryByLabel is a private helper — reduces duplication between rising/falling queries.
// Shared logic extracted into a private function = DRY principle (Don't Repeat Yourself).

func (s *postgresCardStore) queryByLabel(ctx context.Context, label, order string) ([]models.Card, error) {
	rows, err := s.db.Query(ctx,
		// fmt.Sprintf in the ORDER BY is safe here because we control the input
		// (it's always "trending_score DESC" or "trending_score ASC", not user input)
		fmt.Sprintf(`SELECT card_id,name,set_name,set_code,image_url,trending_score,trend_label,
		              pct_change_7d,price_ebay,price_tcgplayer,price_facebook,price_mercari,last_updated
		              FROM cards WHERE trend_label=$1 ORDER BY %s LIMIT 10`, order),
		label,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCards(rows)
}

// GetPriceHistory returns 30 days of price data for a card.
// This powers the Chart.js price chart on the card detail page.
func (s *postgresCardStore) GetPriceHistory(ctx context.Context, cardID string) ([]models.PricePoint, error) {
	rows, err := s.db.Query(ctx,
		`SELECT date,avg_price,price_ebay,price_tcgplayer FROM price_history
		 WHERE card_id=$1 ORDER BY date ASC LIMIT 30`, cardID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var points []models.PricePoint
	for rows.Next() {
		var p models.PricePoint
		// rows.Scan maps database columns to Go struct fields.
		// Column order MUST match the SELECT order above.
		// Go's pgx driver handles type conversion (PostgreSQL DATE → Go string, etc.)
		rows.Scan(&p.Date, &p.AvgPrice, &p.PriceEbay, &p.PriceTCG)
		points = append(points, p)
	}
	return points, nil
}

// GetByID fetches a single card by its card_id.
// Returns *Card (pointer) so we can return nil if not found.
func (s *postgresCardStore) GetByID(ctx context.Context, cardID string) (*models.Card, error) {
	var c models.Card
	err := s.db.QueryRow(ctx,
		// QueryRow vs Query: use QueryRow when you expect exactly ONE row.
		// It returns a *pgx.Row which has a Scan method.
		`SELECT card_id,name,set_name,set_code,image_url,trending_score,trend_label,
		        pct_change_7d,price_ebay,price_tcgplayer,price_facebook,price_mercari,last_updated
		 FROM cards WHERE card_id=$1`, cardID,
	).Scan(&c.CardID, &c.Name, &c.SetName, &c.SetCode, &c.ImageURL,
		&c.TrendingScore, &c.TrendLabel, &c.PctChange7d,
		&c.PriceEbay, &c.PriceTCG, &c.PriceFacebook, &c.PriceMercari, &c.LastUpdated)
	if err != nil {
		return nil, err // pgx.ErrNoRows if card not found
	}
	return &c, nil
}

// scanCards is a private shared helper to scan multiple card rows.
// Used by both Search and queryByLabel to avoid duplicating scan logic.
// The interface{} parameter type accepts any type that has Next() and Scan() —
// this works with pgx.Rows from Query().
func scanCards(rows interface {
	Next() bool
	Scan(...interface{}) error
}) ([]models.Card, error) {
	var cards []models.Card
	for rows.Next() { // iterate over result rows
		var c models.Card
		if err := rows.Scan(&c.CardID, &c.Name, &c.SetName, &c.SetCode, &c.ImageURL,
			&c.TrendingScore, &c.TrendLabel, &c.PctChange7d,
			&c.PriceEbay, &c.PriceTCG, &c.PriceFacebook, &c.PriceMercari, &c.LastUpdated); err != nil {
			continue // skip rows that fail to scan (log in production)
		}
		cards = append(cards, c)
	}
	return cards, nil
}

func normalizeLanguagePreference(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "ENGLISH", "EN":
		return "ENGLISH"
	case "JAPANESE", "JP", "JPN":
		return "JAPANESE"
	default:
		return "BOTH"
	}
}

// TODO #1 (Practice): Implement GetBySetName(ctx, setName string) method
// Add it to the CardStore interface AND implement it in postgresCardStore.
// SQL: SELECT ... FROM cards WHERE set_name ILIKE $1 ORDER BY name ASC
// This would power a "browse by set" feature on the frontend.
// Update routes.go to add a new endpoint: GET /api/cards/set/{setName}

// TODO #2 (Practice): Implement UpdatePrice(ctx, cardID string, prices models.Card) error
// Used by the Python analytics engine worker (future Go replacement)
// SQL: UPDATE cards SET price_ebay=$1, price_tcgplayer=$2, last_updated=NOW() WHERE card_id=$3
// Think about: should this be a partial update (only update provided fields)?
// Research: SQL COALESCE function — useful for "update if provided, else keep existing"
