// services/watchlist_service.go — Business logic: watchlist management
package services

import (
	"context"
	"errors"

	"pokemontool/models"
	"pokemontool/store"
)

var ErrCardNameRequired = errors.New("cardName is required")

type WatchlistService struct{ wl store.WatchlistStore }

func NewWatchlistService(wl store.WatchlistStore) *WatchlistService {
	return &WatchlistService{wl: wl}
}

func (s *WatchlistService) List(ctx context.Context, userID string) ([]models.WatchlistItem, error) {
	return s.wl.ListByUser(ctx, userID)
}

func (s *WatchlistService) Add(ctx context.Context, userID, cardName, setName string, buyPrice, sellPrice *float64, notes string) (*models.WatchlistItem, error) {
	if cardName == "" {
		return nil, ErrCardNameRequired
	}
	item := models.WatchlistItem{
		UserID:          userID,
		CardName:        cardName,
		TargetBuyPrice:  buyPrice,
		TargetSellPrice: sellPrice,
	}
	if setName != "" {
		item.SetName = &setName
	}
	if notes != "" {
		item.Notes = &notes
	}
	return s.wl.Insert(ctx, item)
}

func (s *WatchlistService) Remove(ctx context.Context, itemID, userID string) error {
	return s.wl.Delete(ctx, itemID, userID)
}

func (s *WatchlistService) GetDistinctCardNames(ctx context.Context) ([]string, error) {
	return s.wl.GetDistinctCardNames(ctx)

}
