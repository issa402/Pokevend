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

---

## Task 11 — Write Your First Go Unit Test

> **Files for this task:**
> - `server/services/` — new `auth_service_test.go`
> - No SQL, No Python, No Bash (tests are pure Go)

**What you're building:** A real unit test for `AuthService.HashPassword` that verifies bcrypt hashing is producing valid output without a DB.

### SQL
No DB needed — unit tests mock their dependencies. That's the point.

### Go — create `server/services/auth_service_test.go`
```go
package services_test  // _test = only compiled during test runs

import (
    "testing"
    "pokemontool/services"
    // go test automatically finds files ending in _test.go
)

// Test function MUST start with "Test" + capital letter
func TestHashPassword(t *testing.T) {
    svc := services.NewAuthService(nil, nil, nil, "test-jwt-secret")
    // nil dependencies: unit tests don't need a real DB
    // This is WHY we use dependency injection — testability

    hash, err := svc.HashPassword("mypassword123")

    // t.Fatal: marks test failed + stops the test immediately
    // t.Error: marks test failed but continues
    if err != nil {
        t.Fatalf("HashPassword returned error: %v", err)
    }
    if len(hash) == 0 {
        t.Error("expected non-empty hash")
    }
    if hash == "mypassword123" {
        t.Error("hash should not equal plaintext password")
    }
}

func TestCheckPassword(t *testing.T) {
    svc := services.NewAuthService(nil, nil, nil, "test-jwt-secret")
    hash, _ := svc.HashPassword("correct-horse")

    // Test correct password
    if err := svc.CheckPassword(hash, "correct-horse"); err != nil {
        t.Errorf("correct password should pass: %v", err)
    }

    // Test wrong password
    if err := svc.CheckPassword(hash, "wrong-horse"); err == nil {
        t.Error("wrong password should fail, got nil error")
    }
}
```

### Python — add `pytest` test
Create `services/api-consumer/tests/test_schemas.py`:
```python
from models.schemas import EbayListing

def test_ebay_listing_valid():
    listing = EbayListing(
        card_name="Charizard",
        price=89.99,
        marketplace="ebay",
        listing_url="https://ebay.com/itm/1234",
    )
    assert listing.card_name == "Charizard"
    assert listing.price == 89.99

def test_ebay_listing_invalid_price():
    # Pydantic should reject negative prices
    try:
        EbayListing(card_name="Pikachu", price=-5.00,
                    marketplace="ebay", listing_url="https://ebay.com/1")
        assert False, "Should have raised ValidationError"
    except Exception:
        pass  # Expected
```

### Bash
```bash
# Run Go tests
cd server && go test ./services/... -v -run TestHashPassword
# -v = verbose (shows PASS/FAIL for each test)
# -run TestHashPassword = run only tests matching this name regex

# Run ALL Go tests
cd server && go test ./...

# Run Python tests (install pytest first)
pip install pytest
cd services/api-consumer && python3 -m pytest tests/ -v
```

---

## Task 12 — SQL Transactions + ACID

> **Files for this task:**
> - `server/store/inventory_store.go` (Go) — use pgx transaction
> - `services/analytics-engine/repositories/card_repo.py` (Python) — explicit commit/rollback
> - No new SQL file — transactions are in application code

**What you're building:** When a user adds a card to inventory AND it updates the card's inventory count atomically — both succeed or both fail together.

### SQL (no file — transactions are code-level)
```sql
-- A transaction is a GROUP of SQL statements that must ALL succeed:
BEGIN;
    INSERT INTO inventory(user_id, card_id, quantity) VALUES($1, $2, $3);
    UPDATE cards SET updated_at = NOW() WHERE card_id = $2;
COMMIT;
-- If INSERT succeeds but UPDATE fails → ROLLBACK rolls BOTH back
-- ACID: Atomicity (all or nothing), Consistency, Isolation, Durability

-- Without transaction (DANGEROUS):
INSERT INTO inventory...;  -- succeeds
UPDATE cards...;           -- fails → inventory row exists but cards not updated → INCONSISTENT STATE
```

### Go — `server/store/inventory_store.go`
```go
// Add: InsertWithTransaction(ctx, userID, cardID string, qty int) error
func (s *postgresInventoryStore) InsertWithTransaction(ctx context.Context, userID, cardID string, qty int) error {
    // 1. tx, err := s.db.Begin(ctx)
    // 2. defer tx.Rollback(ctx)  ← rolls back if we return early with error
    // 3. tx.Exec(ctx, "INSERT INTO inventory...")
    // 4. tx.Exec(ctx, "UPDATE cards SET updated_at=NOW()...")
    // 5. return tx.Commit(ctx)   ← only commits if we reach here
    //
    // pgx transaction: pool.Begin(ctx) → returns pgx.Tx
    // pgx.Tx has same methods as pool: Query, Exec, QueryRow
}
```

