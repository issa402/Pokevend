// ============================================================
// FILE: server/store/alert_store.go
// TYPE: Repository Layer — Alert SQL Queries
//
// WHAT IS THIS?
// All PostgreSQL queries for alerts live here.
// Zero business logic — pure SQL operations.
//
// ALERT-SPECIFIC QUERY PATTERNS:
//   ListByUser:   paginated list of alerts for the dashboard
//   MarkRead:     single UPDATE for one alert (with user_id check)
//   MarkAllRead:  bulk UPDATE for all a user's alerts
//   Delete:       DELETE with user_id check (security!)
//   Insert:       called by notification_worker only
//
// CRITICAL SECURITY PATTERN: Always Include user_id in WHERE Clauses
// BAD:  DELETE FROM alerts WHERE id=$1
// GOOD: DELETE FROM alerts WHERE id=$1 AND user_id=$2
//
// Why? Without the user_id check, any authenticated user can delete
// ANY alert if they know (or guess) the UUID. With user_id check,
// they can only affect their OWN alerts.
// This is an "Insecure Direct Object Reference (IDOR)" vulnerability —
// one of the OWASP Top 10 security mistakes.
//
// GO CONCEPTS:
//   pgx.ErrNoRows detection, RETURNING clause, UUID generation
// ============================================================
package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pokemontool/models"
)

// AlertStore defines the interface for all alert data operations.
// Notification worker and alert service both call this interface — never SQL directly.
type AlertStore interface {
	ListByUser(ctx context.Context, userID string, limit int) ([]models.Alert, error)
	MarkRead(ctx context.Context, id, userID string) error
	MarkAllRead(ctx context.Context, userID string) error
	Delete(ctx context.Context, id, userID string) error
	Insert(ctx context.Context, alert models.Alert) (string, error) // returns new alert's UUID
	GetWatchedAlerts(ctx context.Context, userID string) ([]models.Alert, error)
}

// postgresAlertStore implements AlertStore with PostgreSQL.

type postgresAlertStore struct{ db *pgxpool.Pool }

// NewAlertStore is the constructor — returns the interface type (AlertStore).
// main.go calls this: alertStore := store.NewAlertStore(db)
func NewAlertStore(db *pgxpool.Pool) AlertStore {
	return &postgresAlertStore{db: db}
}

// ListByUser returns the most recent alerts for a user, newest first.
// limit prevents loading thousands of alerts at once (pagination).
// ORDER BY created_at DESC = newest alerts first (most relevant to the user).
func (s *postgresAlertStore) ListByUser(ctx context.Context, userID string, limit int) ([]models.Alert, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id,user_id,card_name,alert_type,message,marketplace,price,listing_url,is_read,created_at
		 FROM alerts WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var alerts []models.Alert
	for rows.Next() {
		var a models.Alert
		rows.Scan(&a.ID, &a.UserID, &a.CardName, &a.AlertType, &a.Message,
			&a.Marketplace, &a.Price, &a.ListingURL, &a.IsRead, &a.CreatedAt)
		alerts = append(alerts, a)
	}
	return alerts, nil
}

// MarkRead marks ONE alert as read. The AND user_id=$2 is the security check.
// Returns pgx.ErrNoRows if alert doesn't exist or belongs to another user.

func (s *postgresAlertStore) MarkRead(ctx context.Context, id, userID string) error {
	result, err := s.db.Exec(ctx,
		`UPDATE alerts SET is_read=true WHERE id=$1 AND user_id=$2`, id, userID)
	if err != nil {
		return err
	}
	// pgx.Exec returns a CommandTag with RowsAffected()
	// If 0 rows affected → alert not found or wrong user
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// MarkAllRead marks ALL of a user's alerts as read in one UPDATE.
// Efficient: one SQL statement vs N individual updates.
func (s *postgresAlertStore) MarkAllRead(ctx context.Context, userID string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE alerts SET is_read=true WHERE user_id=$1 AND is_read=false`, userID)
	return err
}

// Delete removes one alert. Always verifies user_id — IDOR prevention.
func (s *postgresAlertStore) Delete(ctx context.Context, id, userID string) error {
	_, err := s.db.Exec(ctx,
		`DELETE FROM alerts WHERE id=$1 AND user_id=$2`, id, userID)
	return err
}

// Insert creates a new alert and returns its UUID.
// Called by notification_worker.go when a price threshold is triggered.
// RETURNING id = PostgreSQL returns the generated UUID immediately.
func (s *postgresAlertStore) Insert(ctx context.Context, a models.Alert) (string, error) {
	var id string
	err := s.db.QueryRow(ctx,
		`INSERT INTO alerts(user_id,card_name,alert_type,message,marketplace,price,listing_url)
		 VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		a.UserID, a.CardName, a.AlertType, a.Message, a.Marketplace, a.Price, a.ListingURL,
	).Scan(&id)
	return id, err
}

// GetWatchedAlerts returns unread alerts for a user (for notification checking).
func (s *postgresAlertStore) GetWatchedAlerts(ctx context.Context, userID string) ([]models.Alert, error) {
	return s.ListByUser(ctx, userID, 50)
}

// ============================================================
// PRACTICE TASK #3 — Add GetUnreadCount to AlertStore
// See: practice_tasks.md → Task 3
// ============================================================
//
// STEP 1: Add this line to the AlertStore interface above (between GetWatchedAlerts and the closing })
//
//   GetUnreadCount(ctx context.Context, userID string) (int, error)
//
// STEP 2: Implement the method below on postgresAlertStore.
// Signature:
//
//   func (s *postgresAlertStore) GetUnreadCount(ctx context.Context, userID string) (int, error)
//
// What it should do:
//   - Run: SELECT COUNT(*) FROM alerts WHERE user_id=$1 AND is_read=false
//   - Scan the count into a local int variable
//   - Return (count, err)
//
// Hint: use s.db.QueryRow(ctx, sql, userID).Scan(&count)
//
// Write your implementation here (delete this comment block and replace it):

// ── SOLUTION (peek only when stuck) ──────────────────────────
// Add to AlertStore interface:
//   GetUnreadCount(ctx context.Context, userID string) (int, error)
//
// func (s *postgresAlertStore) GetUnreadCount(ctx context.Context, userID string) (int, error) {
// 	var count int
// 	err := s.db.QueryRow(ctx,
// 		`SELECT COUNT(*) FROM alerts WHERE user_id=$1 AND is_read=false`,
// 		userID,
// 	).Scan(&count)
// 	return count, err
// }

// ============================================================
// PRACTICE TASK #2 (also here): Add alert auto-cleanup after 30 days
// ============================================================
//
// STEP 1: Add to AlertStore interface:
//   DeleteOlderThan(ctx context.Context, userID string) error
//
// STEP 2: Implement with SQL:
//   DELETE FROM alerts WHERE user_id=$1 AND created_at < NOW() - INTERVAL '30 days'
//
// STEP 3: Call from a goroutine in main.go:
//   go func() { for range time.NewTicker(24*time.Hour).C { alertStore.DeleteOlderThan(ctx, "") } }()
//
// ── SOLUTION ──────────────────────────────────────────────────
// func (s *postgresAlertStore) DeleteOlderThan(ctx context.Context, userID string) error {
// 	_, err := s.db.Exec(ctx,
// 		`DELETE FROM alerts WHERE user_id=$1 AND created_at < NOW() - INTERVAL '30 days'`,
// 		userID)
// 	return err
// }
