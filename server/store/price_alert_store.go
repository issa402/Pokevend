package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"errors"
	"pokemontool/models"
)

type PriceAlertStore interface {
	GetActiveAlertsForCard(ctx context.Context, cardName string) ([]models.PriceAlertSettings, error)
	Create(ctx context.Context, userID string, card string, price float64, dir string) error
	ListByUser(ctx context.Context, userID string) ([]models.PriceAlertSettings, error)
	Delete(ctx context.Context, id string, userID string) error
}

type postgresPriceAlertStore struct {
	db *pgxpool.Pool
}

func NewPriceAlertStore(db *pgxpool.Pool) PriceAlertStore {
	return &postgresPriceAlertStore{db: db}
}

func (s *postgresPriceAlertStore) GetActiveAlertsForCard(ctx context.Context, cardName string) ([]models.PriceAlertSettings, error) {
	query := `
			SELECT id, user_id, card_name, threhold, direction
			FROM price_alerts_settings
			WHERE card_name = $1 AND is_active = true;`

	rows, err := s.db.Query(ctx, query, cardName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []models.PriceAlertSettings
	for rows.Next() {
		var setting models.PriceAlertSettings
		err := rows.Scan(&setting.ID, &setting.UserID, &setting.CardName, &setting.Threshold, &setting.Direction)
		if err != nil {
			return nil, err
		}
		settings = append(settings, setting)
	}
	return settings, nil
}

func (s *postgresPriceAlertStore) Create(ctx context.Context, userID string, cardName string, price float64, dir string) error {
	query := `INSERT INTO price_alerts_settings (user_id, card_name, threshold, direction)
			  VALUES ($1, $2, $3, $4)`

	_, err := s.db.Exec(ctx, query, userID, cardName, price, dir)
	return err
}

func (s *postgresPriceAlertStore) ListByUser(ctx context.Context, userID string) ([]models.PriceAlertSettings, error) {
	query := `SELECT id, user_id, card_name, threshold, direction 
	          FROM price_alerts_settings WHERE user_id = $1`

	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []models.PriceAlertSettings
	for rows.Next() {
		var setting models.PriceAlertSettings
		err := rows.Scan(&setting.ID, &setting.UserID, &setting.CardName, &setting.Threshold, &setting.Direction)
		if err != nil {
			return nil, err
		}
		settings = append(settings, setting)
	}
	return settings, nil
}

func (s *postgresPriceAlertStore) Delete(ctx context.Context, id string, userID string) error {
	// We include user_id in the WHERE clause to prevent IDOR (deleting someone else's alert)
	query := `DELETE FROM price_alerts_settings WHERE id = $1 AND user_id = $2`
	result, err := s.db.Exec(ctx, query, id, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New("alert not found") // Or a custom "not found" error
	}
	return nil
}
