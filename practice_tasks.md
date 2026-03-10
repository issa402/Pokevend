# Hands-On Practice Tasks
## 10 Full-Stack Features — Each Touches Python + Go + SQL + Bash

Each task is ONE real feature. It has a piece in every language.
Do them in order — later tasks build on earlier ones.

---

## The Format

Each task shows you:
- **SQL** → schema or query you need
- **Go** → the backend files to write
- **Python** → the service code to write
- **Bash** → how to run and verify it all works

---

## Task 1 — Unread Alert Count Badge
**What you're building:** A `GET /api/alerts/unread-count` endpoint so the frontend navbar can show a red badge like "5 unread alerts" without fetching all alerts.

### SQL
Add a partial index to make the count query instant, even with millions of rows:
```sql
-- In database/migrations/001_init.sql or a new 002 file
CREATE INDEX IF NOT EXISTS idx_alerts_user_unread
    ON alerts(user_id, is_read)
    WHERE is_read = false;
-- WHERE clause = partial index. Only indexes rows where is_read=false.
-- The COUNT(*) query now only scans unread rows, not the entire table.
```

### Go
Files to touch, in this order:

**`server/store/alert_store.go`**
1. Add to `AlertStore` interface: `GetUnreadCount(ctx context.Context, userID string) (int, error)`
2. Implement: `SELECT COUNT(*) FROM alerts WHERE user_id=$1 AND is_read=false`

**`server/services/alert_service.go`**
3. Add: `func (s *AlertService) GetUnreadCount(ctx context.Context, userID string) (int, error)`
   - Just delegates to `s.alerts.GetUnreadCount(ctx, userID)`

**`server/handlers/alert_handler.go`**
4. Add: `func (h *AlertHandler) UnreadCount(w http.ResponseWriter, r *http.Request)`
   - `middleware.GetUser(r)` → `h.svc.GetUnreadCount(...)` → `pkg.JSON(w, 200, map[string]int{"unread": count})`

**`server/routes/routes.go`**
5. Add (BEFORE the `{id}` routes or chi will treat "unread-count" as an ID):
   ```go
   r.Get("/alerts/unread-count", alerts.UnreadCount)
   ```

### Python
In `services/api-consumer/publisher/rabbitmq_publisher.py`:
Add `publish_batch(queue_name, messages) -> int` — publishes a list of messages, counts successes. This ensures the alerts actually reach the Go worker that writes them to the DB.

### Bash
```bash
# 1. Apply the index (if not already in migration)
docker exec -i pokemontool_postgres psql -U pokemontool_user -d pokemontool \
  -c "CREATE INDEX IF NOT EXISTS idx_alerts_user_unread ON alerts(user_id,is_read) WHERE is_read=false;"

# 2. Log in and grab a token
TOKEN=$(curl -s -X POST http://localhost:3001/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@test.com","password":"yourpassword"}' | python3 -c "import sys,json;print(json.load(sys.stdin)['token'])")

# 3. Hit the new endpoint
curl -s http://localhost:3001/api/alerts/unread-count \
  -H "Authorization: Bearer $TOKEN"
# Expected: {"unread": 0}
```

---

## Task 2 — Price Alert Settings (User Sets Their Own Thresholds)
**What you're building:** Users can say "notify me when Charizard drops below $80." Store their setting. The Go worker checks it every time a listing comes in.

### SQL
```sql
-- database/migrations/002_price_alerts.sql
CREATE TABLE IF NOT EXISTS price_alerts_settings (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    card_name   TEXT NOT NULL,
    threshold   DECIMAL(10,2) NOT NULL,
    direction   TEXT NOT NULL CHECK(direction IN ('BELOW','ABOVE')),
    is_active   BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, card_name, direction)
);
CREATE INDEX IF NOT EXISTS idx_price_alerts_active
    ON price_alerts_settings(card_name, is_active) WHERE is_active = true;
-- Index on card_name: the worker queries by card_name for every listing
```

### Go
**`server/store/alert_store.go`** (or new `server/store/price_alert_store.go`)
- Interface method: `GetActiveAlertsForCard(ctx context.Context, cardName string) ([]PriceAlertSetting, error)`
- SQL: `SELECT user_id, threshold, direction FROM price_alerts_settings WHERE card_name=$1 AND is_active=true`

