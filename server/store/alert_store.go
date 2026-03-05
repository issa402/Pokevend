// store/alert_store.go — Repository: alert SQL queries
package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"pokemontool/models"
)

type AlertStore interface {
	ListByUser(ctx context.Context, userID string) ([]models.Alert, error)
	Insert(ctx context.Context, a models.Alert) (string, error)
	MarkRead(ctx context.Context, alertID, userID string) error
	MarkAllRead(ctx context.Context, userID string) error
	Delete(ctx context.Context, alertID, userID string) error
}

type postgresAlertStore struct{ db *pgxpool.Pool }

func NewAlertStore(db *pgxpool.Pool) AlertStore { return &postgresAlertStore{db: db} }

func (s *postgresAlertStore) ListByUser(ctx context.Context, userID string) ([]models.Alert, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id,user_id,card_name,alert_type,message,marketplace,price,listing_url,is_read,created_at
		 FROM alerts WHERE user_id=$1 ORDER BY created_at DESC LIMIT 100`, userID,
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

func (s *postgresAlertStore) Insert(ctx context.Context, a models.Alert) (string, error) {
	var id string
	err := s.db.QueryRow(ctx,
		`INSERT INTO alerts (user_id,card_name,alert_type,message,marketplace,price,listing_url)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		a.UserID, a.CardName, a.AlertType, a.Message, a.Marketplace, a.Price, a.ListingURL,
	).Scan(&id)
	return id, err
}

func (s *postgresAlertStore) MarkRead(ctx context.Context, alertID, userID string) error {
	_, err := s.db.Exec(ctx, `UPDATE alerts SET is_read=true WHERE id=$1 AND user_id=$2`, alertID, userID)
	return err
}

func (s *postgresAlertStore) MarkAllRead(ctx context.Context, userID string) error {
	_, err := s.db.Exec(ctx, `UPDATE alerts SET is_read=true WHERE user_id=$1`, userID)
	return err
}

func (s *postgresAlertStore) Delete(ctx context.Context, alertID, userID string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM alerts WHERE id=$1 AND user_id=$2`, alertID, userID)
	return err
}
