// ============================================================
// FILE: server/services/alert_service.go
// TYPE: Service Layer — Alert Business Logic
//
// WHAT IS THIS?
// Business logic for alert operations.
// All methods here are thin wrappers around the store —
// alerts have little business logic beyond CRUD.
//
// WHEN SERVICES ARE THIN (like this one) vs THICK (like AuthService):
//   Thin: alert management — just CRUD, no real decisions
//   Thick: auth — bcrypt, JWT, named errors, security logic
// Both are correct. Service thickness reflects domain complexity.
//
// NOTE: List() currently counts unread by looping in Go-land.
// That's fine for small result sets. For large sets (100+ alerts),
// use GetUnreadCount() which does COUNT(*) in SQL (Task 4 below).
// ============================================================
package services

import (
	"context"
	"pokemontool/models"
	"pokemontool/store"
)

// AlertService handles alert business logic.
type AlertService struct{ alerts store.AlertStore }

// NewAlertService injects the alert store.
func NewAlertService(alerts store.AlertStore) *AlertService {
	return &AlertService{alerts: alerts}
}

// List returns alerts + an in-memory unread count.
// For a dedicated count endpoint, see GetUnreadCount() below (Task 4).
func (s *AlertService) List(ctx context.Context, userID string, limit int) ([]models.Alert, error) {
	return s.alerts.ListByUser(ctx, userID, limit)
}

// MarkRead marks ONE alert as read. Returns error if not found or wrong user.
func (s *AlertService) MarkRead(ctx context.Context, alertID, userID string) error {
	return s.alerts.MarkRead(ctx, alertID, userID)
}

// MarkAllRead marks ALL of a user's alerts as read.
func (s *AlertService) MarkAllRead(ctx context.Context, userID string) error {
	return s.alerts.MarkAllRead(ctx, userID)
}

// Delete permanently removes one alert (with user_id security check).
func (s *AlertService) Delete(ctx context.Context, alertID, userID string) error {
	return s.alerts.Delete(ctx, alertID, userID)
}

// ============================================================
// PRACTICE TASK #4 — Add GetUnreadCount to AlertService
// See: practice_tasks.md → Task 4
// ============================================================
//
// STEP 1: After completing Task 3 (adding GetUnreadCount to store),
//         add this method to AlertService.
//
// Signature:
//   func (s *AlertService) GetUnreadCount(ctx context.Context, userID string) (int, error)
//
// What it should do:
//   - Call s.alerts.GetUnreadCount(ctx, userID)
//   - Return (count, err) — no other logic needed
//
// Why no caching? The unread count changes every time the user reads/dismisses
// an alert. Caching it would show a stale badge number.
//
// Write your implementation here (delete this comment block):

// ── SOLUTION (peek only when stuck) ──────────────────────────
// func (s *AlertService) GetUnreadCount(ctx context.Context, userID string) (int, error) {
// 	return s.alerts.GetUnreadCount(ctx, userID)
// }
