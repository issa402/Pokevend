// store/user_store.go — Repository: user SQL queries
package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"pokemontool/models"
)

type UserStore interface {
	Create(ctx context.Context, email, passwordHash, displayName string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, string, error) // returns user + passwordHash
}

type postgresUserStore struct{ db *pgxpool.Pool }

func NewUserStore(db *pgxpool.Pool) UserStore { return &postgresUserStore{db: db} }

func (s *postgresUserStore) Create(ctx context.Context, email, hash, displayName string) (*models.User, error) {
	var u models.User
	err := s.db.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, display_name)
		 VALUES ($1, $2, $3) RETURNING id, email, display_name, created_at`,
		email, hash, displayName,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *postgresUserStore) GetByEmail(ctx context.Context, email string) (*models.User, string, error) {
	var u models.User
	var hash string
	err := s.db.QueryRow(ctx,
		`SELECT id, email, display_name, password_hash, created_at FROM users WHERE email=$1`, email,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &hash, &u.CreatedAt)
	if err != nil {
		return nil, "", err
	}
	return &u, hash, nil
}