### Python — `services/analytics-engine/repositories/card_repo.py`
Add explicit error handling around transactions:
```python
def bulk_update_prices(self, updates: List[dict]) -> int:
    """
    Update multiple cards' prices in one transaction.
    All updates succeed or none do.
    
    psycopg2 transactions:
    - conn.autocommit = False (default): every statement is in a transaction
    - conn.commit(): apply all changes
    - conn.rollback(): undo all changes since last commit
    """
    updated = 0
    try:
        with self.conn.cursor() as cur:
            for update in updates:
                cur.execute(
                    "UPDATE cards SET price_tcgplayer=%s, updated_at=NOW() WHERE card_id=%s",
                    (update["price"], update["card_id"])
                )
                updated += cur.rowcount
        self.conn.commit()   # write ALL updates atomically
        return updated
    except Exception as e:
        self.conn.rollback() # undo ALL updates if any one fails
        raise  # re-raise so caller knows it failed
```

### Bash
```bash
# Verify ACID — simulate a transaction failure
docker exec -it pokemontool_postgres psql -U pokemontool_user -d pokemontool -c "
BEGIN;
INSERT INTO inventory(user_id, card_id, quantity)
SELECT id, 'base1-4', 1 FROM users LIMIT 1;
-- Intentionally fail the second statement:
INSERT INTO inventory(user_id, card_id, quantity) VALUES('bad-uuid', 'base1-4', 1);
COMMIT;
"
# Result: ERROR on second INSERT → ROLLBACK → zero rows inserted
# Verify with:
docker exec -it pokemontool_postgres psql -U pokemontool_user -d pokemontool \
  -c "SELECT COUNT(*) FROM inventory WHERE card_id='base1-4';"
# Should be 0 if you hadn't inserted before
```

---

## Task 13 — Python OOP: Abstract Base Class for Repositories

> **Files for this task:**
> - `services/api-consumer/repositories/base_repo.py` (Python) — new abstract base class
> - `services/api-consumer/repositories/ebay_repo.py` (Python) — inherit from base
> - `services/analytics-engine/repositories/card_repo.py` (Python) — same pattern

**What you're building:** A shared `BaseRepository` that all repos inherit from. Teaches the Python OOP pattern used in production codebases.

### SQL
No changes — this is a Python architecture task.

### Go
No changes — Go uses interfaces, not inheritance. This task is Python-focused.

### Python — create `services/api-consumer/repositories/base_repo.py`
```python
"""
Base class for all repositories.
Teaches: ABC (Abstract Base Class), @abstractmethod, inheritance, __init_subclass__

PYTHON OOP KEY CONCEPTS:
  class Foo(Bar): Foo inherits from Bar (gets all Bar methods)
  super().__init__(): call the parent class's __init__
  @abstractmethod: subclass MUST implement this method
  ABC: Abstract Base Class — cannot be instantiated directly

FAANG PATTERN: Repository Base Class
  All repos share: connection management, logging, error formatting.
  Putting these in a base class = DRY principle.
  New repos inherit for free — just implement the abstract methods.
"""
import logging
from abc import ABC, abstractmethod

import psycopg2


class BaseRepository(ABC):  # ABC = Abstract Base Class
    """
    Base class for all database repositories.
    Provides: connection, cursor context manager, logging.
    
    ABC: you cannot do BaseRepository() — it's abstract.
    You must subclass it and implement all @abstractmethod methods.
    """

    def __init__(self, conn: psycopg2.extensions.connection):
        # super().__init__() would call ABC.__init__ — not needed here but good practice
        self.conn = conn
        self.logger = logging.getLogger(self.__class__.__name__)
        # self.__class__.__name__ = the name of the SUBCLASS, not "BaseRepository"
        # So CardRepo's logger is named "CardRepo", EbayRepo's is "EbayRepo"
        # This makes log lines instantly tell you which repo had an issue

    def _execute(self, sql: str, params: tuple = ()) -> list:
        """
        Execute a SELECT query and return all rows.
        Protected method (single underscore = "don't call from outside the class").
        """
        with self.conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
            cur.execute(sql, params)
            return cur.fetchall()

    def _execute_write(self, sql: str, params: tuple = ()) -> int:
        """
        Execute INSERT/UPDATE/DELETE and commit.
        Returns the number of affected rows.
        """
        with self.conn.cursor() as cur:
            cur.execute(sql, params)
            affected = cur.rowcount
        self.conn.commit()
        return affected

    @abstractmethod  # <- subclass MUST implement this
    def health_check(self) -> bool:
        """Return True if the repository can reach its data source."""
        ...  # ... = "not implemented" (same as pass but more intentional)
```

