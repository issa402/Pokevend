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

type SlabOpportunityHandler struct {
	svc *services.SlabOpportunityService
}

func NewSlabOpportunityHandler(svc *services.SlabOpportunityService) *SlabOpportunityHandler {
	return &SlabOpportunityHandler{svc: svc}
}

func (h *SlabOpportunityHandler) List(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	minMargin := -1000000.0
	if rawMinMargin := query.Get("minMarginPct"); rawMinMargin != "" {
		minMargin, _ = strconv.ParseFloat(rawMinMargin, 64)
	}
	limit, _ := strconv.Atoi(query.Get("limit"))
	filters := models.SlabOpportunityFilters{
		Decision:     query.Get("decision"),
		Grader:       query.Get("grader"),
		Grade:        query.Get("grade"),
		SlabTier:     query.Get("slabTier"),
		Marketplace:  query.Get("marketplace"),
		CardName:     query.Get("q"),
		MinMarginPct: minMargin,
		Signal:       query.Get("signal"),
		Limit:        limit,
	}

	opportunities, err := h.svc.List(r.Context(), filters)
	if err != nil {
		pkg.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	pkg.JSON(w, http.StatusOK, map[string]interface{}{"opportunities": opportunities, "total": len(opportunities)})
}

func (h *SlabOpportunityHandler) RefreshLive(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.RefreshLive(r.Context())
	if err != nil {
		pkg.Error(w, http.StatusBadGateway, err.Error())
		return
	}
	pkg.JSON(w, http.StatusOK, result)
}

func (h *SlabOpportunityHandler) Approve(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	inventoryID, err := h.svc.Approve(r.Context(), chi.URLParam(r, "id"), user.ID)
	if err != nil {
		pkg.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	pkg.JSON(w, http.StatusOK, map[string]string{"message": "approved", "inventoryId": inventoryID})
}

func (h *SlabOpportunityHandler) Reject(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Reject(r.Context(), chi.URLParam(r, "id")); err != nil {
		pkg.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	pkg.JSON(w, http.StatusOK, map[string]string{"message": "rejected"})
}
