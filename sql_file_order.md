# SQL / PostgreSQL: The Exact Order You Build Database Files
## And WHY That Order Is the FAANG Standard

Database files are built in strict dependency order too.
You can't create an index on a table that doesn't exist.
You can't add a foreign key to a column that isn't there yet.
The order is enforced by PostgreSQL itself — build wrong and it'll tell you.

---

## The Mental Model: Schema → Indexes → Triggers → Views → Seeds

```
ENUMS and EXTENSIONS (shared types used by tables below)
       ↓
TABLES in dependency order (referenced tables before referencing)
       ↓
INDEXES (performance — created after tables exist)
       ↓
TRIGGERS + FUNCTIONS (automation — depend on tables)
       ↓
VIEWS (virtual tables — depend on real tables)
       ↓
SEEDS (data — depends on schema being complete)
```

---

## Step 1: Extensions and Custom Types
**First lines of every migration file.**

```sql
-- Enable UUID generation (PostgreSQL doesn't ship with this by default)
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
-- gen_random_uuid() is provided by pgcrypto — used as DEFAULT for PRIMARY KEYs

-- IF NOT EXISTS = idempotent. Safe to run the migration twice.
-- Without IF NOT EXISTS: running migration twice crashes on "extension already exists"
-- FAANG rule: ALL DDL must be idempotent in migrations
```

**Why first?** Everything in the schema can reference these.
`gen_random_uuid()` is used as the DEFAULT value for every `id` column.
PostgreSQL must know what `gen_random_uuid()` is before you reference it.

---

## Step 2: Tables — In Dependency Order (No FK References First)

Write tables in this exact sequence — a table can only reference tables already created.

### Substep 2a: Tables with NO foreign keys (standalone)
```sql
-- The "root" entities — nothing depends on them yet, they depend on nothing
CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           TEXT UNIQUE NOT NULL,
    password_hash   TEXT NOT NULL,         -- bcrypt hash (never plaintext)
    display_name    TEXT,                  -- nullable — not required at signup
    zip_code        TEXT,                  -- nullable — for local show search
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- cards has no FK to users — it's a global catalog
CREATE TABLE IF NOT EXISTS cards (
    card_id         TEXT PRIMARY KEY,      -- "base1-4" (set code + card number)
    name            TEXT NOT NULL,
    set_name        TEXT NOT NULL,
    rarity          TEXT,
    image_url       TEXT,
    trend_label     TEXT DEFAULT 'STABLE', -- "RISING" | "FALLING" | "STABLE"
    trending_score  INT DEFAULT 0,
    price_tcgplayer DECIMAL(10,2),
    price_ebay      DECIMAL(10,2),
    price_facebook  DECIMAL(10,2),
    price_mercari   DECIMAL(10,2),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- shows has no FK — independent event data

CREATE TABLE IF NOT EXISTS shows (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name           TEXT NOT NULL,
    venue_name     TEXT,
    city           TEXT,
    state          TEXT,
    zip_code       TEXT,
    start_date     TIMESTAMPTZ,
    end_date       TIMESTAMPTZ,
    description    TEXT,
    eventbrite_id  TEXT UNIQUE,   -- prevent duplicate shows from same source
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

**Key decisions explained:**
- `UUID` PKs not `SERIAL INT`: UUIDs don't reveal record count, safe to expose in URLs
- `TIMESTAMPTZ` not `TIMESTAMP`: always store timezone-aware times (UTC normalized)
- `DECIMAL(10,2)` not `FLOAT`: exact decimal math for money (FLOAT has rounding errors)
- `TEXT` not `VARCHAR(255)`: PostgreSQL stores TEXT and VARCHAR the same way internally
- `IF NOT EXISTS`: idempotent — migration can run on a database that already has the table

### Substep 2b: Tables with FK to users (one join away)
```sql
-- watchlists references users — write AFTER users
CREATE TABLE IF NOT EXISTS watchlists (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- ON DELETE CASCADE: when user is deleted, all their watchlists are deleted too
    -- Alternatives: ON DELETE SET NULL (orphan the row), ON DELETE RESTRICT (prevent deletion)
    card_name   TEXT NOT NULL,
    target_price DECIMAL(10,2),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, card_name)  -- one watchlist entry per card per user
);