Then update `EbayRepo` (and `CardRepo`) to inherit:
```python
from repositories.base_repo import BaseRepository

class EbayRepo(BaseRepository):
    def __init__(self, conn):
        super().__init__(conn)  # call BaseRepository.__init__

    def health_check(self) -> bool:
        try:
            self._execute("SELECT 1")  # inherited from BaseRepository
            return True
        except Exception:
            return False
```

### Bash
```bash
# Verify the class works without a real DB connection (Python REPL)
cd services/api-consumer
python3 -c "
from repositories.base_repo import BaseRepository

# This should fail — ABC cannot be instantiated directly
try:
    b = BaseRepository(None)
    print('ERROR: should have raised TypeError')
except TypeError as e:
    print(f'Correct! BaseRepository cannot be instantiated: {e}')
"
```

---

## Task 14 — Bash: Cron Job + Log Rotation

> **Files for this task:**
> - `scripts/cron_scan.sh` — new cron-compatible scan trigger script
> - `scripts/rotate_logs.sh` — new log rotation script
> - No Go changes, No SQL changes, No Python changes (this is pure Bash + Linux)

**What you're building:** Two production operations scripts — one that triggers a scan on a schedule (usable by cron), one that cleans up old log files so your disk doesn't fill up.

### SQL (no file — cron is OS-level)
```bash
# What is cron? The Linux job scheduler.
# crontab -e to edit, format:
#   minute hour day month weekday command
#   *      *    *   *     *       = "every" (wildcard)
# 
# Examples:
#   0 * * * *  = every hour at :00
#   */5 * * * * = every 5 minutes
#   0 2 * * *  = every day at 2am
#   0 9 * * 1  = every Monday at 9am
```

### Go
No changes — this is infrastructure not app code.

### Python — No changes either. Cron will trigger the Python service via a REST call.

### Bash — create `scripts/cron_scan.sh`
```bash
#!/usr/bin/env bash
# ============================================================
# FILE: scripts/cron_scan.sh
# PURPOSE: Trigger a card price scan on a schedule via cron
#
# INSTALL:
#   crontab -e
#   # Run every hour:
#   0 * * * * /opt/pokemontool/scripts/cron_scan.sh >> /var/log/pokemontool/scan.log 2>&1
#   # >> appends to log file (don't use > or you'll overwrite)
#   # 2>&1 redirects stderr to stdout (both go to the log file)
# ============================================================
set -euo pipefail

LOG_FILE="/var/log/pokemontool/scan.log"
FASTAPI_URL="${FASTAPI_URL:-http://localhost:8001}"

# Timestamp every log line (cron doesn't add timestamps)
log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"; }

log "=== Scan triggered by cron ==="

# Trigger the FastAPI webhook endpoint
response=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST "$FASTAPI_URL/webhook/scan" \
    -H "Content-Type: application/json" \
    -d '{"source": "cron", "cards": ["Charizard", "Pikachu"]}')

if [[ "$response" == "200" ]]; then
    log "✓ Scan triggered successfully (HTTP $response)"
else
    log "✗ Scan trigger failed (HTTP $response)"
    exit 1
fi
```

