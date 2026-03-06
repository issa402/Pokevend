// ============================================================
// FILE: server/handlers/auth_handler.go
// TYPE: Handler Layer — HTTP Request/Response for Auth
//
// WHAT IS THE HANDLER LAYER?
// Handlers are the HTTP layer. They have ONE job: translate between
// HTTP and your services. A handler:
//   1. Reads the HTTP request (headers, body, URL params)
//   2. Calls the appropriate service method
//   3. Writes the HTTP response (status code + JSON body)
//
// HANDLER RULES (STRICTLY ENFORCED AT FAANG):
//   ✅ Parse request body (json.Decode)
//   ✅ Extract URL parameters (chi.URLParam)
//   ✅ Call ONE service method
//   ✅ Write response (pkg.JSON or pkg.Error)
//   ❌ NO SQL — that's the store's job
//   ❌ NO business rules — that's the service's job
//   ❌ NO direct database access
//
// FAANG PATTERN: Thin Controllers
// A handler should almost never be more than ~30 lines.
// If it's longer, extract logic into a service.
//
// GO CONCEPTS:
//   json.NewDecoder, errors.Is(), http.StatusCreated (201),
//   struct with json tags, method receivers
// ============================================================
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"pokemontool/pkg"
	"pokemontool/services"
)

// AuthHandler holds the service dependency.
// This is injected in main.go: handlers.NewAuthHandler(authSvc)
type AuthHandler struct {
	svc *services.AuthService
}

// NewAuthHandler is the constructor.
// Returns *AuthHandler so routes.go can call handler.Register, handler.Login.
func NewAuthHandler(svc *services.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// Register handles POST /api/auth/register
// HTTP flow:
//   Receive JSON body → call AuthService.Register → return JWT + user
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	// ── Parse Request Body ─────────────────────────────────────
	// Define an anonymous struct for the expected JSON shape.
	// json.NewDecoder reads from the request body (a stream).
	// Decode fills the struct with values from the JSON.
	var body struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		// If the body is missing or malformed JSON → 400 Bad Request
		pkg.Error(w, http.StatusBadRequest, "invalid request body")
		return // ALWAYS return after writing an error response
	}

	// ── Call Service ───────────────────────────────────────────
	// Handler doesn't know HOW registration works — it just calls the service.
	// The service returns (user, token, error).
	user, token, err := h.svc.Register(r.Context(), body.Email, body.Password, body.DisplayName)
	if err != nil {
		// Translate service-layer errors to HTTP status codes.
		// errors.Is() checks if err IS (or wraps) the target error.
		// This is the correct way — not comparing strings.
		status := http.StatusInternalServerError // default: unknown error
		if errors.Is(err, services.ErrEmailTaken) {
			status = http.StatusConflict    // 409 = resource already exists
		}
		if errors.Is(err, services.ErrWeakPassword) {
			status = http.StatusBadRequest  // 400 = client sent bad data
		}
		pkg.Error(w, status, err.Error())
		return
	}

	// ── Write Response ─────────────────────────────────────────
	// 201 Created = resource was successfully created (not 200 OK)
	// HTTP STATUS CODES you must know:
	//   200 OK           - successful GET/POST
	//   201 Created      - successful resource creation
	//   400 Bad Request  - client sent invalid data
	//   401 Unauthorized - not logged in
	//   403 Forbidden    - logged in but no permission
	//   404 Not Found    - resource doesn't exist
	//   409 Conflict     - duplicate resource
	//   500 Internal     - server bug
	pkg.JSON(w, http.StatusCreated, map[string]interface{}{
		"token": token,
		"user":  user,
	})
}

// Login handles POST /api/auth/login
// HTTP flow: Receive credentials → call AuthService.Login → return JWT + user
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		pkg.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, token, err := h.svc.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		// 401 Unauthorized — credentials are wrong or user doesn't exist.
		// We use the SAME error message regardless — don't reveal "user not found vs wrong password."
		pkg.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	pkg.JSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"user":  user,
	})
}

// TODO #1 (Practice): Add input validation before calling the service
// The service validates password length, but the handler should also:
//   - Check that email and password are not empty strings
//   - Check email format (does it contain "@" and "."?)
// Return 400 Bad Request with a clear message for each validation failure.
// FAANG PATTERN: "Validate at the boundary" — catch bad input in the handler,
// let the service focus on business logic (not input sanitization).

// TODO #2 (Practice): Add the /api/auth/me endpoint
// Users often need to verify their token is still valid and get their profile.
// Add a Me() method: GET /api/auth/me (protected — under RequireAuth middleware)
// The handler should:
//   1. Get the user from context: middleware.GetUser(r)
//   2. (Optional) fetch full user profile from users store
//   3. Return user data as JSON
// Register the route in routes.go: r.Get("/auth/me", authH.Me)