**`server/worker/notification_worker.go`**
- After receiving a listing message, call `GetActiveAlertsForCard` for that card
- If listing price < threshold (for BELOW alerts) → call `alertStore.Insert(...)` to create the alert
- The alert SSE push to the user happens automatically (existing SSE code handles it)

**`server/handlers/` (new `price_alert_handler.go`)**
- `POST /api/price-alerts` — create a setting
- `GET /api/price-alerts` — list user's settings
- `DELETE /api/price-alerts/{id}` — remove a setting

### Python
In `services/api-consumer/services/ebay_service.py`:
- After publishing a listing to RabbitMQ, log how many were sent so you can verify end-to-end
- Use the `publish_batch` method you wrote in Task 1 instead of publishing one at a time

### Bash
```bash
# Run the migration
docker exec -i pokemontool_postgres psql -U pokemontool_user -d pokemontool \
  < database/migrations/002_price_alerts.sql

# Verify table was created
docker exec -it pokemontool_postgres psql -U pokemontool_user -d pokemontool \
  -c "\d price_alerts_settings"

# Create a test alert setting
curl -s -X POST http://localhost:3001/api/price-alerts \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"cardName": "Charizard", "threshold": 80.00, "direction": "BELOW"}'
```

---

## Task 3 — Card Detail Page with Concurrent Price History
**What you're building:** `GET /api/cards/{id}` returns the card info AND its price history at the same time, using Go's `errgroup` to fetch both concurrently.

### SQL
```sql
-- Query you'll use in card_store.go
SELECT card_id, marketplace, price, recorded_at
FROM price_history
WHERE card_id = $1
ORDER BY recorded_at DESC
LIMIT 30;

-- Add an index if not already there:
CREATE INDEX IF NOT EXISTS idx_price_history_card_recorded
    ON price_history(card_id, recorded_at DESC);
-- Compound index: filters card_id, then sorts recorded_at in one pass
```

### Go
**`server/store/card_store.go`**
- Add interface method: `GetPriceHistory(ctx context.Context, cardID string, limit int) ([]models.PricePoint, error)`
- Implement with the SQL above

**`server/models/card.go`**
- Add struct: `type PricePoint struct { Marketplace string; Price float64; RecordedAt time.Time }`

**`server/handlers/card_handler.go`**
- Add `func (h *CardHandler) Detail(w http.ResponseWriter, r *http.Request)`
- Use `errgroup.WithContext` to run `GetByID` and `GetPriceHistory` at the same time:
  ```go
  g, ctx := errgroup.WithContext(r.Context())
  var card *models.Card
  var history []models.PricePoint
  g.Go(func() error { card, err = ...; return err })
  g.Go(func() error { history, err = ...; return err })
  if err := g.Wait(); err != nil { ... }
  ```
- `go get golang.org/x/sync/errgroup` first

**`server/routes/routes.go`**
- Add: `r.Get("/cards/{id}", cards.Detail)`

### Python
In `services/analytics-engine/repositories/card_repo.py`:
- Implement `insert_price_point(card_id, marketplace, price)` method
- SQL: `INSERT INTO price_history(card_id, marketplace, price) VALUES(%s,%s,%s)`
- Called by `trend_analyzer.py` after fetching current prices — this populates the data the Go endpoint reads

### Bash
```bash
# Install the errgroup package in Go
cd server && go get golang.org/x/sync/errgroup && cd ..

# Test the new endpoint (after starting the server)
curl -s http://localhost:3001/api/cards/base1-4 | python3 -m json.tool
# Expected: {"card": {...}, "history": [...]}

# Verify price_history has data
docker exec -it pokemontool_postgres psql -U pokemontool_user -d pokemontool \
  -c "SELECT card_id, marketplace, price, recorded_at FROM price_history LIMIT 5;"
```

---

## Task 4 — Full Health Check (All 4 Services)
**What you're building:** `GET /health` for Go that checks PostgreSQL + Redis + RabbitMQ, `/health` for FastAPI, and a Bash script that checks them all.