Create `scripts/rotate_logs.sh`:
```bash
#!/usr/bin/env bash
# ============================================================
# FILE: scripts/rotate_logs.sh
# PURPOSE: Rotate logs older than 7 days to prevent disk fill
#
# WHAT IS LOG ROTATION?
# Services write logs continuously. Without rotation:
# /var/log/pokemontool/app.log → grows to 50GB → disk full → crash
# With rotation: keep 7 days, compress old logs, delete older ones.
#
# Real production uses: logrotate (system tool) or cloud logging.
# This script teaches the concept manually first.
#
# INSTALL:
#   crontab -e
#   0 0 * * * /opt/pokemontool/scripts/rotate_logs.sh
#   (runs at midnight every day)
# ============================================================
set -euo pipefail

LOG_DIR="/var/log/pokemontool"
KEEP_DAYS=7

log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"; }

log "=== Log rotation started ==="

# Create log directory if it doesn't exist
mkdir -p "$LOG_DIR"

# find: search for files
# -name "*.log": files ending in .log
# -mtime +7: modified more than 7 days ago
# -exec: run command on each found file
# {}: placeholder for the filename
# \;: end of -exec command
find "$LOG_DIR" -name "*.log" -mtime "+$KEEP_DAYS" -exec gzip {} \;
log "✓ Compressed logs older than $KEEP_DAYS days"

# Delete compressed logs older than 30 days
find "$LOG_DIR" -name "*.log.gz" -mtime +30 -delete
log "✓ Deleted compressed logs older than 30 days"

# Show current log disk usage
du -sh "$LOG_DIR" 2>/dev/null && log "Log dir size: $(du -sh "$LOG_DIR" | cut -f1)"
```

### Bash (install the cron job)
```bash
# Make scripts executable
chmod +x scripts/cron_scan.sh scripts/rotate_logs.sh

# Test manually first (before adding to cron)
./scripts/cron_scan.sh
./scripts/rotate_logs.sh

# View current crontab
crontab -l

# Add to cron (opens vim or nano):
crontab -e
# Add: 0 * * * * /opt/pokemontool/scripts/cron_scan.sh >> /tmp/scan.log 2>&1
# Add: 0 0 * * * /opt/pokemontool/scripts/rotate_logs.sh >> /tmp/rotate.log 2>&1
```

---

## Task 15 — CI/CD: Set Up GitHub Actions + Push to Trigger It

> **Files for this task:**
> - `.github/workflows/ci.yml` ← already created for you
> - `.github/workflows/deploy.yml` ← already created for you
> - This task is about CONFIGURING GitHub to use them

**What you're building:** Get GitHub Actions actually running on your repo. Push code → GitHub automatically builds and checks it.

### SQL
No changes.

### Go
No changes — the CI pipeline builds your existing Go code.

### Python — add `ruff` and `mypy` to `requirements.txt`
In `services/api-consumer/requirements.txt`, add:
```
ruff>=0.3.0
mypy>=1.9.0
```
The CI pipeline installs these and runs them against your code.

### Bash — push your code and set secrets
```bash
# Step 1: Push your code to GitHub
cd C:\Users\isjim\OneDrive\Desktop\Pokemon
git init  # if not already a git repo
git add .
git commit -m "feat: add CI/CD workflows, automation scripts, practice tasks"
git remote add origin https://github.com/YOUR_USERNAME/Pokemon.git
git push -u origin main

# Step 2: Watch CI run
# Go to: https://github.com/YOUR_USERNAME/Pokemon/actions
# You should see the "CI" workflow running
# Click into it to see each job's output

# Step 3: Set deploy secrets (for the deploy.yml workflow)
# Go to: GitHub → Repo → Settings → Secrets and variables → Actions
# Add:
#   EC2_HOST     = your EC2 public IP (e.g. 54.123.45.67)
#   EC2_USER     = ubuntu
#   EC2_SSH_KEY  = contents of your .pem file (cat ~/pokemontool.pem)

# Step 4: Verify ruff passes locally before pushing
pip install ruff
cd services/api-consumer && ruff check .
# Fix any issues it reports, then push again
```

---

## Updated Quick Reference (All 15 Tasks)

| Task | Feature | Hardest Concept |
|------|---------|-----------------|
| 1 | Unread badge endpoint | Partial SQL index |
| 2 | Price alert settings | New migration + worker logic |
| 3 | Card detail + history | `errgroup` concurrent fetch |
| 4 | Full health check | Pinging DB/Redis in handler |
| 5 | Watchlist price scan | End-to-end flow |
| 6 | Deal of the day | UPSERT + scheduler |
| 7 | Inventory portfolio | JOIN + server-side aggregation |
| 8 | Trending cards | numpy linear regression |
| 9 | API key management | AES encryption in Go |
| 10 | Graceful shutdown | `signal.Notify` + `task.cancel()` |
| 11 | Go unit tests | `testing.T`, mocking with nil deps |
| 12 | SQL transactions | `BEGIN/COMMIT/ROLLBACK` + pgx.Tx |
| 13 | Python OOP | ABC, `@abstractmethod`, inheritance |
| 14 | Cron + log rotation | `crontab`, `find -mtime`, `gzip` |
| 15 | Live CI/CD | GitHub Actions secrets + push trigger |

