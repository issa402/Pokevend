// handlers/inventory_handler.go — HTTP only: multipart + CSV export
package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"pokemontool/middleware"
	"pokemontool/models"
	"pokemontool/pkg"
	"pokemontool/services"
)

type InventoryHandler struct{ svc *services.InventoryService }

func NewInventoryHandler(svc *services.InventoryService) *InventoryHandler {
	return &InventoryHandler{svc: svc}
}

func (h *InventoryHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	items, err := h.svc.List(r.Context(), user.ID)
	if err != nil {
		pkg.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	pkg.JSON(w, http.StatusOK, map[string]interface{}{"inventory": items, "total": len(items)})
}

func (h *InventoryHandler) Add(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	var body struct {
		CardName        string   `json:"cardName"`
		SetName         string   `json:"setName"`
		CardNumber      string   `json:"cardNumber"`
		ExternalCardID  string   `json:"externalCardId"`
		Rarity          string   `json:"rarity"`
		ImageURL        string   `json:"imageUrl"`
		MarketUpdatedAt string   `json:"marketUpdatedAt"`
		PriceSource     string   `json:"priceSource"`
		Condition       string   `json:"condition"`
		Quantity        int      `json:"quantity"`
		PurchasePrice   *float64 `json:"purchasePrice"`
		CurrentValue    *float64 `json:"currentValue"`
		Notes           string   `json:"notes"`
		AssetType       string   `json:"assetType"`
		Grader          string   `json:"grader"`
		Grade           string   `json:"grade"`
		SlabTier        string   `json:"slabTier"`
		CertNumber      string   `json:"certNumber"`
		TargetSalePrice *float64 `json:"targetSalePrice"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.CardName == "" {
		pkg.Error(w, http.StatusBadRequest, "cardName is required")
		return
	}
	item := models.InventoryItem{
		UserID: user.ID, CardName: body.CardName, Quantity: body.Quantity,
		PurchasePrice: body.PurchasePrice, CurrentValue: body.CurrentValue,
	}
	if body.SetName != "" {
		item.SetName = &body.SetName
	}
	if body.CardNumber != "" {
		item.CardNumber = &body.CardNumber
	}
	if body.ExternalCardID != "" {
		item.ExternalCardID = &body.ExternalCardID
	}
	if body.Rarity != "" {
		item.Rarity = &body.Rarity
	}
	if body.ImageURL != "" {
		item.ImageURL = &body.ImageURL
	}
	if body.MarketUpdatedAt != "" {
		item.MarketUpdatedAt = &body.MarketUpdatedAt
	}
	if body.PriceSource != "" {
		item.PriceSource = &body.PriceSource
	}
	if body.Condition != "" {
		item.Condition = &body.Condition
	}
	if body.Notes != "" {
		item.Notes = &body.Notes
	}
	if body.AssetType != "" {
		item.AssetType = &body.AssetType
	}
	if body.Grader != "" {
		item.Grader = &body.Grader
	}
	if body.Grade != "" {
		item.Grade = &body.Grade
	}
	if body.SlabTier != "" {
		item.SlabTier = &body.SlabTier
	}
	if body.CertNumber != "" {
		item.CertNumber = &body.CertNumber
	}
	item.TargetSalePrice = body.TargetSalePrice

	id, err := h.svc.Add(r.Context(), item)
	if err != nil {
		pkg.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	pkg.JSON(w, http.StatusCreated, map[string]string{"id": id, "message": "added"})
}

func (h *InventoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	h.svc.Delete(r.Context(), chi.URLParam(r, "id"), user.ID)
	pkg.JSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

func (h *InventoryHandler) MarkReadyForStore(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	var body models.StoreListingInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		pkg.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	item, err := h.svc.MarkReadyForStore(r.Context(), chi.URLParam(r, "id"), user.ID, body)
	if err != nil {
		pkg.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	pkg.JSON(w, http.StatusOK, map[string]interface{}{"message": "ready_for_store", "inventoryItem": item})
}

func (h *InventoryHandler) Import(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	r.ParseMultipartForm(10 << 20)
	file, _, err := r.FormFile("file")
	if err != nil {
		pkg.Error(w, http.StatusBadRequest, "no file uploaded")
		return
	}
	defer file.Close()
	count, err := h.svc.ImportCSV(r.Context(), user.ID, file)
	if err != nil {
		pkg.Error(w, http.StatusBadRequest, "invalid CSV file")
		return
	}
	pkg.JSON(w, http.StatusOK, map[string]interface{}{"message": fmt.Sprintf("imported %d cards", count)})
}

func (h *InventoryHandler) Export(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	items, _ := h.svc.List(r.Context(), user.ID)
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=inventory.csv")
	cw := csv.NewWriter(w)
	cw.Write([]string{"card_name", "set_name", "card_number", "condition", "quantity", "purchase_price"})
	for _, item := range items {
		cw.Write([]string{
			item.CardName, strPtrVal(item.SetName), strPtrVal(item.CardNumber),
			strPtrVal(item.Condition), fmt.Sprintf("%d", item.Quantity),
			floatPtrStr(item.PurchasePrice),
		})
	}
	cw.Flush()
}

func strPtrVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
func floatPtrStr(f *float64) string {
	if f == nil {
		return ""
	}
	return fmt.Sprintf("%.2f", *f)
}
