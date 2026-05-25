// services/watchlist_service.go — Business logic: watchlist management
package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"pokemontool/models"
	"pokemontool/store"
)

var ErrCardNameRequired = errors.New("cardName is required")

type WatchlistService struct {
	wl             store.WatchlistStore
	cards          store.CardStore
	apiConsumerURL string
	httpClient     *http.Client
}

func NewWatchlistService(wl store.WatchlistStore, apiConsumerURL string, cards ...store.CardStore) *WatchlistService {
	s := &WatchlistService{
		wl:             wl,
		apiConsumerURL: strings.TrimRight(apiConsumerURL, "/"),
		httpClient:     &http.Client{Timeout: 45 * time.Second},
	}
	if len(cards) > 0 {
		s.cards = cards[0]
	}
	return s
}

func (s *WatchlistService) List(ctx context.Context, userID string) ([]models.WatchlistItem, error) {
	return s.wl.ListByUser(ctx, userID)
}

func (s *WatchlistService) Add(ctx context.Context, item models.WatchlistItem) (*models.WatchlistItem, error) {
	if item.CardName == "" {
		return nil, ErrCardNameRequired
	}
	if item.AssetType == "" {
		item.AssetType = "RAW"
	}
	item.LanguagePreference = normalizeLanguagePreference(item.LanguagePreference)
	if item.PriceSource == nil {
		source := "manual"
		item.PriceSource = &source
	}
	inserted, err := s.wl.Insert(ctx, item)
	if err != nil {
		return nil, err
	}
	if s.cards != nil && item.ExternalCardID != nil && item.MarketPrice != nil {
		if snapshotErr := s.cards.UpsertPriceSnapshot(ctx, *item.ExternalCardID, "tcgplayer", *item.MarketPrice); snapshotErr != nil {
			// Snapshot failure should not block a user's watchlist action.
		}
	}
	go s.triggerImmediateScan(*inserted)
	return inserted, nil
}

func (s *WatchlistService) Remove(ctx context.Context, itemID, userID string) error {
	return s.wl.Delete(ctx, itemID, userID)
}

func (s *WatchlistService) GetDistinctCardNames(ctx context.Context) ([]string, error) {
	return s.wl.GetDistinctCardNames(ctx)

}

func (s *WatchlistService) GetScanTargets(ctx context.Context) ([]models.WatchlistScanTarget, error) {
	return s.wl.GetScanTargets(ctx)
}

func (s *WatchlistService) triggerImmediateScan(item models.WatchlistItem) {
	if s.apiConsumerURL == "" || item.CardName == "" {
		return
	}

	tiers := []string{""}
	if item.AssetType == "ALL_SLABS" {
		tiers = []string{
			"PSA_10", "PSA_9", "PSA_8", "PSA_7",
			"CGC_10", "CGC_9_5", "CGC_9",
			"BGS_10", "BGS_9_5", "BGS_9",
		}
	} else if item.AssetType == "SLAB" && item.SlabTier != nil && *item.SlabTier != "" {
		tiers = []string{*item.SlabTier}
	}

	for _, tier := range tiers {
		assetType := item.AssetType
		if item.AssetType == "ALL_SLABS" {
			assetType = "SLAB"
		}
		if err := s.triggerOneScan(item, assetType, tier); err != nil {
			log.Printf("[watchlist] immediate scan failed for %s %s: %v", item.CardName, tier, err)
		}
		if item.AssetType == "ALL_SLABS" {
			time.Sleep(750 * time.Millisecond)
		}
	}
}

func (s *WatchlistService) triggerOneScan(item models.WatchlistItem, assetType string, slabTier string) error {
	u, err := url.Parse(s.apiConsumerURL + "/ebay/search")
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("cardName", item.CardName)
	q.Set("assetType", assetType)
	q.Set("publish", "true")
	if item.ExternalCardID != nil {
		q.Set("externalCardId", *item.ExternalCardID)
	}
	if item.SetName != nil {
		q.Set("setName", *item.SetName)
	}
	q.Set("languagePreference", normalizeLanguagePreference(item.LanguagePreference))
	if slabTier != "" {
		q.Set("slabTier", slabTier)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("api-consumer returned status %d", resp.StatusCode)
	}
	return nil
}

func normalizeLanguagePreference(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "ENGLISH", "EN":
		return "ENGLISH"
	case "JAPANESE", "JP", "JPN":
		return "JAPANESE"
	default:
		return "BOTH"
	}
}
