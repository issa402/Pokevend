// services/deal_service.go — Business logic: deal of the day with Redis cache
package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"

	"pokemontool/models"
	"pokemontool/store"
)

type DealService struct {
	deals store.DealStore
	cache *redis.Client
}

func NewDealService(deals store.DealStore, cache *redis.Client) *DealService {
	return &DealService{deals: deals, cache: cache}
}

func (s *DealService) GetToday(ctx context.Context) ([]models.Deal, string, error) {
	today := time.Now().Format("2006-01-02")
	key   := "deals:" + today

	if b, err := s.cache.Get(ctx, key).Bytes(); err == nil {
		var deals []models.Deal
		json.Unmarshal(b, &deals)
		return deals, "cache", nil
	}

	deals, err := s.deals.GetByDate(ctx, today)
	if err != nil {
		return nil, "", err
	}

	if b, err := json.Marshal(deals); err == nil {
		s.cache.Set(ctx, key, b, 30*time.Minute)
	}
	return deals, "database", nil
}
