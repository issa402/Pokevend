// ============================================================
// FILE: server/handlers/alert_handler.go
// TYPE: Handler Layer — Alert HTTP Endpoints
//
// ROUTES SERVED:
//
//	GET    /api/alerts            → list user's alerts (paginated)
//	PUT    /api/alerts/read-all  → mark all read
//	PUT    /api/alerts/{id}/read → mark one read
//	DELETE /api/alerts/{id}      → delete one alert
//
// SECURITY PATTERN: USER CONTEXT INJECTION
// All endpoints here require authentication (under RequireAuth middleware).
// The middleware already validated the JWT and put user info in context.
// We call middleware.GetUser(r) to retrieve it — no re-validation needed.
//
// WHY NOT PASS USER ID IN THE REQUEST BODY?
// Client-side IDs are UNTRUSTED. A client could send userID="someone-elses-uuid".
// The authenticated user ID comes from the JWT (server-signed) — cannot be forged.
// FAANG RULE: User identity always comes from the server-side auth context,
// never from the request body or query params.
// ============================================================
package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"pokemontool/middleware"
	"pokemontool/models"
	"pokemontool/pkg"
	"pokemontool/services"
)

// AlertHandler handles all alert HTTP requests.
type AlertHandler struct{ svc *services.AlertService }

// NewAlertHandler injects the AlertService.

func NewAlertHandler(svc *services.AlertService) *AlertHandler {
	return &AlertHandler{svc: svc}
}

// List handles GET /api/alerts
// Returns alerts for the authenticated user, newest first.
// Query param: limit (optional, default 50, max 200)
func (h *AlertHandler) List(w http.ResponseWriter, r *http.Request) {
	// middleware.GetUser extracts UserClaims from request context.
	// This is set by RequireAuth middleware (validates and injects JWT claims).
	user := middleware.GetUser(r)

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	alerts, err := h.svc.List(r.Context(), user.ID, limit)
	if err != nil {
		pkg.Error(w, http.StatusInternalServerError, "failed to load alerts")
		return
	}
	if alerts == nil {
		alerts = []models.Alert{}
	}
	unreadCount := 0
	for _, alert := range alerts {
		if !alert.IsRead {
			unreadCount++
		}
	}
	pkg.JSON(w, http.StatusOK, map[string]interface{}{
		"alerts":      alerts,
		"unreadCount": unreadCount,
	})
}

// MarkRead handles PUT /api/alerts/{id}/read
// Marks ONE alert as read. Includes user_id check (IDOR prevention).
func (h *AlertHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	id := chi.URLParam(r, "id") // {id} from URL path

	if err := h.svc.MarkRead(r.Context(), id, user.ID); err != nil {
		pkg.Error(w, http.StatusNotFound, "alert not found")
		return
	}
	pkg.JSON(w, http.StatusOK, map[string]string{"status": "read"})
}

// MarkAllRead handles PUT /api/alerts/read-all
// Marks ALL of a user's alerts as read in one operation.
func (h *AlertHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if err := h.svc.MarkAllRead(r.Context(), user.ID); err != nil {
		pkg.Error(w, http.StatusInternalServerError, "failed to update alerts")
		return
	}
	pkg.JSON(w, http.StatusOK, map[string]string{"status": "all read"})
}

// Delete handles DELETE /api/alerts/{id}
// Permanently removes one alert. user_id check prevents deleting others' alerts.
func (h *AlertHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	id := chi.URLParam(r, "id")

	if err := h.svc.Delete(r.Context(), id, user.ID); err != nil {
		pkg.Error(w, http.StatusNotFound, "alert not found")
		return
	}
	// HTTP 204 No Content = success with no response body
	// The frontend doesn't need any data back — just confirmation it's gone
	w.WriteHeader(http.StatusNoContent)
}

// ============================================================
// PRACTICE TASK #5 — Add UnreadCount Handler
// See: practice_tasks.md → Task 5
// ============================================================
//
// After completing Tasks 3 and 4, add this method to AlertHandler.
func (h *AlertHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	count, err := h.svc.GetUnreadCount(r.Context(), user.ID)
	if err != nil {
		pkg.Error(w, http.StatusInternalServerError, "failed to retrive count")
		return
	}
	pkg.JSON(w, http.StatusOK, map[string]int{"unread": count})
}

// Signature:
//   func (h *AlertHandler) UnreadCount(w http.ResponseWriter, r *http.Request)
//
// What it should do:
//   1. user := middleware.GetUser(r)             → get authenticated user
//   2. count, err := h.svc.GetUnreadCount(...)   → call service
//   3. If err → pkg.Error(w, 500, "failed to get unread count")
//   4. pkg.JSON(w, 200, map[string]int{"unread": count})
//
// Expected JSON response: {"unread": 5}
// This powers the navbar badge on the frontend (shows red dot with number).
//
// Write your implementation here (delete this comment block):

// ── SOLUTION (peek only when stuck) ──────────────────────────
// func (h *AlertHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
// 	user := middleware.GetUser(r)
// 	count, err := h.svc.GetUnreadCount(r.Context(), user.ID)
// 	if err != nil {
// 		pkg.Error(w, http.StatusInternalServerError, "failed to get unread count")
// 		return
// 	}
// 	pkg.JSON(w, http.StatusOK, map[string]int{"unread": count})
// }
//
// Then go to routes/routes.go and add (Task 6):
//   r.Get("/alerts/unread-count", alerts.UnreadCount)
// ⚠️  Add BEFORE r.Get("/alerts/{id}/read", ...) — chi matches top-to-bottom!
//     "unread-count" must come before {id} or chi will treat it as an ID.

// ============================================================
// PRACTICE (bonus): Add alert filtering by type
// Let the frontend filter: GET /api/alerts?type=PRICE_DROP
// In this handler: alertType := r.URL.Query().Get("type")
// Pass it to service → store adds it to the SQL WHERE clause
// When empty: return all alerts. When set: filter by alert_type.
// ============================================================
