package handlers

import (
	"net/http"
	"pokemontool/pkg"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type HealthHandler struct {
	db  *pgxpool.Pool
	rdb *redis.Client
}

func NewHealthHandler(db *pgxpool.Pool, rdb *redis.Client) *HealthHandler {
	return &HealthHandler{db: db, rdb: rdb}
}

// HealthHandler checks if our dependencies (DB and Redis) are actually working.
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {

	if err := h.db.Ping(r.Context()); err != nil {
		pkg.Error(w, http.StatusServiceUnavailable, "database down")
		return
	}

	// 2. Check Redis (Sends PING, expects PONG)
	if err := h.rdb.Ping(r.Context()).Err(); err != nil {
		pkg.Error(w, http.StatusServiceUnavailable, "redis down")
		return
	}

	// 3. Both are good!
	pkg.JSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "pokemontool-go",
	})

}
