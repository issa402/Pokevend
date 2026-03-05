// handlers/apikey_handler.go — HTTP only: API key management
package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pokemontool/config"
	"pokemontool/middleware"
	"pokemontool/pkg"
)

// APIKeyHandler manages encrypted third-party API keys per user.
// Keys are encrypted with AES-256-GCM before storage and never returned after saving.
type APIKeyHandler struct {
	db  *pgxpool.Pool
	cfg *config.Config
}

func NewAPIKeyHandler(db *pgxpool.Pool, cfg *config.Config) *APIKeyHandler {
	return &APIKeyHandler{db: db, cfg: cfg}
}

func (h *APIKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	rows, err := h.db.Query(context.Background(),
		`SELECT platform, created_at FROM api_keys WHERE user_id=$1`, user.ID,
	)
	if err != nil {
		pkg.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	keys := []map[string]interface{}{}
	for rows.Next() {
		var platform string
		var createdAt interface{}
		rows.Scan(&platform, &createdAt)
		keys = append(keys, map[string]interface{}{"platform": platform, "created_at": createdAt})
	}
	pkg.JSON(w, http.StatusOK, map[string]interface{}{"keys": keys})
}

func (h *APIKeyHandler) Save(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	var body struct {
		Platform string `json:"platform"`
		KeyValue string `json:"keyValue"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Platform == "" || body.KeyValue == "" {
		pkg.Error(w, http.StatusBadRequest, "platform and keyValue required")
		return
	}
	encrypted, err := pkg.Encrypt(body.KeyValue, h.cfg.EncryptionKey)
	if err != nil {
		pkg.Error(w, http.StatusInternalServerError, "encryption failed")
		return
	}
	_, err = h.db.Exec(context.Background(),
		`INSERT INTO api_keys (user_id,platform,encrypted_key)
		 VALUES ($1,$2,$3)
		 ON CONFLICT (user_id,platform) DO UPDATE SET encrypted_key=$3`,
		user.ID, body.Platform, encrypted,
	)
	if err != nil {
		pkg.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	pkg.JSON(w, http.StatusOK, map[string]string{"message": "API key saved and encrypted"})
}

func (h *APIKeyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	h.db.Exec(context.Background(),
		`DELETE FROM api_keys WHERE user_id=$1 AND platform=$2`,
		user.ID, chi.URLParam(r, "platform"),
	)
	pkg.JSON(w, http.StatusOK, map[string]string{"message": "key removed"})
}
