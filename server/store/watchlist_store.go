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
	GetAlertCandidates(ctx context.Context, cardName string, externalCardID *string, assetType string, slabTier *string, maxPrice float64) ([]models.WatchlistItem, error)
	GetDistinctCardNames(ctx context.Context) ([]string, error)
	GetScanTargets(ctx context.Context) ([]models.WatchlistScanTarget, error)
}

type postgresWatchlistStore struct{ db *pgxpool.Pool }

func NewWatchlistStore(db *pgxpool.Pool) WatchlistStore { return &postgresWatchlistStore{db: db} }

func (s *postgresWatchlistStore) ListByUser(ctx context.Context, userID string) ([]models.WatchlistItem, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id,user_id,card_name,set_name,external_card_id,card_number,rarity,image_url,
		        market_price,market_updated_at,target_discount_pct,price_source,
		        target_buy_price,target_sell_price,notes,
		        COALESCE(asset_type, 'RAW'), grader, grade, slab_tier, COALESCE(language_preference, 'BOTH'), added_at
		 FROM watchlists WHERE user_id=$1 ORDER BY added_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.WatchlistItem
	for rows.Next() {
		var w models.WatchlistItem
		rows.Scan(
			&w.ID, &w.UserID, &w.CardName, &w.SetName, &w.ExternalCardID, &w.CardNumber, &w.Rarity, &w.ImageURL,
			&w.MarketPrice, &w.MarketUpdatedAt, &w.TargetDiscountPct, &w.PriceSource,
			&w.TargetBuyPrice, &w.TargetSellPrice, &w.Notes,
			&w.AssetType, &w.Grader, &w.Grade, &w.SlabTier, &w.LanguagePreference, &w.AddedAt,
		)
		items = append(items, w)
	}
	return items, nil
}

func (s *postgresWatchlistStore) Insert(ctx context.Context, item models.WatchlistItem) (*models.WatchlistItem, error) {
	err := s.db.QueryRow(ctx,
		`INSERT INTO watchlists (
			user_id,card_name,set_name,external_card_id,card_number,rarity,image_url,
			market_price,market_updated_at,target_discount_pct,price_source,
			target_buy_price,target_sell_price,notes,asset_type,grader,grade,slab_tier,language_preference
		 )
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
		 RETURNING id,added_at`,
		item.UserID, item.CardName, item.SetName, item.ExternalCardID, item.CardNumber, item.Rarity, item.ImageURL,
		item.MarketPrice, item.MarketUpdatedAt, item.TargetDiscountPct, item.PriceSource,
		item.TargetBuyPrice, item.TargetSellPrice, item.Notes, item.AssetType, item.Grader, item.Grade, item.SlabTier, item.LanguagePreference,
	).Scan(&item.ID, &item.AddedAt)
	return &item, err
}

func (s *postgresWatchlistStore) Delete(ctx context.Context, itemID, userID string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM watchlists WHERE id=$1 AND user_id=$2`, itemID, userID)
	return err
}

func (s *postgresWatchlistStore) GetWatchedCards(ctx context.Context) ([]models.WatchlistItem, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id,user_id,card_name,set_name,external_card_id,card_number,rarity,image_url,
		        market_price,market_updated_at,target_discount_pct,price_source,
		        target_buy_price,target_sell_price,notes,
		        COALESCE(asset_type, 'RAW'), grader, grade, slab_tier, COALESCE(language_preference, 'BOTH'), added_at
		 FROM watchlists WHERE target_buy_price IS NOT NULL OR target_sell_price IS NOT NULL`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.WatchlistItem
	for rows.Next() {
		var w models.WatchlistItem
		rows.Scan(
			&w.ID, &w.UserID, &w.CardName, &w.SetName, &w.ExternalCardID, &w.CardNumber, &w.Rarity, &w.ImageURL,
			&w.MarketPrice, &w.MarketUpdatedAt, &w.TargetDiscountPct, &w.PriceSource,
			&w.TargetBuyPrice, &w.TargetSellPrice, &w.Notes,
			&w.AssetType, &w.Grader, &w.Grade, &w.SlabTier, &w.LanguagePreference, &w.AddedAt,
		)
		items = append(items, w)
	}
	return items, nil
}

func (s *postgresWatchlistStore) GetAlertCandidates(ctx context.Context, cardName string, externalCardID *string, assetType string, slabTier *string, maxPrice float64) ([]models.WatchlistItem, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, user_id, card_name, external_card_id, target_buy_price,
		        COALESCE(asset_type, 'RAW'), slab_tier
		 FROM watchlists
		 WHERE target_buy_price >= $5
		   AND (COALESCE(asset_type, 'RAW') = $3 OR (COALESCE(asset_type, 'RAW') = 'ALL_SLABS' AND $3 = 'SLAB'))
		   AND (
		     $3 = 'RAW'
		     OR COALESCE(asset_type, 'RAW') = 'ALL_SLABS'
		     OR (NULLIF($4::text, '') IS NOT NULL AND slab_tier = $4)
		   )
		   AND (
		     (NULLIF($2::text, '') IS NOT NULL AND external_card_id = $2)
		     OR ((external_card_id IS NULL OR external_card_id = '') AND LOWER(card_name) = LOWER($1))
		   )`,
		cardName, externalCardID, assetType, slabTier, maxPrice)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []models.WatchlistItem
	for rows.Next() {
		var w models.WatchlistItem
		if err := rows.Scan(&w.ID, &w.UserID, &w.CardName, &w.ExternalCardID, &w.TargetBuyPrice, &w.AssetType, &w.SlabTier); err != nil {
			return nil, err
		}
		matches = append(matches, w)
	}
	return matches, nil
}

func (s *postgresWatchlistStore) GetScanTargets(ctx context.Context) ([]models.WatchlistScanTarget, error) {
	rows, err := s.db.Query(ctx,
		`SELECT DISTINCT ON (COALESCE(NULLIF(external_card_id, ''), LOWER(card_name)), COALESCE(asset_type, 'RAW'), COALESCE(slab_tier, ''), COALESCE(language_preference, 'BOTH'))
		        card_name, set_name, external_card_id, COALESCE(asset_type, 'RAW'), slab_tier, COALESCE(language_preference, 'BOTH')
		   FROM watchlists
		  WHERE target_buy_price IS NOT NULL OR target_sell_price IS NOT NULL
		  ORDER BY COALESCE(NULLIF(external_card_id, ''), LOWER(card_name)), COALESCE(asset_type, 'RAW'), COALESCE(slab_tier, ''), COALESCE(language_preference, 'BOTH'), added_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	targets := []models.WatchlistScanTarget{}
	for rows.Next() {
		var target models.WatchlistScanTarget
		if err := rows.Scan(&target.CardName, &target.SetName, &target.ExternalCardID, &target.AssetType, &target.SlabTier, &target.LanguagePreference); err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, nil
}

func (s *postgresWatchlistStore) GetDistinctCardNames(ctx context.Context) ([]string, error) {
	// SELECT DISTINCT ensures if 50 users watch "Pikachu", we only scan it ONCE.
	names := []string{}
	rows, err := s.db.Query(ctx, "SELECT DISTINCT card_name FROM watchlists")
	if err != nil {
		return names, err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return names, err
		}
		names = append(names, name)
	}
	return names, nil
}
