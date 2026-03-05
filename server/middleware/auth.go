// ============================================================
// middleware/auth.go — JWT validation middleware
// Reads Bearer token from Authorization header, validates it,
// and stores the user ID + email in request context.
// ============================================================
package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"pokemontool/config"

	"github.com/golang-jwt/jwt/v5"
)

// Context key type to avoid collisions
type contextKey string

const UserKey contextKey = "user"

type UserClaims struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// RequireAuth validates JWT and injects user claims into context
func RequireAuth(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				writeError(w, http.StatusUnauthorized, "missing or invalid Authorization header")
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims := jwt.MapClaims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
				return []byte(cfg.JWTSecret), nil
			})
			if err != nil || !token.Valid {
				writeError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			user := UserClaims{
				ID:    toString(claims["id"]),
				Email: toString(claims["email"]),
			}
			ctx := context.WithValue(r.Context(), UserKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUser pulls the authenticated user from context
func GetUser(r *http.Request) UserClaims {
	if u, ok := r.Context().Value(UserKey).(UserClaims); ok {
		return u
	}
	return UserClaims{}
}

// ValidateStreamToken validates a JWT passed as a query param (for SSE)
func ValidateStreamToken(tokenStr, secret string) (UserClaims, bool) {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return UserClaims{}, false
	}
	return UserClaims{ID: toString(claims["id"]), Email: toString(claims["email"])}, true
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func toString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
