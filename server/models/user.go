// ============================================================
// FILE: server/models/user.go
// TYPE: Models Layer — User Domain Type
//
// WHAT IS THIS?
// The User model — the central entity everything else belongs to.
// Every watchlist, alert, inventory item, and API key has a user_id.
//
// IMPORTANT: This struct does NOT have a PasswordHash field.
// Why? Because models.User is serialized to JSON in API responses.
// If PasswordHash were in the struct with json:"passwordHash",
// it would accidentally leak bcrypt hashes to the frontend.
//
// FAANG SECURITY PATTERN: Never include sensitive fields in response models.
// The password hash lives only in memory during auth (store/user_store.go
// returns it separately via GetByEmail, not part of User).
//
// json:"-" tag: if you EVER add a sensitive field to a response struct,
// use json:"-" to TOTALLY exclude it from JSON serialization.
// ============================================================
package models

import "time"

// User represents a vendor account in the system.
// Populated from the users table by store/user_store.go.

type User struct {
	ID          string     `json:"id"`          // UUID primary key
	Email       string     `json:"email"`
	DisplayName *string    `json:"displayName"` // nullable — not required during registration
	ZipCode     *string    `json:"zipCode"`     // nullable — used for nearby show search
	CreatedAt   *time.Time `json:"createdAt"`
	UpdatedAt   *time.Time `json:"updatedAt"`   // auto-updated by PostgreSQL trigger

	// NOTE: PasswordHash is intentionally NOT here.
	// store/user_store.go's GetByEmail() returns (User, hash string, error)
	// separating the hash from the serializable User model.
}

// TODO #1 (Practice): Add an UpdateProfileRequest request body type
// Handlers that update a user (PUT /api/users/me) need to parse a body.
// Create a separate type for the update request (not the full User model):
//   type UpdateProfileRequest struct {
//     DisplayName *string `json:"displayName"`
//     ZipCode     *string `json:"zipCode"`
//   }
// Why not just use User for the request body?
// User has id, email, timestamps — field you don't want clients to set directly.
// Separate request models = explicit about what CAN be changed.

// TODO #2 (Practice): Add a UserStats type for a profile stats endpoint
// Add a GET /api/me/stats endpoint that returns:
//   type UserStats struct {
//     WatchlistCount int     `json:"watchlistCount"`
//     InventoryCount int     `json:"inventoryCount"`
//     AlertsUnread   int     `json:"alertsUnread"`
//     PortfolioValue float64 `json:"portfolioValue"` // sum of inventory current_value
//   }
// This powers a "dashboard summary" widget on the frontend.
// You'll need to add a method to a service — think about which service is responsible.
