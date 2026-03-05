// handlers/alert_handler.go — HTTP only
package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"pokemontool/middleware"
	"pokemontool/pkg"
	"pokemontool/services"
)

type AlertHandler struct{ svc *services.AlertService }

func NewAlertHandler(svc *services.AlertService) *AlertHandler { return &AlertHandler{svc: svc} }

func (h *AlertHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	alerts, unread, err := h.svc.List(r.Context(), user.ID)
	if err != nil {
		pkg.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	pkg.JSON(w, http.StatusOK, map[string]interface{}{"alerts": alerts, "unreadCount": unread})
}

func (h *AlertHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	h.svc.MarkRead(r.Context(), chi.URLParam(r, "id"), user.ID)
	pkg.JSON(w, http.StatusOK, map[string]string{"message": "marked read"})
}

func (h *AlertHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	h.svc.MarkAllRead(r.Context(), user.ID)
	pkg.JSON(w, http.StatusOK, map[string]string{"message": "all marked read"})
}

func (h *AlertHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	h.svc.Delete(r.Context(), chi.URLParam(r, "id"), user.ID)
	pkg.JSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}