### SQL
```sql
-- The health check queries this to verify DB is alive:
SELECT 1;
-- Sounds trivial, but a real SELECT 1 confirms:
-- 1. TCP connection is alive
-- 2. PostgreSQL process is running
-- 3. Our user has permission to query
-- pgxpool.Ping() does this internally
```

### Go
In `server/main.go` — update the health handler (or create in `server/handlers/health_handler.go`):
```go
func HealthHandler(db *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Check PostgreSQL: db.Ping(r.Context())
        // Check Redis:      rdb.Ping(r.Context()).Err()
        // If either fails → pkg.JSON(w, 503, {"status": "degraded", "error": "..."})
        // Both pass       → pkg.JSON(w, 200, {"status": "ok"})
    }
}
```
Wire in routes: `r.Get("/health", handlers.HealthHandler(db, rdb))`

### Python
In `services/api-consumer/main.py` — add a health route to FastAPI:
```python
@app.get("/health")
async def health():
    # Return {"status": "ok", "service": "api-consumer"}
    # Optionally: check publisher._connection is not None
```

### Bash
This IS the `scripts/health-check.sh` from your practice stub. Write it to:
1. `check_endpoint "Go API" http://localhost:3001/health 200`
2. `check_endpoint "FastAPI" http://localhost:8001/health 200`
3. `docker exec pokemontool_postgres pg_isready`
4. `docker exec pokemontool_redis redis-cli ping | grep -q PONG`
5. Print `✅ All healthy` or fail with exit code 1

```bash
chmod +x scripts/health-check.sh
./scripts/health-check.sh
```

---

## Task 5 — Watchlist Price Scan (End-to-End Alert Flow)
**What you're building:** When the Python scanner runs, it checks each card on each user's watchlist. If a listing comes in under the target price, a real alert is created and pushed via SSE.

### SQL
```sql
-- Query all unique card names being watched (for the Python scanner)
SELECT DISTINCT card_name FROM watchlists;

-- Query watchlists with prices for the Go worker:
SELECT w.user_id, w.card_name, w.target_price
FROM watchlists w
WHERE w.card_name = $1 AND w.target_price IS NOT NULL;
```

### Go
In `server/store/` — add to watchlist store or create `watchlist_store.go`:
- `GetAlertCandidates(ctx, cardName string, maxPrice float64) ([]models.Watchlist, error)`
- SQL: users watching this card AND their target_price > listing price

In `server/worker/notification_worker.go`:
- After receiving a listing, call `watchlistStore.GetAlertCandidates(ctx, listing.CardName, listing.Price)`
- For each match → `alertStore.Insert(...)` → `sseManager.SendToUser(userID, message)`

### Python
In `services/api-consumer/services/ebay_service.py`:
- At startup, read watched cards from the DB (or from a config list)
- Scan those specific card names on eBay (not a hardcoded list)
- In `services/api-consumer/repositories/card_repo.py` — add `get_watched_card_names()`:
  ```python
  def get_watched_card_names(self) -> List[str]:
      # SELECT DISTINCT card_name FROM watchlists
  ```

### Bash
```bash
# Manually insert a watchlist entry to test
docker exec -it pokemontool_postgres psql -U pokemontool_user -d pokemontool -c "
INSERT INTO watchlists(user_id, card_name, target_price)
SELECT id, 'Charizard', 150.00 FROM users LIMIT 1
ON CONFLICT DO NOTHING;"

# Check if your worker processes it:
docker logs pokemontool_go 2>&1 | grep -i "charizard\|alert\|listing" | tail -20

# Verify an alert was created
docker exec -it pokemontool_postgres psql -U pokemontool_user -d pokemontool \
  -c "SELECT user_id, card_name, price, created_at FROM alerts ORDER BY created_at DESC LIMIT 5;"
```

---

## Task 6 — Deal of the Day
**What you're building:** `GET /api/deals` returns today's best deals (listings 20%+ below market price). Python finds them, writes them, Go serves them.

### SQL
```sql
-- deals table has: card_name, market_price, best_price, savings_pct, deal_date
-- UPSERT: one deal per card per marketplace per day
INSERT INTO deals(card_name, market_price, best_price, savings_pct, listing_url, marketplace, deal_date)
VALUES(%s, %s, %s, %s, %s, %s, CURRENT_DATE)
ON CONFLICT(card_name, marketplace, deal_date)
DO UPDATE SET best_price = EXCLUDED.best_price, savings_pct = EXCLUDED.savings_pct;
-- ON CONFLICT + DO UPDATE: if a better deal comes in later today, update it
```

