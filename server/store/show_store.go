// store/show_store.go — Repository: show SQL queries
package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"pokemontool/models"
)

type ShowStore interface {
	GetUpcoming(ctx context.Context) ([]models.Show, error)
}

type postgresShowStore struct{ db *pgxpool.Pool }

func NewShowStore(db *pgxpool.Pool) ShowStore { return &postgresShowStore{db: db} }

func (s *postgresShowStore) GetUpcoming(ctx context.Context) ([]models.Show, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id,name,venue_name,city,state,start_date,end_date,event_url
		 FROM shows WHERE start_date >= NOW() ORDER BY start_date ASC LIMIT 50`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var shows []models.Show
	for rows.Next() {
		var sh models.Show
		rows.Scan(&sh.ID, &sh.Name, &sh.VenueName, &sh.City, &sh.State, &sh.StartDate, &sh.EndDate, &sh.EventURL)
		shows = append(shows, sh)
	}
	return shows, nil
}
