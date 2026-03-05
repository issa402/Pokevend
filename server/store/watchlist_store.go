// store/watchlist_store.go — Repository: watchlist SQL queries
package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"pokemontool/models"
)

type WatchlistStore interface {
	ListByUser(ctx context.Context, userID string) ([]models.WatchlistItem, error)
	Insert(ctx context.Context, item models.WatchlistItem) (*models.WatchlistItem, error)
	Delete(ctx context.Context, itemID, userID string) error
	GetWatchedCards(ctx context.Context) ([]models.WatchlistItem, error) // used by notification worker
}

type postgresWatchlistStore struct{ db *pgxpool.Pool }

func NewWatchlistStore(db *pgxpool.Pool) WatchlistStore { return &postgresWatchlistStore{db: db} }

func (s *postgresWatchlistStore) ListByUser(ctx context.Context, userID string) ([]models.WatchlistItem, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id,user_id,card_name,set_name,target_buy_price,target_sell_price,notes,added_at
		 FROM watchlists WHERE user_id=$1 ORDER BY added_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.WatchlistItem
	for rows.Next() {
		var w models.WatchlistItem
		rows.Scan(&w.ID, &w.UserID, &w.CardName, &w.SetName, &w.TargetBuyPrice, &w.TargetSellPrice, &w.Notes, &w.AddedAt)
		items = append(items, w)
	}
	return items, nil
}

func (s *postgresWatchlistStore) Insert(ctx context.Context, item models.WatchlistItem) (*models.WatchlistItem, error) {
	err := s.db.QueryRow(ctx,
		`INSERT INTO watchlists (user_id,card_name,set_name,target_buy_price,target_sell_price,notes)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id,added_at`,
		item.UserID, item.CardName, item.SetName, item.TargetBuyPrice, item.TargetSellPrice, item.Notes,
	).Scan(&item.ID, &item.AddedAt)
	return &item, err
}

func (s *postgresWatchlistStore) Delete(ctx context.Context, itemID, userID string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM watchlists WHERE id=$1 AND user_id=$2`, itemID, userID)
	return err
}

func (s *postgresWatchlistStore) GetWatchedCards(ctx context.Context) ([]models.WatchlistItem, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id,user_id,card_name,set_name,target_buy_price,target_sell_price,notes,added_at
		 FROM watchlists WHERE target_buy_price IS NOT NULL OR target_sell_price IS NOT NULL`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.WatchlistItem
	for rows.Next() {
		var w models.WatchlistItem
		rows.Scan(&w.ID, &w.UserID, &w.CardName, &w.SetName, &w.TargetBuyPrice, &w.TargetSellPrice, &w.Notes, &w.AddedAt)
		items = append(items, w)
	}
	return items, nil
}
