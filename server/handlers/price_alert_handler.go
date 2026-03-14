package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"pokemontool/middleware"
	"pokemontool/pkg"
	"pokemontool/services"
)

type PriceAlertStore struct{
	repo store.PriceAlertStore
}

func NewPriceAlertHandler(repo store.PriceAlertStore) *PriceAlertHandler {
	return &PriceAlertHandler{repo:repo}
}

func (h *PriceAlertHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	
	var input struct{
		CardName string `json:"cardName`
		Threshold float64 `json:"threshold"`
		Direction string `json:"direction"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		pkg.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if input.CardName == "" || input.Threshold <= 0 {
		pkg.Error(w, http.StatusBadRequest, "missing card name or invalid price")
		return
	}

	err := h.repo.Create(r.Context(), user.ID, input.CardName, input.Threshold, input.Direction)
	if err != nil {
		log.Printf("[handler] create error: %v", err)
		pkg.Error(w, http.StatusInternalServerError, "failed to save price alert")
		return
	}

	// 4. Success Response
	pkg.JSON(w, http.StatusCreated, map[string]string{"status": "alert created"})
}
	


func (h *PriceAlertHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	
	alerts, err := h.repo.ListByUser(r.Context(), user.ID)
	if err != nil {
		pkg.Error(w, http.StatusInternalServerError, "db error")
		return
	}
	pkg.JSON(w, http.StatusOK, alerts)
}

// DELETE /api/price-alerts/{id}
func (h *PriceAlertHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	id := chi.URLParam(r, "id")

	if err := h.repo.Delete(r.Context(), id, user.ID); err != nil {
		pkg.Error(w, http.StatusNotFound, "not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}