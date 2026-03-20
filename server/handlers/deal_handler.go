// handlers/deal_handler.go — HTTP only
package handlers

import (
	"net/http"

	"pokemontool/pkg"
	"pokemontool/services"
)

type DealHandler struct{ svc *services.DealService }

func NewDealHandler(svc *services.DealService) *DealHandler { return &DealHandler{svc: svc} }

func (h *DealHandler) Today(w http.ResponseWriter, r *http.Request) {
	deals, err := h.svc.GetToday(r.Context())
	if err != nil {
		pkg.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(deals) == 0 {
		pkg.JSON(w, http.StatusOK, map[string]interface{}{
			"deals": []interface{}{}, "message": "Deals computed at 6 AM daily.",
		})
		return
	}
	pkg.JSON(w, http.StatusOK, map[string]interface{}{"deals": deals})
}
