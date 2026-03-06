// ============================================================
// FILE: server/services/auth_service.go
// TYPE: Service Layer — Authentication Business Logic
//
// WHAT IS THE SERVICE LAYER?
// Services contain BUSINESS LOGIC — the rules that make your app unique.
// They don't know about HTTP (no http.Request, no http.ResponseWriter).
// They don't know about SQL (they call Store interfaces).
// They ONLY know about business rules.
//
// WHAT COUNTS AS BUSINESS LOGIC?
//   ✅ "Passwords must be at least 6 characters"         → Service
//   ✅ "Email must be unique" (enforced + caught)        → Service
//   ✅ "JWT expires in 7 days"                          → Service
//   ✅ "Use bcrypt cost factor 12"                      → Service
//   ❌ "Parse JSON from request body"                   → Handler
//   ❌ "Write 401 status code"                          → Handler
//   ❌ "Run SELECT query"                               → Store
//
// FAANG PATTERN: Named Error Variables
// Instead of returning error.New("email taken") (hard to check in tests),
// we define package-level error variables.
// Handlers can then do: errors.Is(err, services.ErrEmailTaken)
// This is type-safe and refactor-friendly.
//
// GO CONCEPTS:
//   var ErrX = errors.New(), errors.Is(), bcrypt, JWT signing,
//   method receivers (s *AuthService), struct initialization
// ============================================================
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

// ── Named Business Logic Errors ───────────────────────────────
// These are the "business rule violations."
// Handlers check these with errors.Is() to return the right HTTP status.
// NEVER return raw SQL errors to clients — that leaks implementation details.
var (
	ErrEmailTaken   = errors.New("email already registered")
	ErrInvalidCreds = errors.New("invalid credentials")
	ErrWeakPassword = errors.New("password must be at least 6 characters")
)

// AuthService handles all authentication business logic.
// It receives a UserStore interface — it could be PostgreSQL, DynamoDB, or a mock.
// It also receives jwtSecret — the signing key for JWTs.
type AuthService struct {
	users     store.UserStore // interface — not *postgresUserStore
	jwtSecret string
}

// NewAuthService is the constructor — receives dependencies via injection.
// Called in main.go: services.NewAuthService(userStore, cfg.JWTSecret)
func NewAuthService(users store.UserStore, jwtSecret string) *AuthService {
	return &AuthService{users: users, jwtSecret: jwtSecret}
}

// Register creates a new user account.
// Business rules enforced here:
//   1. Password must be at least 6 characters
//   2. Email must be unique (enforced by DB, caught here)
//   3. Password is ALWAYS hashed before storing — never plaintext
//   4. JWT is issued immediately on registration (no separate login step)
func (s *AuthService) Register(ctx context.Context, email, password, displayName string) (*models.User, string, error) {
	// Rule 1: Password strength validation
	if len(password) < 6 {
		return nil, "", ErrWeakPassword
	}

	// BCRYPT: a one-way hashing algorithm designed for passwords.
	// Cost factor 12 = ~250ms to hash on modern hardware.
	// This makes brute-force attacks very slow (250ms * billions of guesses = years).
	// NEVER use MD5 or SHA256 for passwords — they're too fast to brute-force.
	// FAANG STANDARD: bcrypt with cost 10-12, or argon2id (newer, stronger).
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, "", err
	}

	// Attempt to create user — if email exists, PostgreSQL UNIQUE constraint fires.
	// The store returns an error on duplicate email → we translate to ErrEmailTaken.
	user, err := s.users.Create(ctx, email, string(hash), displayName)
	if err != nil {
		// Don't return the raw DB error (it would say "duplicate key value violates unique constraint")
		// Return our clean business error instead.
		return nil, "", ErrEmailTaken
	}

	// Issue JWT immediately so user doesn't have to log in separately after registering.
	token, err := s.makeToken(user.ID, user.Email)
	return user, token, err
}

// Login verifies credentials and returns a JWT on success.
// Business rules:
//   1. User must exist (by email)
//   2. Password must match the stored bcrypt hash
//   3. On success: issue a fresh JWT
func (s *AuthService) Login(ctx context.Context, email, password string) (*models.User, string, error) {
	// GetByEmail returns the user AND their password hash.
	// We keep the hash separate from models.User — it should never be serialized to JSON.
	user, hash, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		// If the user doesn't exist, return a GENERIC error ("invalid credentials")
		// NOT "user not found" — that leaks information about which emails are registered.
		// This is the FAANG standard for security: never confirm or deny email existence.
		return nil, "", ErrInvalidCreds
	}

	// CompareHashAndPassword securely checks if the password matches the bcrypt hash.
	// It's timing-safe — the comparison takes the same time regardless of how similar
	// the passwords are (prevents timing attacks).
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, "", ErrInvalidCreds
	}

	token, err := s.makeToken(user.ID, user.Email)
	return user, token, err
}

// makeToken creates and signs a JWT.
// This is a private method — only AuthService uses it.
//
// JWT STRUCTURE: three base64-encoded parts separated by dots
//   Header.Payload.Signature
//   - Header: {"alg": "HS256", "typ": "JWT"}
//   - Payload (claims): {"id": "uuid", "email": "...", "exp": 1234567890}
//   - Signature: HMAC-SHA256(header + "." + payload, jwtSecret)
//
// Anyone can decode the payload (it's just base64), but they can't
// FORGE a valid token without knowing the secret key.
func (s *AuthService) makeToken(id, email string) (string, error) {
	// jwt.MapClaims = a map of key-value pairs for the JWT payload
	claims := jwt.MapClaims{
		"id":    id,
		"email": email,
		// "exp" = expiry — Unix timestamp. Token is invalid after this time.
		"exp": time.Now().Add(7 * 24 * time.Hour).Unix(),
	}
	// SignedString signs with HMAC-SHA256 using our secret key.
	// This produces the final "eyJhbGc..." string you send to the client.
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.jwtSecret))
}

// TODO #1 (Practice): Add email validation
// Before calling s.users.Create(), validate that email is a valid email format.
// Use the standard library: strings.Contains(email, "@") is too naive.
// Research: regexp.MustCompile for a proper email regex validator,
// OR use the "net/mail" package: mail.ParseAddress(email)
// Return a new error: var ErrInvalidEmail = errors.New("invalid email format")

// TODO #2 (Practice): Implement password reset
// Add: RequestReset(ctx, email string) (resetToken string, error)
// Business logic:
//   1. Confirm the email exists
//   2. Generate a random 32-byte token: crypto/rand.Read()
//   3. Store the token + expiry in a new "password_resets" table
//   4. Return the token (caller would email it)
// Add: ConfirmReset(ctx, token, newPassword string) error
//   1. Look up the token in the table, verify not expired
//   2. Hash the new password
//   3. Update the user's password_hash
//   4. Delete the reset token (one-time use)
