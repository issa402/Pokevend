// store/deal_store.go — Repository: deal SQL queries
package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"pokemontool/models"
)

type DealStore interface {
	GetByDate(ctx context.Context, date string) ([]models.Deal, error)
}

type postgresDealStore struct{ db *pgxpool.Pool }

func NewDealStore(db *pgxpool.Pool) DealStore { return &postgresDealStore{db: db} }

func (s *postgresDealStore) GetByDate(ctx context.Context, date string) ([]models.Deal, error) {
	rows, err := s.db.Query(ctx,
		`SELECT card_name,set_name,image_url,market_price,best_price,
		        savings,savings_pct,listing_url,marketplace,reason
		 FROM deals WHERE deal_date=$1 ORDER BY savings_pct DESC LIMIT 10`, date,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var deals []models.Deal
	for rows.Next() {
		var d models.Deal
		rows.Scan(&d.CardName, &d.SetName, &d.ImageURL, &d.MarketPrice,
			&d.BestPrice, &d.Savings, &d.SavingsPct, &d.ListingURL, &d.Marketplace, &d.Reason)
		deals = append(deals, d)
	}
	return deals, nil
}
