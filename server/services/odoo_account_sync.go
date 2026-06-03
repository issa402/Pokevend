package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"pokemontool/models"
)

type CommerceAccountSyncer interface {
	SyncAccount(ctx context.Context, email, password, displayName string) error
}

type CommerceProductSyncer interface {
	SyncInventoryProduct(ctx context.Context, item models.InventoryItem) error
}

type noopCommerceAccountSyncer struct{}

func (noopCommerceAccountSyncer) SyncAccount(ctx context.Context, email, password, displayName string) error {
	return nil
}

func (noopCommerceAccountSyncer) SyncInventoryProduct(ctx context.Context, item models.InventoryItem) error {
	return nil
}

type OdooAccountSyncer struct {
	baseURL  string
	db       string
	username string
	password string
	client   *http.Client
}

func NewOdooAccountSyncer(baseURL, db, username, password string) CommerceAccountSyncer {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" || db == "" || username == "" || password == "" {
		return noopCommerceAccountSyncer{}
	}
	jar, _ := cookiejar.New(nil)
	return &OdooAccountSyncer{
		baseURL:  baseURL,
		db:       db,
		username: username,
		password: password,
		client:   &http.Client{Timeout: 20 * time.Second, Jar: jar},
	}
}

func (s *OdooAccountSyncer) SyncAccount(ctx context.Context, email, password, displayName string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || password == "" {
		return nil
	}
	name := strings.TrimSpace(displayName)
	if name == "" {
		name = email
	}
	if err := s.authenticate(ctx); err != nil {
		return err
	}
	ids, err := s.searchUser(ctx, email)
	if err != nil {
		return err
	}
	values := map[string]any{"name": name, "login": email, "email": email, "password": password}
	if len(ids) > 0 {
		_, err = s.callKW(ctx, "res.users", "write", []any{[]int{ids[0]}, values}, map[string]any{})
		return err
	}
	_, err = s.callKW(ctx, "res.users", "create", []any{values}, map[string]any{})
	return err
}

func (s *OdooAccountSyncer) authenticate(ctx context.Context) error {
	_, err := s.postJSON(ctx, "/web/session/authenticate", map[string]any{
		"jsonrpc": "2.0",
		"params":  map[string]any{"db": s.db, "login": s.username, "password": s.password},
	})
	return err
}

func (s *OdooAccountSyncer) searchUser(ctx context.Context, email string) ([]int, error) {
	result, err := s.callKW(ctx, "res.users", "search", []any{[]any{[]any{"login", "=", email}}}, map[string]any{"limit": 1})
	if err != nil {
		return nil, err
	}
	items, ok := result.([]any)
	if !ok {
		return nil, nil
	}
	ids := []int{}
	for _, item := range items {
		switch value := item.(type) {
		case float64:
			ids = append(ids, int(value))
		case int:
			ids = append(ids, value)
		}
	}
	return ids, nil
}

func (s *OdooAccountSyncer) callKW(ctx context.Context, model, method string, args []any, kwargs map[string]any) (any, error) {
	return s.postJSON(ctx, "/web/dataset/call_kw/"+model+"/"+method, map[string]any{
		"jsonrpc": "2.0",
		"params":  map[string]any{"model": model, "method": method, "args": args, "kwargs": kwargs},
	})
}