### Go
**`server/store/deal_store.go`** — add:
- `GetTodaysDeals(ctx) ([]models.Deal, error)`
- SQL: `SELECT * FROM deals WHERE deal_date = CURRENT_DATE ORDER BY savings_pct DESC`

**`server/handlers/deal_handler.go`**
- `func (h *DealHandler) ListDeals(w, r)` → calls `h.svc.GetDeals(ctx)` → `pkg.JSON(w, 200, deals)`

**`server/routes/routes.go`**
- Add (public, no auth required): `r.Get("/deals", deals.ListDeals)`

### Python
In `services/analytics-engine/analyzers/deal_finder.py`:
- Implement `find_and_save_deals()`:
  - For each card, compare market price (TCGplayer) vs best eBay listing
  - If savings > 20%: call `card_repo.upsert_deal(card_name, market_price, best_price, savings_pct, ...)`
- Schedule this in `services/analytics-engine/main.py`:
  ```python
  schedule.every(1).hours.do(lambda: deal_finder.find_and_save_deals())
  ```

### Bash
```bash
# Manually insert a test deal to verify the Go endpoint works
docker exec -it pokemontool_postgres psql -U pokemontool_user -d pokemontool -c "
INSERT INTO deals(card_name, market_price, best_price, savings_pct, listing_url, marketplace)
VALUES('Charizard', 100.00, 75.00, 25.00, 'https://ebay.com/test', 'ebay')
ON CONFLICT DO NOTHING;"

# Hit the deals endpoint
curl -s http://localhost:3001/api/deals | python3 -m json.tool

# Check analytics logs when deal_finder runs
docker logs pokemontool_analytics 2>&1 | grep -i "deal" | tail -10
```

---

## Task 7 — Inventory Portfolio Value
**What you're building:** `GET /api/inventory` returns each card you own AND a `totalValue` field — your portfolio's worth — calculated server-side.

### SQL
```sql
-- Update current_value for all inventory rows using latest market price
-- Run this in analytics-engine after each price scan:
UPDATE inventory i
SET current_value = (
    SELECT price_tcgplayer FROM cards WHERE card_id = i.card_id
) * i.quantity,
updated_at = NOW()
WHERE EXISTS (SELECT 1 FROM cards WHERE card_id = i.card_id);

-- The Go endpoint aggregates total:
SELECT i.*, c.name, c.price_tcgplayer
FROM inventory i
JOIN cards c ON i.card_id = c.card_id
WHERE i.user_id = $1;
```

### Go
**`server/store/inventory_store.go`**
- `GetByUser(ctx, userID string) ([]models.InventoryItem, error)`
- SQL: JOIN with cards to get current prices

**`server/services/inventory_service.go`**
- `GetInventory(ctx, userID) (*InventoryResponse, error)` where:
  ```go
  type InventoryResponse struct {
      Items      []models.InventoryItem `json:"items"`
      TotalValue float64                `json:"totalValue"`
  }
  ```
- Compute `TotalValue` by summing `item.CurrentValue` in Go (not SQL)

**`server/routes/routes.go`**
- Add: `r.Get("/inventory", inventory.List)`

### Python
In `services/analytics-engine/analyzers/` — add `portfolio_updater.py`:
- `update_inventory_values()` runs after each price scan
- SQL: the UPDATE above — sets `current_value = price * quantity`
- Schedule in `main.py` to run every hour after price data updates

### Bash
```bash
# Insert a test inventory item
docker exec -it pokemontool_postgres psql -U pokemontool_user -d pokemontool -c "
INSERT INTO inventory(user_id, card_id, quantity, purchase_price)
SELECT u.id, 'base1-4', 2, 50.00
FROM users u LIMIT 1
ON CONFLICT DO NOTHING;"

# Verify current_value was calculated
docker exec -it pokemontool_postgres psql -U pokemontool_user -d pokemontool \
  -c "SELECT card_id, quantity, purchase_price, current_value FROM inventory;"

# Test the API
curl -s http://localhost:3001/api/inventory \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
# Expected: {"items": [...], "totalValue": 179.98}
```

