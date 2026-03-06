// ============================================================
// FILE: server/services/deal_service.go
// TYPE: Service Layer — Deal Business Logic
//
// WHAT IS THIS?
// Business logic for retrieving "Deal of the Day" cards.
// Deals are WRITTEN by Python's analytics-engine/analyzers/deal_finder.py
// and READ here via Go's store layer.
//
// THIS IS A READ-ONLY SERVICE:
// DealService only reads from the deals table — it never creates deals.
// Python creates them (writes via psycopg2 to deals table daily at 6 AM).
// Go reads them (via this service → deal_store → SELECT from deals table).
//
// ARCHITECTURE NOTE: Cross-service data sharing via shared PostgreSQL.
// Python and Go share data through the database — NOT by calling each other's HTTP APIs.
// Python writes → PostgreSQL → Go reads
// This is simple, reliable, and avoids service-to-service authentication complexity.
//
// CACHING STRATEGY:
// Deals of the day change once per day. Cache them for 6 hours.
// If deals are updated mid-day (manual re-run), they'll appear within 6 hours.
// For immediate updates: call InvalidateDealsCache via an admin endpoint.
// ============================================================
package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"

	"pokemontool/models"
	"pokemontool/store"
)

// DealService handles deal retrieval with aggressive Redis caching.
type DealService struct {
	store store.DealStore
	cache *redis.Client
}

// NewDealService wires the store and cache via dependency injection.
func NewDealService(store store.DealStore, cache *redis.Client) *DealService {
	return &DealService{store: store, cache: cache}
}

// GetToday returns today's deals — cached for 6 hours (they only change once/day).
// Cache key "deals:today" — simple because there's only one "today" at any moment.
func (s *DealService) GetToday(ctx context.Context) ([]models.Deal, error) {
	if cached, err := s.cache.Get(ctx, "deals:today").Bytes(); err == nil {
		var deals []models.Deal
		if json.Unmarshal(cached, &deals) == nil {
			return deals, nil
		}
	}
	deals, err := s.store.GetToday(ctx)
	if err != nil {
		return nil, err
	}
	if bytes, err := json.Marshal(deals); err == nil {
		// 6-hour TTL — deals are refreshed by Python once per day
		s.cache.Set(ctx, "deals:today", bytes, 6*time.Hour)
	}
	return deals, nil
}

// InvalidateDealsCache clears the deals cache — call after Python re-runs deal_finder.
// Admin endpoint: POST /api/admin/refresh-deals → calls this.
func (s *DealService) InvalidateDealsCache(ctx context.Context) {
	s.cache.Del(ctx, "deals:today")
}

// TODO #1 (Practice): Add DealHistory(ctx, days int) for historical deals
// The dashboard could show "Best deals from this week."
// Add: store.DealStore.GetHistory(ctx, days) → SELECT from deals WHERE deal_date >= NOW()-N days
// Service: cache with key "deals:history:7" with 1-hour TTL
// New route: GET /api/deals/history?days=7

// TODO #2 (Practice): Add a best deal of the week computed field
// After GetToday() returns deals, compute the single best deal:
//   func (s *DealService) BestDealToday(ctx context.Context) (*models.Deal, error)
// Business logic: sort by savings_pct DESC, return first element.
// Cache key: "deals:best:today" with same TTL as deals:today.
// Show this as a featured "Deal of the Day" hero card on the dashboard.