func (s *OdooAccountSyncer) postJSON(ctx context.Context, path string, payload map[string]any) (any, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var decoded struct {
		Result any `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("odoo returned status %d", resp.StatusCode)
	}
	if decoded.Error != nil {
		return nil, fmt.Errorf("odoo error: %s", decoded.Error.Message)
	}
	return decoded.Result, nil
}

func (s *OdooAccountSyncer) SyncInventoryProduct(ctx context.Context, item models.InventoryItem) error {
	if item.ID == "" || item.CardName == "" {
		return nil
	}
	if err := s.authenticate(ctx); err != nil {
		return err
	}
	ids, err := s.searchProduct(ctx, item.ID)
	if err != nil {
		return err
	}
	values := s.productValues(item)
	if len(ids) > 0 {
		_, err = s.callKW(ctx, "product.template", "write", []any{[]int{ids[0]}, values}, map[string]any{})
		return err
	}
	_, err = s.callKW(ctx, "product.template", "create", []any{values}, map[string]any{})
	return err
}

func (s *OdooAccountSyncer) searchProduct(ctx context.Context, inventoryID string) ([]int, error) {
	result, err := s.callKW(ctx, "product.template", "search", []any{[]any{[]any{"pokemon_inventory_id", "=", inventoryID}}}, map[string]any{"limit": 1})
	if err != nil {
		return nil, err
	}
	items, ok := result.([]any)
	if !ok {
		return nil, nil
	}
	ids := []int{}
	for _, item := range items {
		if value, ok := item.(float64); ok {
			ids = append(ids, int(value))
		}
	}
	return ids, nil
}

func (s *OdooAccountSyncer) productValues(item models.InventoryItem) map[string]any {
	price := firstFloat(item.StorePrice, item.TargetSalePrice, item.CurrentValue)
	values := map[string]any{
		"name":                      productName(item),
		"type":                      "consu",
		"sale_ok":                   true,
		"purchase_ok":               true,
		"is_published":              true,
		"list_price":                price,
		"standard_price":            floatPtrValue(item.PurchasePrice),
		"default_code":              defaultCode(item),
		"description_sale":          productDescription(item),
		"pokemon_inventory_id":      item.ID,
		"pokemon_external_card_id":  stringPtrValue(item.ExternalCardID),
		"pokemon_set_name":          stringPtrValue(item.SetName),
		"pokemon_card_number":       stringPtrValue(item.CardNumber),
		"pokemon_rarity":            stringPtrValue(item.Rarity),
		"pokemon_image_url":         stringPtrValue(item.ImageURL),
		"pokemon_condition":         conditionValue(item.Condition),
		"pokemon_asset_type":        defaultString(item.AssetType, "RAW"),
		"pokemon_grader":            stringPtrValue(item.Grader),
		"pokemon_grade":             stringPtrValue(item.Grade),
		"pokemon_cert_number":       stringPtrValue(item.CertNumber),
		"pokemon_acquisition_cost":  floatPtrValue(item.PurchasePrice),
		"pokemon_market_value":      floatPtrValue(item.CurrentValue),
		"pokemon_target_margin_pct": 30.0,
		"pokemon_price_source":      stringPtrValue(item.PriceSource),
		"pokemon_market_updated_at": stringPtrValue(item.MarketUpdatedAt),
		"pokemon_sync_source":       "pokemontool",
	}
	clean := map[string]any{}
	for key, value := range values {
		if value != nil && value != "" {
			clean[key] = value
		}
	}
	return clean
}

func productName(item models.InventoryItem) string {
	parts := []string{item.CardName}
	if value := stringPtrValue(item.SetName); value != "" {
		parts = append(parts, value)
	}
	if value := stringPtrValue(item.CardNumber); value != "" {
		parts = append(parts, "#"+value)
	}
	if value := stringPtrValue(item.Condition); value != "" {
		parts = append(parts, value)
	}
	return strings.Join(parts, " - ")
}

func productDescription(item models.InventoryItem) string {
	parts := []string{"Pokemon card inventory synced from PokemonTool."}
	for _, line := range []string{
		labelValue("Set", item.SetName), labelValue("Card number", item.CardNumber), labelValue("Condition", item.Condition),
		labelValue("External card ID", item.ExternalCardID), labelValue("Asset type", item.AssetType), labelValue("Grader", item.Grader),
		labelValue("Grade", item.Grade), labelValue("Certification", item.CertNumber), labelValue("Store notes", item.StoreListingNotes),
	} {
		if line != "" {
			parts = append(parts, line)
		}
	}
	return strings.Join(parts, "\n")
}

func labelValue(label string, value *string) string {
	if value == nil || *value == "" {
		return ""
	}
	return label + ": " + *value
}
func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func floatPtrValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}
func firstFloat(values ...*float64) float64 {
	for _, value := range values {
		if value != nil && *value > 0 {
			return *value
		}
	}
	return 0
}
func defaultString(value *string, fallback string) string {
	if value == nil || *value == "" {
		return fallback
	}
	return *value
}
func defaultCode(item models.InventoryItem) string {
	if v := stringPtrValue(item.ExternalCardID); v != "" {
		return v
	}
	if len(item.ID) > 12 {
		return item.ID[:12]
	}
	return item.ID
}
func conditionValue(value *string) string {
	normalized := strings.ToUpper(strings.ReplaceAll(stringPtrValue(value), " ", "_"))
	switch normalized {
	case "NM", "LP", "MP", "HP", "DAMAGED", "SEALED", "GRADED":
		return normalized
	default:
		return ""
	}
}