---

## Task 8 — Trending Cards Feed
**What you're building:** `GET /api/cards/trending` returns cards sorted by their trend score, updated by the Python analytics engine.

### SQL
```sql
-- Go endpoint query:
SELECT card_id, name, set_name, trend_label, trending_score,
       price_tcgplayer, price_ebay, image_url
FROM cards
WHERE trending_score != 0
ORDER BY trending_score DESC
LIMIT $1;

-- Python updates it (analytics-engine):
UPDATE cards
SET trend_label = %s, trending_score = %s, updated_at = NOW()
WHERE card_id = %s;
```

### Go
**`server/store/card_store.go`**
- Add to `CardStore` interface: `GetTrending(ctx context.Context, limit int) ([]models.Card, error)`
- SQL: the SELECT above

**`server/handlers/card_handler.go`**
- Add: `func (h *CardHandler) Trending(w, r)`
  - Parse `?limit=` query param (default 20, max 50)
  - Call `h.svc.GetTrending(ctx, limit)`
  - Return `[]models.Card`

**`server/routes/routes.go`**
- Add (public): `r.Get("/cards/trending", cards.Trending)`
- ⚠️ Add BEFORE `/cards/{id}` or chi treats "trending" as a card ID

### Python
In `services/analytics-engine/analyzers/trend_analyzer.py`:
- Implement the full `_analyze_card` method using numpy linear regression:
  ```python
  import numpy as np
  slope = float(np.polyfit(range(len(prices)), prices, 1)[0])
  label = "RISING" if slope > 1 else "FALLING" if slope < -1 else "STABLE"
  score = max(-100, min(100, int(slope * 10)))
  ```
- Then call `card_repo.update_trend(card_id, label, score)`

### Bash
```bash
# Manually set a trending score to test the Go endpoint immediately
docker exec -it pokemontool_postgres psql -U pokemontool_user -d pokemontool -c "
UPDATE cards SET trending_score = 85, trend_label = 'RISING' WHERE name ILIKE '%charizard%';"

# Test the trending endpoint
curl -s "http://localhost:3001/api/cards/trending?limit=5" | python3 -m json.tool

# Once analytics engine is running, verify it updates scores:
docker logs pokemontool_analytics 2>&1 | grep -i "trend\|score" | tail -10
```

---

## Task 9 — API Key Management (Store eBay Keys Per User)
**What you're building:** Users store their own eBay API key, encrypted at rest. Go stores it encrypted. Python can retrieve and use it for requests.

### SQL
```sql
-- api_keys table already exists. Verify:
\d api_keys
-- Should have: id, user_id, platform, encrypted_key, created_at

-- Query to get a user's key for a platform:
SELECT encrypted_key FROM api_keys WHERE user_id=$1 AND platform=$2 LIMIT 1;
```

### Go
**`server/pkg/crypto.go`** — already has `Encrypt` and `Decrypt` functions. Use them.

**`server/store/`** — add `api_key_store.go`:
- Interface: `Save(ctx, userID, platform, encryptedKey string) error`
- Interface: `Get(ctx, userID, platform string) (string, error)`

**`server/handlers/apikey_handler.go`**:
- `POST /api/keys` — encrypt the key with `pkg.Encrypt(rawKey, cfg.EncryptionKey)`, save to DB
- `GET /api/keys` — list platforms the user has keys for (NOT the keys themselves)
- `DELETE /api/keys/{platform}` — remove a key

### Python
In `services/api-consumer/repositories/ebay_repo.py`:
- Instead of reading `EBAY_CLIENT_ID`/`EBAY_CLIENT_SECRET` from env vars,
  add a method `load_credentials_from_db(user_id)` that calls the Go API:
  ```python
  # GET http://server:3001/api/keys   (with user's JWT)
  # Returns which platforms are configured
  ```
- Keep env var fallback for cases where no DB key is set

### Bash
```bash
# Test encryption/decryption works by storing and retrieving a key
curl -s -X POST http://localhost:3001/api/keys \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"platform": "ebay", "apiKey": "your-test-api-key"}'

# Verify it's stored encrypted (not plaintext) in DB
docker exec -it pokemontool_postgres psql -U pokemontool_user -d pokemontool \
  -c "SELECT platform, encrypted_key FROM api_keys LIMIT 1;"
# encrypted_key should look like: a1b2c3d4e5... (hex-encoded ciphertext, NOT the raw key)
```

