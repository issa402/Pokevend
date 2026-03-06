// ============================================================
// FILE: server/middleware/auth.go
// TYPE: Middleware — JWT Authentication
//
// WHAT IS MIDDLEWARE?
// Middleware is a function that wraps HTTP handlers.
// It runs BEFORE or AFTER your handler for every request on its route.
//
// WITHOUT middleware:
//   Every handler has to check JWT → duplicate code in 20 handlers
//
// WITH middleware:
//   Check JWT once → apply to all protected routes in routes.go
//   Like a security checkpoint at the front door, not inside every room.
//
// FAANG PATTERN: Authentication middleware + context injection
// The middleware validates the JWT, extracts user info, and stores it
// in the request context (r.Context()). Handlers then read it with
// middleware.GetUser(r) — no re-validating in every handler.
//
// JWT FLOW:
//   1. User logs in → POST /api/auth/login
//   2. Server creates JWT with user ID, email, expiry (7 days)
//   3. Server signs it with JWT_SECRET → encrypted token string
//   4. Browser stores token (localStorage or HTTPOnly cookie)
//   5. Every protected request: Authorization: Bearer <token>
//   6. THIS MIDDLEWARE: validates signature + expiry → puts user in context
//   7. Handlers: user := middleware.GetUser(r)
//
// GO CONCEPTS:
//   http.Handler interface, next.ServeHTTP(), context.WithValue(),
//   type assertion (v.(type)), strings.TrimPrefix()
// ============================================================
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"pokemontool/config"
)

// contextKey is a custom type for context keys.
// GO CONCEPT: Using a custom type (not just a string) prevents collisions.
// If you used string "user", any other package could accidentally overwrite it.
// Using type contextKey string means only THIS package can set/read the key.
type contextKey string

const userContextKey contextKey = "user"

// UserClaims holds the data extracted from a valid JWT.
// This is what you get when you call middleware.GetUser(r).
type UserClaims struct {
	ID    string // The user's UUID from the users table
	Email string // The user's email address
}

// RequireAuth returns an HTTP middleware that validates JWT tokens.
// It's a function that RETURNS a middleware (factory pattern).
// This lets us inject cfg (for the JWT secret) into the middleware.
//
// GO CONCEPT: func(...) func(http.Handler) http.Handler
// This is a "middleware factory" — a function that returns a middleware function.
// chi expects middlewares in the form: func(http.Handler) http.Handler
func RequireAuth(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// http.HandlerFunc converts a function into an http.Handler interface
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// ── Extract token from Authorization header ────────────
			// HTTP standard: Authorization: Bearer <token>
			header := r.Header.Get("Authorization")
			if header == "" {
				http.Error(w, `{"error":"authorization header required"}`, http.StatusUnauthorized)
				return // STOP — don't call next.ServeHTTP
			}

			// TrimPrefix removes "Bearer " from the front of the string
			tokenStr := strings.TrimPrefix(header, "Bearer ")
			if tokenStr == header { // no "Bearer " found
				http.Error(w, `{"error":"invalid authorization format"}`, http.StatusUnauthorized)
				return
			}

			// ── Validate the JWT ────────────────────────────────────
			// jwt.ParseWithClaims:
			//   1. Decodes the token
			//   2. Verifies the HMAC-SHA256 signature using our secret
			//   3. Checks the "exp" claim (expiry)
			// If ANY of these fail, token is invalid.
			token, err := jwt.ParseWithClaims(tokenStr, &jwt.MapClaims{},
				func(t *jwt.Token) (interface{}, error) {
					// Return the secret key for signature verification
					return []byte(cfg.JWTSecret), nil
				},
			)
			if err != nil || !token.Valid {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			// ── Extract claims (user data) ──────────────────────────
			claims, ok := token.Claims.(*jwt.MapClaims)
			if !ok {
				http.Error(w, `{"error":"invalid token claims"}`, http.StatusUnauthorized)
				return
			}

			// GO CONCEPT: Type assertion — claims is interface{}, we assert it's a map
			// (*claims)["id"] = the "id" field from the JWT payload
			user := UserClaims{
				ID:    (*claims)["id"].(string),    // .(string) = assert this is a string
				Email: (*claims)["email"].(string),
			}

			// ── Inject user into context ────────────────────────────
			// context.WithValue creates a NEW context with the user attached.
			// This context is threaded through the entire request lifecycle.
			// Handlers read it with: GetUser(r)
			ctx := context.WithValue(r.Context(), userContextKey, user)

			// ── Call the next handler ───────────────────────────────
			// Without this line, the request stops here.
			// next.ServeHTTP = "continue to the actual handler"
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUser retrieves the authenticated user from the request context.
// Call this in any handler that's under the RequireAuth middleware.
// Returns empty UserClaims if called on an unauthenticated route (shouldn't happen).
func GetUser(r *http.Request) UserClaims {
	// context.Value returns interface{} — we type-assert to UserClaims
	if user, ok := r.Context().Value(userContextKey).(UserClaims); ok {
		return user
	}
	return UserClaims{} // shouldn't happen if middleware is applied correctly
}

// ValidateStreamToken validates a JWT from a URL query parameter.
// Used for SSE: browsers can't set headers in EventSource, so token is in URL.
// Example: GET /api/stream?token=eyJhbGc...
func ValidateStreamToken(tokenStr, secret string) (UserClaims, bool) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwt.MapClaims{},
		func(t *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		},
	)
	if err != nil || !token.Valid {
		return UserClaims{}, false
	}
	claims := token.Claims.(*jwt.MapClaims)
	return UserClaims{
		ID:    (*claims)["id"].(string),
		Email: (*claims)["email"].(string),
	}, true
}

// TODO #1 (Practice): Add token refresh logic
// JWTs expire after 7 days in our implementation. Add a route:
//   POST /api/auth/refresh
// that accepts a valid (but possibly near-expiry) token and returns
// a new token with a fresh 7-day expiry.
// HINT: Parse the old token, verify it's valid, create a new one with
// services.AuthService.makeToken(). This is standard practice.

// TODO #2 (Practice): Add role-based authorization
// Right now, every authenticated user has equal access.
// Add a "role" field to the JWT claims: "role": "admin" or "role": "user"
// Add a RequireRole(role string) middleware that checks the claim.
// Example: only admins can access POST /api/admin/seed-cards.
// HINT: Extract role from (*claims)["role"].(string) in the middleware.
