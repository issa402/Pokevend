package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"pokemontool/models"
	"pokemontool/store"
)

type SlabOpportunityService struct {
	store          store.SlabOpportunityStore
	apiConsumerURL string
	httpClient     *http.Client
}

type LiveSlabRefreshResult struct {
	Count         int `json:"count"`
	Persisted     int `json:"persisted"`
	BuyCandidates int `json:"buyCandidates"`
	SellResearch  int `json:"sellResearch"`
}

func NewSlabOpportunityService(store store.SlabOpportunityStore, apiConsumerURL ...string) *SlabOpportunityService {
	baseURL := ""
	if len(apiConsumerURL) > 0 {
		baseURL = strings.TrimRight(apiConsumerURL[0], "/")
	}
	return &SlabOpportunityService{store: store, apiConsumerURL: baseURL, httpClient: &http.Client{Timeout: 75 * time.Second}}
}

func (s *SlabOpportunityService) List(ctx context.Context, filters models.SlabOpportunityFilters) ([]models.SlabOpportunity, error) {
	return s.store.List(ctx, filters)
}

func (s *SlabOpportunityService) RefreshLive(ctx context.Context) (*LiveSlabRefreshResult, error) {
	if s.apiConsumerURL == "" {
		return nil, fmt.Errorf("api consumer URL is not configured")
	}
	u, err := url.Parse(s.apiConsumerURL + "/live-slab-research/run")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("moverLimit", "10")
	q.Set("minProfit", "25")
	q.Set("minMarginPct", "20")
	q.Set("includeSellResearch", "true")
	q.Set("persist", "true")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("api consumer returned status %d", resp.StatusCode)
	}
	var result LiveSlabRefreshResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *SlabOpportunityService) Approve(ctx context.Context, opportunityID, userID string) (string, error) {
	return s.store.Approve(ctx, opportunityID, userID)
}

func (s *SlabOpportunityService) Reject(ctx context.Context, opportunityID string) error {
	return s.store.Reject(ctx, opportunityID)
}
