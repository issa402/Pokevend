// handlers/watchlist_handler.go — HTTP only
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"pokemontool/middleware"
	"pokemontool/models"
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
		CardName           string   `json:"cardName"`
		SetName            string   `json:"setName"`
		ExternalCardID     string   `json:"externalCardId"`
		CardNumber         string   `json:"cardNumber"`
		Rarity             string   `json:"rarity"`
		ImageURL           string   `json:"imageUrl"`
		MarketPrice        *float64 `json:"marketPrice"`
		MarketUpdatedAt    string   `json:"marketUpdatedAt"`
		TargetDiscountPct  *float64 `json:"targetDiscountPct"`
		PriceSource        string   `json:"priceSource"`
		TargetBuyPrice     *float64 `json:"targetBuyPrice"`
		TargetSellPrice    *float64 `json:"targetSellPrice"`
		Notes              string   `json:"notes"`
		AssetType          string   `json:"assetType"`
		Grader             string   `json:"grader"`
		Grade              string   `json:"grade"`
		SlabTier           string   `json:"slabTier"`
		LanguagePreference string   `json:"languagePreference"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		pkg.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	item, err := h.svc.Add(r.Context(), models.WatchlistItem{
		UserID:             user.ID,
		CardName:           body.CardName,
		SetName:            stringPtr(body.SetName),
		ExternalCardID:     stringPtr(body.ExternalCardID),
		CardNumber:         stringPtr(body.CardNumber),
		Rarity:             stringPtr(body.Rarity),
		ImageURL:           stringPtr(body.ImageURL),
		MarketPrice:        body.MarketPrice,
		MarketUpdatedAt:    stringPtr(body.MarketUpdatedAt),
		TargetDiscountPct:  body.TargetDiscountPct,
		PriceSource:        stringPtr(body.PriceSource),
		TargetBuyPrice:     body.TargetBuyPrice,
		TargetSellPrice:    body.TargetSellPrice,
		Notes:              stringPtr(body.Notes),
		AssetType:          body.AssetType,
		Grader:             stringPtr(body.Grader),
		Grade:              stringPtr(body.Grade),
		SlabTier:           stringPtr(body.SlabTier),
		LanguagePreference: body.LanguagePreference,
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, services.ErrCardNameRequired) {
			status = http.StatusBadRequest
		}
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

func (h *WatchlistHandler) GetWatchedNames(w http.ResponseWriter, r *http.Request) {

	names, err := h.svc.GetDistinctCardNames(r.Context())
	if err != nil {
		pkg.Error(w, http.StatusInternalServerError, "Failed to get distinct names")
		return
	}
	pkg.JSON(w, http.StatusOK, names)
}

func (h *WatchlistHandler) GetScanTargets(w http.ResponseWriter, r *http.Request) {
	targets, err := h.svc.GetScanTargets(r.Context())
	if err != nil {
		pkg.Error(w, http.StatusInternalServerError, "Failed to get watchlist scan targets")
		return
	}
	pkg.JSON(w, http.StatusOK, targets)
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