-- api_keys references users
CREATE TABLE IF NOT EXISTS api_keys (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform      TEXT NOT NULL,        -- "ebay", "tcgplayer"
    encrypted_key TEXT NOT NULL,        -- AES-256-GCM encrypted (see pkg/crypto.go)
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- inventory references users + cards
CREATE TABLE IF NOT EXISTS inventory (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    card_id         TEXT NOT NULL REFERENCES cards(card_id) ON DELETE CASCADE,
    -- TEXT FK to cards(card_id) — cards uses TEXT PK so FK is also TEXT
    quantity        INT NOT NULL DEFAULT 1 CHECK(quantity > 0),
    -- CHECK constraint: PostgreSQL rejects quantity <= 0 at the DB level
    purchase_price  DECIMAL(10,2),
    current_value   DECIMAL(10,2),      -- updated by analytics-engine
    condition       TEXT DEFAULT 'NM',  -- NM, LP, MP, HP, DMG
    notes           TEXT,
    purchased_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Substep 2c: Tables with FK to multiple parents
```sql
-- market_listings references cards
CREATE TABLE IF NOT EXISTS market_listings (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id      TEXT REFERENCES cards(card_id) ON DELETE SET NULL,
    -- SET NULL: if card is deleted, keep listing row but clear the card_id
    card_name    TEXT NOT NULL,
    price        DECIMAL(10,2) NOT NULL,
    marketplace  TEXT NOT NULL,
    listing_url  TEXT,
    discovered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed    BOOLEAN DEFAULT false   -- true once notification_worker processed it
);

-- alerts references users
CREATE TABLE IF NOT EXISTS alerts (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    card_name    TEXT,
    alert_type   TEXT NOT NULL,   -- "PRICE_DROP" | "PRICE_SPIKE" | "TREND_CHANGE" | "DEAL_OF_DAY"
    message      TEXT NOT NULL,
    marketplace  TEXT,
    price        DECIMAL(10,2),
    listing_url  TEXT,
    is_read      BOOLEAN NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- deals references cards
CREATE TABLE IF NOT EXISTS deals (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    card_name     TEXT NOT NULL,
    market_price  DECIMAL(10,2) NOT NULL,
    best_price    DECIMAL(10,2) NOT NULL,
    savings_pct   DECIMAL(5,2) NOT NULL,
    listing_url   TEXT NOT NULL,
    marketplace   TEXT NOT NULL,
    deal_date     DATE NOT NULL DEFAULT CURRENT_DATE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(card_name, marketplace, deal_date)  -- one deal per card per marketplace per day
);

-- price_history references cards — time-series price data
CREATE TABLE IF NOT EXISTS price_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id     TEXT NOT NULL REFERENCES cards(card_id) ON DELETE CASCADE,
    marketplace TEXT NOT NULL,
    price       DECIMAL(10,2) NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## Step 3: Indexes — Performance Layer

Write after ALL tables exist. Indexes reference tables and columns.

```sql
-- ── B-Tree Indexes (default — good for =, <, >, BETWEEN, ORDER BY) ────────

-- Every FK should have an index — PostgreSQL doesn't auto-create FK indexes
-- Without these, JOIN and DELETE on FK would do full table scans
CREATE INDEX IF NOT EXISTS idx_watchlists_user_id  ON watchlists(user_id);
CREATE INDEX IF NOT EXISTS idx_alerts_user_id       ON alerts(user_id);
CREATE INDEX IF NOT EXISTS idx_inventory_user_id    ON inventory(user_id);
CREATE INDEX IF NOT EXISTS idx_alerts_created_at    ON alerts(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_price_history_card_id ON price_history(card_id);

-- Compound index: queries that filter on MULTIPLE columns
-- idx_alerts_user_unread covers: WHERE user_id=$1 AND is_read=false
-- This is MUCH faster than two separate indexes — PostgreSQL uses both conditions at once
CREATE INDEX IF NOT EXISTS idx_alerts_user_unread ON alerts(user_id, is_read)
    WHERE is_read = false;
-- WHERE clause = PARTIAL INDEX: only indexes rows where is_read=false
-- Smaller index = faster queries, less disk space

-- ── GIN / Full-Text Search Indexes ────────────────────────────────────────
-- GIN = Generalized Inverted Index (works on text, arrays, JSONB)
-- tsvector = PostgreSQL's full-text search type
-- to_tsvector('english', name): tokenize "Charizard Holo" → {'charizard', 'holo'}
-- This powers: WHERE to_tsvector('english', name) @@ to_tsquery('charizard')
CREATE INDEX IF NOT EXISTS idx_cards_name_fts ON cards
    USING GIN(to_tsvector('english', name));

-- Partial index on shows for upcoming events only
-- No point indexing past shows — we never query them
CREATE INDEX IF NOT EXISTS idx_shows_upcoming ON shows(start_date)
    WHERE start_date >= NOW();
```

**Index decision framework:**
| Query pattern | Index to create |
|---|---|
| `WHERE id = $1` | Usually not needed — PRIMARY KEY is auto-indexed |
| `WHERE user_id = $1` | `CREATE INDEX ON table(user_id)` |
| `WHERE user_id=$1 AND is_read=false` | `CREATE INDEX ON table(user_id, is_read)` |
| `ORDER BY created_at DESC` | `CREATE INDEX ON table(created_at DESC)` |
| `WHERE name ILIKE '%charizard%'` | GIN with `to_tsvector` |
| `WHERE start_date >= NOW()` | Partial index with `WHERE start_date >= NOW()` |

---

## Step 4: Functions and Triggers

Write after tables and indexes — triggers need tables to exist, and trigger functions often query tables.

```sql
-- ── Trigger Function: auto-update updated_at ──────────────────────────────
-- This is a reusable function attached to multiple tables
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    -- NEW = the row AFTER the update
    -- OLD = the row BEFORE the update (available in UPDATE/DELETE triggers)
    NEW.updated_at = NOW();
    RETURN NEW;  -- return the modified row (required for BEFORE triggers)
END;
$$ LANGUAGE plpgsql;
-- $$ = dollar-quoting (lets you use single quotes inside the function body)
-- LANGUAGE plpgsql = PostgreSQL's procedural language (like SQL + control flow)

-- Attach the trigger to tables with updated_at columns
-- BEFORE UPDATE = function runs BEFORE the row is written, so we can modify NEW
CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_cards_updated_at
    BEFORE UPDATE ON cards
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_inventory_updated_at
    BEFORE UPDATE ON inventory
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

**Why triggers instead of updating in code?**
If you update `updated_at` in Go/Python code: every developer must remember to do it.
If someone forgets: your `updated_at` column is stale — useless for debugging.
Trigger: runs automatically in the database — impossible to forget.

---

## Step 5: Views (Optional — for common complex queries)

```sql
-- A view is a saved SELECT query referenced like a table
-- No data is stored — runs the underlying query each time
CREATE OR REPLACE VIEW user_alert_counts AS
SELECT
    user_id,
    COUNT(*) FILTER (WHERE is_read = false) AS unread_count,
    COUNT(*) AS total_count
FROM alerts
GROUP BY user_id;

-- Query view like a table:
-- SELECT unread_count FROM user_alert_counts WHERE user_id = $1
```

---

## Step 6: Seed Data — After ALL Schema is Complete

```sql
-- seeds/pokemon_shows.sql
-- Insert initial data only AFTER all tables exist

INSERT INTO shows (name, venue_name, city, state, start_date, end_date)
VALUES (
    'Regional Championship',
    'Dallas Convention Center',
    'Dallas', 'TX',
    NOW() + INTERVAL '14 days',
    NOW() + INTERVAL '15 days'
) ON CONFLICT DO NOTHING;   -- idempotent: skip if already inserted
```

---

## Complete File Creation Order

```
Phase 1: Migration file starts with
  1.  CREATE EXTENSION IF NOT EXISTS "pgcrypto"
  2.  CREATE EXTENSION IF NOT EXISTS "uuid-ossp"  (backup UUID method)

Phase 2: Standalone tables (no FK dependencies)
  3.  CREATE TABLE users
  4.  CREATE TABLE cards
  5.  CREATE TABLE shows

Phase 3: Tables with FK to standalone tables
  6.  CREATE TABLE watchlists      → references users
  7.  CREATE TABLE api_keys        → references users
  8.  CREATE TABLE inventory       → references users + cards
  9.  CREATE TABLE market_listings → references cards
  10. CREATE TABLE alerts          → references users
  11. CREATE TABLE deals           → references cards
  12. CREATE TABLE price_history   → references cards

Phase 4: Indexes
  13. CREATE INDEX on all FK columns
  14. CREATE INDEX for common query patterns
  15. CREATE INDEX for ORDER BY columns
  16. GIN indexes for full-text search
  17. Partial indexes for filtered queries

Phase 5: Functions and Triggers
  18. CREATE FUNCTION update_updated_at_column()
  19. CREATE TRIGGER for each table with updated_at

Phase 6: Views (optional)
  20. CREATE VIEW for complex reused queries

Phase 7: Seeds (separate file, run separately)
  21. INSERT INTO shows ... ON CONFLICT DO NOTHING
  22. INSERT INTO cards ... ON CONFLICT DO NOTHING
```

---

## The Rule Behind the Order

```
Table B can only REFERENCE Table A if A is created first.
Index X can only exist if its table exists first.
Trigger Y can only fire if its table exists first.
Seed data can only INSERT if the schema exists first.
```

PostgreSQL enforces this with errors:
- FK to non-existent table → `ERROR: relation "users" does not exist`
- Index on non-existent column → `ERROR: column "user_id" does not exist`

The creation order is not a style choice — it's a hard constraint.

---

## The 5 Most Important SQL Concepts for FAANG

```sql
-- 1. UPSERT (INSERT ... ON CONFLICT)
INSERT INTO cards(card_id, name, price_tcgplayer)
VALUES('base1-4', 'Charizard', 89.99)
ON CONFLICT (card_id) DO UPDATE
    SET price_tcgplayer = EXCLUDED.price_tcgplayer,
        updated_at = NOW();
-- EXCLUDED = the row that WOULD have been inserted (before conflict)
-- Idempotent price updates: insert if new, update price if exists

-- 2. WINDOW FUNCTIONS (analytics)
SELECT card_id, price, recorded_at,
    AVG(price) OVER (
        PARTITION BY card_id           -- separate average per card
        ORDER BY recorded_at
        ROWS BETWEEN 6 PRECEDING AND CURRENT ROW  -- 7-day rolling average
    ) AS rolling_avg_7d
FROM price_history;

-- 3. CTEs (Common Table Expressions — readable complex queries)
WITH recent_prices AS (
    SELECT card_id, AVG(price) AS avg_price
    FROM price_history
    WHERE recorded_at >= NOW() - INTERVAL '7 days'
    GROUP BY card_id
),
market_prices AS (
    SELECT card_id, price_tcgplayer AS market
    FROM cards
)
SELECT r.card_id, r.avg_price, m.market,
    (m.market - r.avg_price) / m.market * 100 AS discount_pct
FROM recent_prices r
JOIN market_prices m ON r.card_id = m.card_id
WHERE r.avg_price < m.market * 0.8;  -- 20%+ discount = deal

-- 4. EXPLAIN ANALYZE (performance debugging)
EXPLAIN ANALYZE
SELECT * FROM alerts WHERE user_id = 'some-uuid' AND is_read = false;
-- Look for: "Index Scan" (good) vs "Seq Scan" (bad — add an index)
-- "Seq Scan on alerts (cost=0.00..1234)" = reading every row
-- "Index Scan using idx_alerts_user_unread" = using your index

-- 5. Transactions (atomic groups of operations)
BEGIN;
    INSERT INTO alerts(user_id, message) VALUES('uuid', 'Price dropped!');
    UPDATE watchlists SET last_alerted = NOW() WHERE id = 'wl-uuid';
COMMIT;
-- Both happen or neither happens — if second fails, first is rolled back
-- psycopg2: conn.commit() after all writes in a block
```
