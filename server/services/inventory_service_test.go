package services

import (
	"context"
	"testing"

	"pokemontool/models"
)

type fakeInventoryStore struct {
	listedID     string
	listedUserID string
	listedInput  models.StoreListingInput
	syncedID     string
	syncedUserID string
}

func (f *fakeInventoryStore) ListByUser(ctx context.Context, userID string) ([]models.InventoryItem, error) {
	return nil, nil
}

func (f *fakeInventoryStore) Insert(ctx context.Context, item models.InventoryItem) (string, error) {
	return "inventory-1", nil
}

func (f *fakeInventoryStore) BulkInsert(ctx context.Context, items []models.InventoryItem) (int, error) {
	return len(items), nil
}

func (f *fakeInventoryStore) Delete(ctx context.Context, itemID, userID string) error {
	return nil
}

func (f *fakeInventoryStore) MarkReadyForStore(ctx context.Context, itemID, userID string, input models.StoreListingInput) (*models.InventoryItem, error) {
	f.listedID = itemID
	f.listedUserID = userID
	f.listedInput = input
	status := "READY"
	return &models.InventoryItem{ID: itemID, UserID: userID, CardName: "Armored Mewtwo", Quantity: 1, StoreListingStatus: &status, StorePrice: input.StorePrice}, nil
}

func (f *fakeInventoryStore) MarkStoreSynced(ctx context.Context, itemID, userID string) error {
	f.syncedID = itemID
	f.syncedUserID = userID
	return nil
}

func TestInventoryServiceMarkReadyForStoreDefaultsQuantityAndDelegates(t *testing.T) {
	store := &fakeInventoryStore{}
	svc := NewInventoryService(store)
	price := 1250.0

	item, err := svc.MarkReadyForStore(context.Background(), "inventory-1", "user-1", models.StoreListingInput{StorePrice: &price})
	if err != nil {
		t.Fatalf("MarkReadyForStore returned error: %v", err)
	}
	if store.listedID != "inventory-1" || store.listedUserID != "user-1" {
		t.Fatalf("store called with id=%q user=%q", store.listedID, store.listedUserID)
	}
	if item.StoreListingStatus == nil || *item.StoreListingStatus != "READY" {
		t.Fatalf("expected READY status, got %#v", item.StoreListingStatus)
	}
	if item.StorePrice == nil || *item.StorePrice != price {
		t.Fatalf("expected store price %.2f, got %#v", price, item.StorePrice)
	}
	if store.syncedID != "inventory-1" || store.syncedUserID != "user-1" {
		t.Fatalf("expected store sync marker for inventory-1/user-1, got id=%q user=%q", store.syncedID, store.syncedUserID)
	}
}
