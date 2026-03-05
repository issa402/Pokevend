// handlers/watchlist_handler.go — HTTP only
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"pokemontool/middleware"
	"pokemontool/pkg"
	"pokemontool/services"
)

type WatchlistHandler struct{ svc *services.WatchlistService }

func NewWatchlistHandler(svc *services.WatchlistService) *WatchlistHandler {
	return &WatchlistHandler{svc: svc}
}

func (h *WatchlistHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	items, err := h.svc.List(r.Context(), user.ID)
	if err != nil {
		pkg.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	pkg.JSON(w, http.StatusOK, map[string]interface{}{"watchlist": items})
}

func (h *WatchlistHandler) Add(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	var body struct {
		CardName        string   `json:"cardName"`
		SetName         string   `json:"setName"`
		TargetBuyPrice  *float64 `json:"targetBuyPrice"`
		TargetSellPrice *float64 `json:"targetSellPrice"`
		Notes           string   `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		pkg.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	item, err := h.svc.Add(r.Context(), user.ID, body.CardName, body.SetName, body.TargetBuyPrice, body.TargetSellPrice, body.Notes)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, services.ErrCardNameRequired) { status = http.StatusBadRequest }
		pkg.Error(w, status, err.Error())
		return
	}
	pkg.JSON(w, http.StatusCreated, map[string]interface{}{"item": item})
}

func (h *WatchlistHandler) Remove(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	h.svc.Remove(r.Context(), chi.URLParam(r, "id"), user.ID)
	pkg.JSON(w, http.StatusOK, map[string]string{"message": "removed"})
}
