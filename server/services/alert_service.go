// services/alert_service.go — Business logic: alert management
package services

import (
	"context"
	"pokemontool/models"
	"pokemontool/store"
)

type AlertService struct{ alerts store.AlertStore }

func NewAlertService(alerts store.AlertStore) *AlertService {
	return &AlertService{alerts: alerts}
}

func (s *AlertService) List(ctx context.Context, userID string) ([]models.Alert, int, error) {
	alerts, err := s.alerts.ListByUser(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	unread := 0
	for _, a := range alerts {
		if !a.IsRead {
			unread++
		}
	}
	return alerts, unread, nil
}

func (s *AlertService) MarkRead(ctx context.Context, alertID, userID string) error {
	return s.alerts.MarkRead(ctx, alertID, userID)
}

func (s *AlertService) MarkAllRead(ctx context.Context, userID string) error {
	return s.alerts.MarkAllRead(ctx, userID)
}

func (s *AlertService) Delete(ctx context.Context, alertID, userID string) error {
	return s.alerts.Delete(ctx, alertID, userID)
}