---

## Task 10 — Graceful Shutdown + Alert Cleanup
**What you're building:** The Go server shuts down cleanly when it gets SIGTERM (for Docker/EC2 deployments). Also adds a daily cleanup job that deletes alerts older than 30 days.

### SQL
```sql
-- Cleanup query (run via daily goroutine):
DELETE FROM alerts WHERE created_at < NOW() - INTERVAL '30 days';
-- Keep old unread alerts so user doesn't miss them:
DELETE FROM alerts WHERE created_at < NOW() - INTERVAL '30 days' AND is_read = true;

-- Check table size before/after cleanup:
SELECT COUNT(*) FROM alerts;
SELECT pg_size_pretty(pg_total_relation_size('alerts'));
```

### Go
In `server/main.go` — add graceful shutdown:
```go
// After http.ListenAndServe would normally block, do this instead:
srv := &http.Server{Addr: ":" + cfg.Port, Handler: r}

// Run server in goroutine
go func() {
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatalf("Server error: %v", err)
    }
}()

// Wait for OS signal
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit   // blocks here until signal received

log.Println("Shutting down server...")
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
srv.Shutdown(ctx)   // waits for in-flight requests to finish (up to 30s)
log.Println("Server stopped")
```

In `server/store/alert_store.go` — add:
- `DeleteOlderThan(ctx, days int) (int64, error)` — returns rows deleted
- Schedule from main.go with `time.NewTicker(24 * time.Hour)`

### Python
In `services/api-consumer/main.py` — update lifespan to cancel the scanner task cleanly:
```python
@asynccontextmanager
async def lifespan(app: FastAPI):
    task = asyncio.create_task(scanner_loop())
    yield
    task.cancel()          # signal the task to stop
    await asyncio.gather(task, return_exceptions=True)  # wait for it to stop
    await publisher.close()
    logger.info("Shutdown complete")
```

### Bash
```bash
# Test graceful shutdown: start server, send SIGTERM, watch logs
docker compose up -d server
sleep 3
docker kill --signal=SIGTERM pokemontool_go
docker logs pokemontool_go 2>&1 | tail -5
# Should see: "Shutting down server..." then "Server stopped"
# NOT: immediate crash or "killed" with no log

# Verify cleanup query works
docker exec -it pokemontool_postgres psql -U pokemontool_user -d pokemontool -c "
DELETE FROM alerts WHERE created_at < NOW() - INTERVAL '30 days' AND is_read=true;
-- Should say DELETE 0 on a fresh DB (no old alerts yet)
"
```

---

## Quick Reference

| Task | Feature | SQL | Go Files | Python Files | Bash |
|------|---------|-----|----------|--------------|------|
| 1 | Unread count badge | Partial index | store → service → handler → route | `publish_batch` | curl + login |
| 2 | Price alert settings | New table | alert settings store + handler | `publish_batch` + ebay scan | migration + curl |
| 3 | Card detail + history | Compound index | errgroup handler + history store | `insert_price_point` | `go get errgroup` + curl |
| 4 | Full health check | `SELECT 1` | health handler (DB + Redis check) | FastAPI `/health` | `health-check.sh` |
| 5 | Watchlist price scan | DISTINCT query | worker matching logic | `get_watched_card_names` | psql insert + logs |
| 6 | Deal of the day | UPSERT deal | `GetTodaysDeals` + public route | `find_and_save_deals` | psql seed + curl |
| 7 | Inventory portfolio | UPDATE + JOIN | inventory service with totalValue | `portfolio_updater.py` | psql insert + curl |
| 8 | Trending cards feed | UPDATE score | `GetTrending` + handler | `numpy` regression | psql seed + curl |
| 9 | API key management | Encrypted keys | crypto + api_key_store + handler | DB credential loading | store + verify encrypted |
| 10 | Graceful shutdown | DELETE old alerts | `signal.Notify` + cleanup ticker | `task.cancel()` in lifespan | `docker kill --signal=SIGTERM` |
