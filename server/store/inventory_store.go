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
	MarkReadyForStore(ctx context.Context, itemID, userID string, input models.StoreListingInput) (*models.InventoryItem, error)
	MarkStoreSynced(ctx context.Context, itemID, userID string) error
}

type postgresInventoryStore struct{ db *pgxpool.Pool }

func NewInventoryStore(db *pgxpool.Pool) InventoryStore { return &postgresInventoryStore{db: db} }

func (s *postgresInventoryStore) ListByUser(ctx context.Context, userID string) ([]models.InventoryItem, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id,user_id,card_name,set_name,card_number,external_card_id,rarity,image_url,
                market_updated_at,price_source,condition,quantity,purchase_price,current_value,notes,
                asset_type,grader,grade,slab_tier,cert_number,target_sale_price::float,
                store_listing_status,store_price::float,store_listing_notes,store_listed_at,store_synced_at,
                acquired_at
		 FROM inventory WHERE user_id=$1 ORDER BY acquired_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.InventoryItem
	for rows.Next() {
		var i models.InventoryItem
		rows.Scan(
			&i.ID, &i.UserID, &i.CardName, &i.SetName, &i.CardNumber, &i.ExternalCardID, &i.Rarity, &i.ImageURL,
			&i.MarketUpdatedAt, &i.PriceSource, &i.Condition, &i.Quantity, &i.PurchasePrice, &i.CurrentValue,
			&i.Notes, &i.AssetType, &i.Grader, &i.Grade, &i.SlabTier, &i.CertNumber, &i.TargetSalePrice,
			&i.StoreListingStatus, &i.StorePrice, &i.StoreListingNotes, &i.StoreListedAt, &i.StoreSyncedAt, &i.AcquiredAt,
		)
		items = append(items, i)
	}
	return items, nil
}

func (s *postgresInventoryStore) Insert(ctx context.Context, item models.InventoryItem) (string, error) {
	var id string
	err := s.db.QueryRow(ctx,
		`INSERT INTO inventory (
			user_id,card_name,set_name,card_number,external_card_id,rarity,image_url,
            market_updated_at,price_source,condition,quantity,purchase_price,current_value,notes,
            asset_type,grader,grade,slab_tier,cert_number,target_sale_price
         )
         VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20) RETURNING id`,
		item.UserID, item.CardName, item.SetName, item.CardNumber, item.ExternalCardID, item.Rarity, item.ImageURL,
		item.MarketUpdatedAt, item.PriceSource, item.Condition, item.Quantity, item.PurchasePrice, item.CurrentValue, item.Notes,
		item.AssetType, item.Grader, item.Grade, item.SlabTier, item.CertNumber, item.TargetSalePrice,
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

func (s *postgresInventoryStore) MarkReadyForStore(ctx context.Context, itemID, userID string, input models.StoreListingInput) (*models.InventoryItem, error) {
	row := s.db.QueryRow(ctx,
		`UPDATE inventory
		 SET store_listing_status = 'READY',
		     store_price = COALESCE($3, target_sale_price, current_value),
		     store_listing_notes = $4,
		     store_listed_at = COALESCE(store_listed_at, NOW()),
		     updated_at = NOW()
		 WHERE id=$1 AND user_id=$2
		 RETURNING id,user_id,card_name,set_name,card_number,external_card_id,rarity,image_url,
		           market_updated_at,price_source,condition,quantity,purchase_price,current_value,notes,
		           asset_type,grader,grade,slab_tier,cert_number,target_sale_price::float,
		           store_listing_status,store_price::float,store_listing_notes,store_listed_at,store_synced_at,
		           acquired_at`,
		itemID, userID, input.StorePrice, input.Notes,
	)
	var i models.InventoryItem
	err := row.Scan(
		&i.ID, &i.UserID, &i.CardName, &i.SetName, &i.CardNumber, &i.ExternalCardID, &i.Rarity, &i.ImageURL,
		&i.MarketUpdatedAt, &i.PriceSource, &i.Condition, &i.Quantity, &i.PurchasePrice, &i.CurrentValue,
		&i.Notes, &i.AssetType, &i.Grader, &i.Grade, &i.SlabTier, &i.CertNumber, &i.TargetSalePrice,
		&i.StoreListingStatus, &i.StorePrice, &i.StoreListingNotes, &i.StoreListedAt, &i.StoreSyncedAt, &i.AcquiredAt,
	)
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func (s *postgresInventoryStore) MarkStoreSynced(ctx context.Context, itemID, userID string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE inventory
		 SET store_synced_at = NOW(),
		     updated_at = NOW()
		 WHERE id=$1 AND user_id=$2`,
		itemID, userID,
	)
	return err
}
