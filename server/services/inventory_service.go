// services/inventory_service.go — Business logic: inventory + CSV parsing
package services

import (
	"context"
	"encoding/csv"
	"io"
	"strconv"

	"pokemontool/models"
	"pokemontool/store"
)

type InventoryService struct {
	inv   store.InventoryStore
	cards store.CardStore
}

func NewInventoryService(inv store.InventoryStore, cards ...store.CardStore) *InventoryService {
	s := &InventoryService{inv: inv}
	if len(cards) > 0 {
		s.cards = cards[0]
	}
	return s
}

func (s *InventoryService) List(ctx context.Context, userID string) ([]models.InventoryItem, error) {
	return s.inv.ListByUser(ctx, userID)
}

func (s *InventoryService) Add(ctx context.Context, item models.InventoryItem) (string, error) {
	if item.Quantity <= 0 {
		item.Quantity = 1
	}
	id, err := s.inv.Insert(ctx, item)
	if err == nil && s.cards != nil && item.ExternalCardID != nil && item.CurrentValue != nil {
		if snapshotErr := s.cards.UpsertPriceSnapshot(ctx, *item.ExternalCardID, "tcgplayer", *item.CurrentValue); snapshotErr != nil {
			// Inventory saves should not fail just because history snapshot storage failed.
		}
	}
	return id, err
}

func (s *InventoryService) Delete(ctx context.Context, itemID, userID string) error {
	return s.inv.Delete(ctx, itemID, userID)
}

// ImportCSV parses a CSV reader and bulk-inserts items for the user
func (s *InventoryService) ImportCSV(ctx context.Context, userID string, r io.Reader) (int, error) {
	records, err := csv.NewReader(r).ReadAll()
	if err != nil || len(records) < 2 {
		return 0, err
	}
	var items []models.InventoryItem
	for _, row := range records[1:] { // skip header row
		if len(row) < 1 {
			continue
		}
		qty, _ := strconv.Atoi(safeGet(row, 4))
		if qty <= 0 {
			qty = 1
		}
		item := models.InventoryItem{
			UserID:   userID,
			CardName: row[0],
			Quantity: qty,
		}
		if v := safeGet(row, 1); v != "" {
			item.SetName = &v
		}
		if v := safeGet(row, 2); v != "" {
			item.CardNumber = &v
		}
		if v := safeGet(row, 3); v != "" {
			item.Condition = &v
		}
		items = append(items, item)
	}
	return s.inv.BulkInsert(ctx, items)
}

func safeGet(row []string, i int) string {
	if i < len(row) {
		return row[i]
	}
	return ""
}
