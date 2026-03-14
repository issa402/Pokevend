package store 

import (
	"context"
	"time"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pokemontool/models"

)

type PriceAlertStore interface(
	GetActiveAlertsForCard(ctx context.Context, userID string, cardName string) ([]models.PriceAlertsSetting, error)
)

type postgresPriceAlertStore struct{
	db *pgxpool.Pool
}

func NewPriceAlertStore(db *pgxpool.Pool) Alerts {
	return &postgresPriceAlertStore{db : db}
}

func (s* postgresPriceAlertStore) GetActiveAlertsForCard(ctx context, cardName string) ([]models.PriceAlertsSetting, error) {
	query = `
			SELECT id, user_id, card_name, threhold, direction
			FROM price_alerts_settings
			WHERE card_name = $1 AND is_active = true`
	
	rows, err := s.db.Query(ctx, query, cardName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []models.PriceAlertSetting 
	for rows.Next(){
		var s models.PriceAlertSetting 
		err := rows.Scan(&s.ID, &s.UserID, &s.CardName, &s.Threshold, &s.Direction )
		if err != nil {
			return nil, err
		}
		settings = append(setting,s)
	}
	return settings, nil
}