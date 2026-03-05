// handlers/card_handler.go — HTTP only: parse → call CardService → respond
package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"pokemontool/pkg"
	"pokemontool/services"
)

type CardHandler struct{ svc *services.CardService }

func NewCardHandler(svc *services.CardService) *CardHandler { return &CardHandler{svc: svc} }

func (h *CardHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if len(q) < 2 {
		pkg.Error(w, http.StatusBadRequest, "query must be at least 2 characters")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 { limit = 20 }

	cards, source, err := h.svc.Search(r.Context(), q, limit)
	if err != nil {
		pkg.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	pkg.JSON(w, http.StatusOK, map[string]interface{}{"cards": cards, "source": source})
}

func (h *CardHandler) Trending(w http.ResponseWriter, r *http.Request) {
	rising, falling, source, err := h.svc.GetTrending(r.Context())
	if err != nil {
		pkg.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	pkg.JSON(w, http.StatusOK, map[string]interface{}{
		"rising": rising, "falling": falling, "source": source,
	})
}

func (h *CardHandler) PriceHistory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	card, history, err := h.svc.GetPriceHistory(r.Context(), id)
	if err != nil {
		pkg.Error(w, http.StatusNotFound, "card not found")
		return
	}
	pkg.JSON(w, http.StatusOK, map[string]interface{}{
		"card":         card,
		"priceHistory": history,
	})
}
