// services/show_service.go — Business logic: upcoming shows with Redis cache
package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"

	"pokemontool/models"
	"pokemontool/store"
)

type ShowService struct {
	shows store.ShowStore
	cache *redis.Client
}

func NewShowService(shows store.ShowStore, cache *redis.Client) *ShowService {
	return &ShowService{shows: shows, cache: cache}
}

func (s *ShowService) GetUpcoming(ctx context.Context) ([]models.Show, string, error) {
	key := "shows:upcoming"
	if b, err := s.cache.Get(ctx, key).Bytes(); err == nil {
		var shows []models.Show
		json.Unmarshal(b, &shows)
		return shows, "cache", nil
	}
	shows, err := s.shows.GetUpcoming(ctx)
	if err != nil {
		return nil, "", err
	}
	if b, err := json.Marshal(shows); err == nil {
		s.cache.Set(ctx, key, b, 6*time.Hour)
	}
	return shows, "database", nil
}
