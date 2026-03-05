// handlers/show_handler.go — HTTP only
package handlers

import (
	"net/http"

	"pokemontool/pkg"
	"pokemontool/services"
)

type ShowHandler struct{ svc *services.ShowService }

func NewShowHandler(svc *services.ShowService) *ShowHandler { return &ShowHandler{svc: svc} }

func (h *ShowHandler) Upcoming(w http.ResponseWriter, r *http.Request) {
	shows, _, err := h.svc.GetUpcoming(r.Context())
	if err != nil {
		pkg.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	pkg.JSON(w, http.StatusOK, map[string]interface{}{"shows": shows})
}
