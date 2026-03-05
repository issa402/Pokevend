// store/card_store.go — Repository: all card SQL queries
// Services call this interface. Swap the implementation anytime (e.g. DynamoDB).
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"pokemontool/models"
)

// CardStore defines the data access contract for cards
type CardStore interface {
	Search(ctx context.Context, query string, limit int) ([]models.Card, error)
	GetTrending(ctx context.Context) (rising, falling []models.Card, err error)
	GetPriceHistory(ctx context.Context, cardID string) ([]models.PricePoint, error)
	GetByID(ctx context.Context, cardID string) (*models.Card, error)
}

type postgresCardStore struct{ db *pgxpool.Pool }

func NewCardStore(db *pgxpool.Pool) CardStore { return &postgresCardStore{db: db} }

func (s *postgresCardStore) Search(ctx context.Context, query string, limit int) ([]models.Card, error) {
	rows, err := s.db.Query(ctx,
		`SELECT card_id,name,set_name,set_code,image_url,trending_score,trend_label,
		        pct_change_7d,price_ebay,price_tcgplayer,price_facebook,price_mercari,last_updated
		 FROM cards WHERE name ILIKE $1 OR set_name ILIKE $1
		 ORDER BY trending_score DESC LIMIT $2`,
		fmt.Sprintf("%%%s%%", query), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCards(rows)
}

func (s *postgresCardStore) GetTrending(ctx context.Context) ([]models.Card, []models.Card, error) {
	rising, err := s.queryByLabel(ctx, "RISING", "trending_score DESC")
	if err != nil {
		return nil, nil, err
	}
	falling, err := s.queryByLabel(ctx, "FALLING", "trending_score ASC")
	return rising, falling, err
}

func (s *postgresCardStore) queryByLabel(ctx context.Context, label, order string) ([]models.Card, error) {
	rows, err := s.db.Query(ctx,
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
		rows.Scan(&p.Date, &p.AvgPrice, &p.PriceEbay, &p.PriceTCG)
		points = append(points, p)
	}
	return points, nil
}

func (s *postgresCardStore) GetByID(ctx context.Context, cardID string) (*models.Card, error) {
	var c models.Card
	err := s.db.QueryRow(ctx,
		`SELECT card_id,name,set_name,set_code,image_url,trending_score,trend_label,
		        pct_change_7d,price_ebay,price_tcgplayer,price_facebook,price_mercari,last_updated
		 FROM cards WHERE card_id=$1`, cardID,
	).Scan(&c.CardID, &c.Name, &c.SetName, &c.SetCode, &c.ImageURL,
		&c.TrendingScore, &c.TrendLabel, &c.PctChange7d,
		&c.PriceEbay, &c.PriceTCG, &c.PriceFacebook, &c.PriceMercari, &c.LastUpdated)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// scanCards — shared row scanner for card queries
func scanCards(rows interface {
	Next() bool
	Scan(...interface{}) error
}) ([]models.Card, error) {
	var cards []models.Card
	for rows.Next() {
		var c models.Card
		if err := rows.Scan(&c.CardID, &c.Name, &c.SetName, &c.SetCode, &c.ImageURL,
			&c.TrendingScore, &c.TrendLabel, &c.PctChange7d,
			&c.PriceEbay, &c.PriceTCG, &c.PriceFacebook, &c.PriceMercari, &c.LastUpdated); err != nil {
			continue
		}
		cards = append(cards, c)
	}
	return cards, nil
}
