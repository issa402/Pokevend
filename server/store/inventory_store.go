// store/inventory_store.go — Repository: inventory SQL queries
package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"pokemontool/models"
)

type InventoryStore interface {
	ListByUser(ctx context.Context, userID string) ([]models.InventoryItem, error)
	Insert(ctx context.Context, item models.InventoryItem) (string, error)
	BulkInsert(ctx context.Context, items []models.InventoryItem) (int, error)
	Delete(ctx context.Context, itemID, userID string) error
}

type postgresInventoryStore struct{ db *pgxpool.Pool }

func NewInventoryStore(db *pgxpool.Pool) InventoryStore { return &postgresInventoryStore{db: db} }

func (s *postgresInventoryStore) ListByUser(ctx context.Context, userID string) ([]models.InventoryItem, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id,user_id,card_name,set_name,card_number,condition,quantity,
		        purchase_price,current_value,notes,acquired_at
		 FROM inventory WHERE user_id=$1 ORDER BY acquired_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.InventoryItem
	for rows.Next() {
		var i models.InventoryItem
		rows.Scan(&i.ID, &i.UserID, &i.CardName, &i.SetName, &i.CardNumber,
			&i.Condition, &i.Quantity, &i.PurchasePrice, &i.CurrentValue, &i.Notes, &i.AcquiredAt)
		items = append(items, i)
	}
	return items, nil
}

func (s *postgresInventoryStore) Insert(ctx context.Context, item models.InventoryItem) (string, error) {
	var id string
	err := s.db.QueryRow(ctx,
		`INSERT INTO inventory (user_id,card_name,set_name,card_number,condition,quantity,purchase_price,notes)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		item.UserID, item.CardName, item.SetName, item.CardNumber,
		item.Condition, item.Quantity, item.PurchasePrice, item.Notes,
	).Scan(&id)
	return id, err
}

func (s *postgresInventoryStore) BulkInsert(ctx context.Context, items []models.InventoryItem) (int, error) {
	count := 0
	for _, item := range items {
		_, err := s.db.Exec(ctx,
			`INSERT INTO inventory (user_id,card_name,set_name,card_number,condition,quantity)
			 VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`,
			item.UserID, item.CardName, item.SetName, item.CardNumber, item.Condition, item.Quantity,
		)
		if err == nil {
			count++
		}
	}
	return count, nil
}

func (s *postgresInventoryStore) Delete(ctx context.Context, itemID, userID string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM inventory WHERE id=$1 AND user_id=$2`, itemID, userID)
	return err
}
