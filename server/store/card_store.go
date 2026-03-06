// ============================================================
// FILE: server/store/card_store.go
// TYPE: Repository Layer — Card SQL Queries
//
// WHAT IS THE REPOSITORY PATTERN?
// The Repository pattern is one of the most important FAANG backend patterns.
// The idea: ALL database queries for a domain (e.g. cards) live in ONE place.
//
// BEFORE Repository pattern: SQL scattered everywhere
//   handlers/cards.go       → has SQL
//   services/card_service.go → also has SQL
//   worker/notification.go  → also has SQL
//
// AFTER Repository pattern: SQL in ONE place
//   store/card_store.go     → ALL card SQL is here and ONLY here
//   handlers, services, worker → call the INTERFACE, never write SQL
//
// WHY THIS IS FAANG STANDARD:
//   1. Want to switch from PostgreSQL to DynamoDB? Change the store, nothing else.
//   2. Want to add query caching? Add it in the store, once.
//   3. Want to test your service? Mock the interface — no real DB needed.
//
// GO CONCEPT: Interface-based Repository
//   - Define an interface (CardStore) with the methods you need
//   - Write a concrete implementation (postgresCardStore)
//   - Services accept the INTERFACE, not the concrete type
//   - In tests, you can provide a testCardStore that returns fake data
// ============================================================
package store

import (
	"context"
	"fmt"

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
