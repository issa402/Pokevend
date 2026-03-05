// services/auth_service.go — Business logic: registration + login
// Knows WHAT to do (validate, hash, token). Calls UserStore for persistence.
package services

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"pokemontool/models"
	"pokemontool/store"
)

var (
	ErrEmailTaken      = errors.New("email already registered")
	ErrInvalidCreds    = errors.New("invalid credentials")
	ErrWeakPassword    = errors.New("password must be at least 6 characters")
)

type AuthService struct {
	users     store.UserStore
	jwtSecret string
}

func NewAuthService(users store.UserStore, jwtSecret string) *AuthService {
	return &AuthService{users: users, jwtSecret: jwtSecret}
}

func (s *AuthService) Register(ctx context.Context, email, password, displayName string) (*models.User, string, error) {
	if len(password) < 6 {
		return nil, "", ErrWeakPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, "", err
	}
	user, err := s.users.Create(ctx, email, string(hash), displayName)
	if err != nil {
		return nil, "", ErrEmailTaken
	}
	token, err := s.makeToken(user.ID, user.Email)
	return user, token, err
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*models.User, string, error) {
	user, hash, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, "", ErrInvalidCreds
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, "", ErrInvalidCreds
	}
	token, err := s.makeToken(user.ID, user.Email)
	return user, token, err
}

func (s *AuthService) makeToken(id, email string) (string, error) {
	claims := jwt.MapClaims{
		"id":    id,
		"email": email,
		"exp":   time.Now().Add(7 * 24 * time.Hour).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.jwtSecret))
}
