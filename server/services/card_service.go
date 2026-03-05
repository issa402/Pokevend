// services/card_service.go — Business logic: search, trending, price history
package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"

	"pokemontool/models"
	"pokemontool/store"
)

type CardService struct {
	cards store.CardStore
	cache *redis.Client
}

func NewCardService(cards store.CardStore, cache *redis.Client) *CardService {
	return &CardService{cards: cards, cache: cache}
}

func (s *CardService) Search(ctx context.Context, query string, limit int) ([]models.Card, string, error) {
	key := "search:" + query
	if cached := s.fromCache(ctx, key); cached != nil {
		var cards []models.Card
		json.Unmarshal(cached, &cards)
		return cards, "cache", nil
	}
	cards, err := s.cards.Search(ctx, query, limit)
	if err != nil {
		return nil, "", err
	}
	s.toCache(ctx, key, cards, 5*time.Minute)
	return cards, "database", nil
}

func (s *CardService) GetTrending(ctx context.Context) ([]models.Card, []models.Card, string, error) {
	key := "trending:all"
	type trendResult struct {
		Rising  []models.Card `json:"rising"`
		Falling []models.Card `json:"falling"`
	}
	if cached := s.fromCache(ctx, key); cached != nil {
		var r trendResult
		json.Unmarshal(cached, &r)
		return r.Rising, r.Falling, "cache", nil
	}
	rising, falling, err := s.cards.GetTrending(ctx)
	if err != nil {
		return nil, nil, "", err
	}
	s.toCache(ctx, key, trendResult{rising, falling}, 30*time.Minute)
	return rising, falling, "database", nil
}

func (s *CardService) GetPriceHistory(ctx context.Context, cardID string) (*models.Card, []models.PricePoint, error) {
	card, err := s.cards.GetByID(ctx, cardID)
	if err != nil {
		return nil, nil, err
	}
	history, err := s.cards.GetPriceHistory(ctx, cardID)
	return card, history, err
}

func (s *CardService) fromCache(ctx context.Context, key string) []byte {
	v, err := s.cache.Get(ctx, key).Bytes()
	if err != nil {
		return nil
	}
	return v
}

func (s *CardService) toCache(ctx context.Context, key string, v interface{}, ttl time.Duration) {
	if b, err := json.Marshal(v); err == nil {
		s.cache.Set(ctx, key, b, ttl)
	}
}
