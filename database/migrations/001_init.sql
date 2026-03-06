-- ============================================================
-- FILE: database/migrations/001_init.sql
-- TYPE: PostgreSQL Migration Script
--
-- WHAT IS THIS?
-- A "migration" is a versioned SQL file that defines or changes
-- your database schema. Think of it like Git commits for your DB.
-- This is migration #001 — the very first one, creating all tables.
--
-- FAANG STANDARD: Every schema change gets its own numbered file:
--   001_init.sql       ← you are here (creates all tables)
--   002_add_index.sql  ← next change
--   003_add_column.sql ← another change
-- You NEVER modify 001_init.sql again once it runs in production.
-- You create NEW files instead. This gives you full history.
--
-- HOW IT RUNS:
-- Docker automatically executes this file when the postgres container
-- starts for the first time (mapped in docker-compose.yml as initdb).
-- You can also run it manually:
--   docker exec -i pokemontool_postgres psql -U pokemontool_user -d pokemontool < 001_init.sql
--
-- KEY CONCEPTS DEMONSTRATED:
--   UUID, PRIMARY KEY, FOREIGN KEY, INDEX, TRIGGER, ON DELETE CASCADE,
--   UNIQUE constraint, DEFAULT values, IF NOT EXISTS (idempotent)
-- ============================================================

-- ============================================================
-- EXTENSION: pgcrypto
-- PostgreSQL comes with extensions that add extra functions.
-- pgcrypto gives us gen_random_uuid() for creating UUIDs.
--
-- FAANG pattern: Always use extensions with IF NOT EXISTS
-- so the script is safe to run multiple times (idempotent).
--
-- WHY UUID over INT?
-- INT auto-increments: 1, 2, 3... users can guess other users' IDs
-- UUID: random 128-bit string — not guessable, works across distributed DBs
-- Stripe, GitHub, and most FAANG APIs use UUIDs in their public APIs
-- ============================================================
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================
-- TABLE: users
-- Stores vendor accounts. This is the "root" table — everything
-- else (watchlists, alerts, inventory) belongs to a user.
--
-- FAANG PATTERNS HERE:
-- 1. UUID primary key — not guessable, globally unique
-- 2. email UNIQUE — enforced at DB level, not just app level
-- 3. password_hash — NEVER store plaintext passwords
--    We store a bcrypt hash (Go's golang.org/x/crypto/bcrypt)
-- 4. TIMESTAMPTZ — timestamp WITH timezone. Use this, never TIMESTAMP.
-- 5. updated_at — tracked via a TRIGGER (see below)
-- ============================================================
CREATE TABLE IF NOT EXISTS users (
    -- gen_random_uuid() creates a new UUID on every INSERT automatically
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- UNIQUE constraint: PostgreSQL rejects duplicate emails at the DB level
    -- If app code fails to check, the DB is the safety net
    email           VARCHAR(255) UNIQUE NOT NULL,

    -- The bcrypt hash of the user's password. Looks like:
    -- "$2a$12$..." — never decrypt this, just compare with bcrypt.CompareHashAndPassword
    password_hash   VARCHAR(255) NOT NULL,

    display_name    VARCHAR(100),
    zip_code        VARCHAR(20),    -- Used by Python show_finder.py for proximity search

    -- NOW() = current timestamp in PostgreSQL
    -- TIMESTAMPTZ stores both the time AND the timezone (UTC internally)
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()   -- auto-updated by trigger below
);

-- ============================================================
-- TABLE: api_keys
-- Stores encrypted third-party API keys (eBay, TCGplayer, Eventbrite).
--
-- FAANG SECURITY PATTERN:
-- Never store API keys in plaintext. Our Go server (pkg/crypto.go)
-- encrypts them with AES-256-GCM before INSERT, and decrypts on read.
-- If someone dumps your database, they see gibberish, not real keys.
--
-- REFERENCES = foreign key — user_id must exist in users.id
-- ON DELETE CASCADE = when a user is deleted, their API keys are too
-- UNIQUE(user_id, platform) = one key per platform per user
-- ============================================================
CREATE TABLE IF NOT EXISTS api_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Foreign key: api_keys.user_id → users.id
    -- If this user is deleted: CASCADE deletes their keys automatically
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    platform        VARCHAR(50) NOT NULL,   -- 'ebay', 'tcgplayer', 'eventbrite'

    -- The AES-256-GCM encrypted key value (hex string)
    -- Decryption happens in Go (pkg/crypto.go), not here
    encrypted_key   TEXT NOT NULL,

    key_label       VARCHAR(100),           -- User-friendly name for the key
    created_at      TIMESTAMPTZ DEFAULT NOW(),

    -- Composite unique constraint: prevents one user having two keys for same platform
    UNIQUE(user_id, platform)
);

-- ============================================================
-- TABLE: watchlists
-- Cards a vendor is tracking for price alerts.
-- When Python publishes a listing, Go's notification worker
-- checks it against this table to decide whether to fire an alert.
--
-- KEY DESIGN: target_buy_price / target_sell_price are NULLABLE.
-- NULL means "no threshold set for this direction."
-- The notification worker only alerts when a non-NULL value is exceeded.
-- ============================================================
CREATE TABLE IF NOT EXISTS watchlists (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    card_name       VARCHAR(255) NOT NULL,
    set_name        VARCHAR(255),

    -- DECIMAL(10, 2) = up to 10 digits total, 2 after decimal
    -- Use DECIMAL for money ALWAYS — float has rounding errors (0.1 + 0.2 != 0.3)
    target_buy_price DECIMAL(10, 2),    -- Alert when listing price drops BELOW this
    target_sell_price DECIMAL(10, 2),   -- Alert when listing price rises ABOVE this

    notes           TEXT,
    added_at        TIMESTAMPTZ DEFAULT NOW()
);

-- INDEX: Speeds up "WHERE user_id = ?" queries
-- Without this, PostgreSQL scans every row in the table (Seq Scan)
-- With this, it jumps directly to that user's rows (Index Scan)
-- FAANG rule: always index foreign key columns
CREATE INDEX IF NOT EXISTS idx_watchlists_user_id ON watchlists(user_id);

-- ============================================================
-- TABLE: alerts
-- Notification history: every price alert that fired for a user.
-- Written by Go's worker/notification_worker.go
-- Read by Go's handlers/alert_handler.go
--
-- alert_type values:
--   PRICE_DROP    — listing price dropped below target_buy_price
--   PRICE_SPIKE   — listing price rose above target_sell_price
--   NEW_LISTING   — new listing appeared for a watched card
--   TREND_CHANGE  — Python analytics marked card as RISING/FALLING
--   DEAL_OF_DAY   — Python found a significant below-market deal
-- ============================================================
CREATE TABLE IF NOT EXISTS alerts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    card_name       VARCHAR(255),
    alert_type      VARCHAR(50) NOT NULL,  -- see values above
    message         TEXT NOT NULL,         -- human-readable explanation
    marketplace     VARCHAR(50),           -- 'ebay', 'tcgplayer', 'facebook', 'mercari'
    price           DECIMAL(10, 2),
    listing_url     TEXT,
    is_read         BOOLEAN DEFAULT FALSE, -- false until user views/dismisses it
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Index on user_id — for "show me this user's alerts" query
CREATE INDEX IF NOT EXISTS idx_alerts_user_id ON alerts(user_id);

-- PARTIAL INDEX: only unread alerts — much smaller index, faster for unread queries
-- This is a more advanced optimization — only works if you always query unread separately
CREATE INDEX IF NOT EXISTS idx_alerts_is_read ON alerts(is_read)
    WHERE is_read = FALSE;  -- only index unread rows

-- ============================================================
-- TABLE: inventory
-- A vendor's personal card collection. Tracks what they own,
-- what they paid, and the current market value.
--
-- condition values: NM (Near Mint), LP (Lightly Played),
--   MP (Moderately Played), HP (Heavily Played), Damaged
-- These are the standard TCG card condition grades.
-- ============================================================
CREATE TABLE IF NOT EXISTS inventory (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    card_name       VARCHAR(255) NOT NULL,
    set_name        VARCHAR(255),
    card_number     VARCHAR(50),        -- e.g. "4/102" (Base Set Charizard)
    condition       VARCHAR(20),
    quantity        INTEGER DEFAULT 1,
    purchase_price  DECIMAL(10, 2),     -- What the vendor paid for it
    current_value   DECIMAL(10, 2),     -- Last fetched market price (updated by analytics)
    notes           TEXT,
    acquired_at     TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()  -- auto-updated by trigger
);

CREATE INDEX IF NOT EXISTS idx_inventory_user_id ON inventory(user_id);

-- ============================================================
-- TABLE: shows
-- Upcoming Pokémon TCG events/tournaments.
-- Populated by Python's analytics-engine/analyzers/show_finder.py
-- using the Eventbrite API. Cached here to avoid hitting the
-- Eventbrite API on every request.
--
-- eventbrite_id UNIQUE: prevents inserting the same event twice
-- even if the Python service runs multiple times.
-- lat/long stored for future geolocation "shows near me" feature.
-- ============================================================
CREATE TABLE IF NOT EXISTS shows (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    eventbrite_id   VARCHAR(100) UNIQUE,   -- Deduplication key from Eventbrite
    name            VARCHAR(255) NOT NULL,
    venue_name      VARCHAR(255),
    address         TEXT,
    city            VARCHAR(100),
    state           VARCHAR(50),
    zip_code        VARCHAR(20),

    -- DECIMAL(10, 6) — 6 decimal places for GPS precision (accurate to ~0.1 meters)
    latitude        DECIMAL(10, 6),
    longitude       DECIMAL(10, 6),

    start_date      TIMESTAMPTZ,
    end_date        TIMESTAMPTZ,
    event_url       TEXT,
    description     TEXT,
    fetched_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Index on start_date for "upcoming shows" query: WHERE start_date >= NOW()
CREATE INDEX IF NOT EXISTS idx_shows_start_date ON shows(start_date);

-- ============================================================
-- TRIGGER: Auto-update updated_at column
--
-- WHAT IS A TRIGGER?
-- A trigger is a function that PostgreSQL runs automatically when
-- a row is inserted, updated, or deleted.
--
-- FAANG PATTERN: updated_at tracking at DB level, not app level.
-- This means updated_at is ALWAYS accurate, even if someone runs
-- raw SQL directly on the database, bypassing your app.
--
-- PL/pgSQL = PostgreSQL's procedural language (like SQL + if/else/loops)
-- NEW = the row being updated (the new values before writing)
-- TRIGGER = returns TRIGGER, a special PostgreSQL type
-- $$ $$ = dollar-quoting for the function body (avoids escaping quotes)
-- ============================================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    -- Set updated_at to current time on every UPDATE
    NEW.updated_at = NOW();
    RETURN NEW;  -- Must return NEW to allow the UPDATE to proceed
END;
$$ LANGUAGE plpgsql;

-- Attach trigger to users table
-- BEFORE UPDATE = runs BEFORE the row is written (so we can modify NEW)
-- FOR EACH ROW = fires once per updated row (not once per statement)
CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Same trigger for inventory (purchase_price and current_value change over time)
CREATE TRIGGER update_inventory_updated_at
    BEFORE UPDATE ON inventory
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================
-- TABLE: cards
-- The central card registry — all known Pokémon cards with prices.
-- Written by Python analytics-engine, read by Go API handlers.
--
-- card_id = TCGplayer Product ID or custom internal ID
-- trending_score: -100 (sharply falling) to +100 (sharply rising)
--   This is computed by analytics-engine/analyzers/trend_analyzer.py
--   using linear regression on price_history data.
--
-- Why store prices from multiple marketplaces?
-- Vendors compare prices across eBay, TCGplayer, FB, Mercari
-- to find arbitrage opportunities (buy low on eBay, sell high on TCGplayer).
-- ============================================================
CREATE TABLE IF NOT EXISTS cards (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id         VARCHAR(100) UNIQUE NOT NULL,  -- Deduplication key

    name            VARCHAR(255) NOT NULL,
    set_name        VARCHAR(255),
    set_code        VARCHAR(20),   -- Short code like "BS" (Base Set), "SW" (Sword & Shield)
    image_url       TEXT,

    -- Trend data — computed by Python, stored here for fast reads
    trending_score  INTEGER DEFAULT 0,
    trend_label     VARCHAR(20) DEFAULT 'STABLE',  -- 'RISING', 'FALLING', 'STABLE'
    pct_change_7d   DECIMAL(8, 2),   -- Percentage price change over 7 days

    -- 7-day and 30-day moving averages — used by trend algorithm
    avg_price_7d    DECIMAL(10, 2),
    avg_price_30d   DECIMAL(10, 2),

    -- Current prices per marketplace (updated by Python api-consumer)
    price_ebay      DECIMAL(10, 2),
    price_tcgplayer DECIMAL(10, 2),
    price_facebook  DECIMAL(10, 2),
    price_mercari   DECIMAL(10, 2),

    last_updated    TIMESTAMPTZ DEFAULT NOW()
);

-- Index on name for case-insensitive search: WHERE name ILIKE '%charizard%'
CREATE INDEX IF NOT EXISTS idx_cards_name        ON cards(name);

-- Index on trend_label for: WHERE trend_label = 'RISING'
CREATE INDEX IF NOT EXISTS idx_cards_trend_label ON cards(trend_label);

-- Index on trending_score DESC — for ORDER BY trending_score DESC queries
CREATE INDEX IF NOT EXISTS idx_cards_trending_score ON cards(trending_score DESC);

-- ============================================================
-- TABLE: price_history
-- Daily price snapshots per card — powers the price chart on the dashboard.
-- Written by Python analytics-engine once per day.
-- Read by Go's handlers/card_handler.go → GET /api/cards/:id/history
--
-- FAANG PATTERN: Time-series data in its own table.
-- Don't store history as an array in the cards table — that doesn't scale.
-- One row per day per card is clean and indexable.
--
-- UNIQUE(card_id, date) = one row per card per day.
-- If analytics runs twice in a day, the second INSERT uses ON CONFLICT
-- to UPDATE the existing row instead of inserting a duplicate.
-- ============================================================
CREATE TABLE IF NOT EXISTS price_history (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id         VARCHAR(100) NOT NULL,  -- Matches cards.card_id
    date            DATE NOT NULL,          -- Just the date, not time
    avg_price       DECIMAL(10, 2),
    price_ebay      DECIMAL(10, 2),
    price_tcgplayer DECIMAL(10, 2),
    sale_count      INTEGER,               -- How many sales were observed that day
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(card_id, date)                  -- Idempotent inserts
);

CREATE INDEX IF NOT EXISTS idx_price_history_card_id ON price_history(card_id);
-- DESC index — we almost always want most recent prices first
CREATE INDEX IF NOT EXISTS idx_price_history_date    ON price_history(date DESC);

-- ============================================================
-- TABLE: card_listings
-- Raw marketplace listings (eBay, TCGplayer, Facebook, Mercari).
-- Published by Python api-consumer and Go scraping-service to RabbitMQ,
-- then written here by a separate consumer (future enhancement).
--
-- DEDUPLICATION: UNIQUE(marketplace, listing_id)
-- eBay listing ID "12345" should only be stored once from eBay.
-- ON CONFLICT DO NOTHING prevents duplicate INSERT errors.
--
-- This table grows fast — consider a retention policy:
-- DELETE FROM card_listings WHERE discovered_at < NOW() - INTERVAL '30 days'
-- ============================================================
CREATE TABLE IF NOT EXISTS card_listings (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    card_name       VARCHAR(255) NOT NULL,
    marketplace     VARCHAR(50) NOT NULL,   -- 'ebay', 'tcgplayer', 'facebook', 'mercari'
    price           DECIMAL(10, 2) NOT NULL,
    listing_url     TEXT,
    image_url       TEXT,
    seller          VARCHAR(255),
    condition       VARCHAR(50),            -- NM, LP, MP, HP, Damaged
    listing_id      VARCHAR(255),           -- The platform's own listing ID
    set_name        VARCHAR(255),
    location        VARCHAR(255),           -- For local FB/Mercari listings (city, state)
    discovered_at   TIMESTAMPTZ DEFAULT NOW(),

    -- Composite unique: prevents same listing from being stored twice
    UNIQUE(marketplace, listing_id)
);

-- Index on card_name — for "find all listings for Charizard" queries
CREATE INDEX IF NOT EXISTS idx_listings_card_name    ON card_listings(card_name);
CREATE INDEX IF NOT EXISTS idx_listings_marketplace  ON card_listings(marketplace);
-- DESC = most recent listings first (default sort order)
CREATE INDEX IF NOT EXISTS idx_listings_discovered   ON card_listings(discovered_at DESC);

-- ============================================================
-- TABLE: deals
-- "Deal of the Day" rows computed by Python's deal_finder.py.
-- A deal = listing price significantly below the 30-day average.
-- Written by Python at 6 AM daily (configurable via DEAL_OF_DAY_HOUR).
-- Read by Go's handlers/deal_handler.go → GET /api/deals/today
--
-- savings_pct = (market_price - best_price) / market_price * 100
-- A 20% deal on a $100 card = $20 savings. Significant.
-- ============================================================
CREATE TABLE IF NOT EXISTS deals (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deal_date       DATE NOT NULL,                   -- Which day this deal applies to
    card_name       VARCHAR(255) NOT NULL,
    set_name        VARCHAR(255),
    image_url       TEXT,
    market_price    DECIMAL(10, 2) NOT NULL,          -- 30-day average price
    best_price      DECIMAL(10, 2) NOT NULL,          -- Lowest active listing price
    savings         DECIMAL(10, 2),                   -- market_price - best_price
    savings_pct     DECIMAL(6, 2),                    -- (savings / market_price) * 100
    listing_url     TEXT,
    marketplace     VARCHAR(50),
    reason          TEXT,                             -- Human-readable explanation
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- DESC index — most recent deals first
CREATE INDEX IF NOT EXISTS idx_deals_deal_date ON deals(deal_date DESC);

-- ============================================================
-- TODO #1 (Practice): Add a "price_alerts_settings" table
-- This table would let users configure their alert preferences:
--   - minimum savings percentage to be notified about
--   - whether they want email notifications (future feature)
--   - daily digest vs immediate alerts
-- Think about: what columns it needs, what its foreign keys are,
-- and what constraints prevent bad data.
-- HINT: It should have a unique constraint on user_id (one row per user).
-- ============================================================

-- ============================================================
-- TODO #2 (Practice): Add a composite index for the most common query
-- The notification worker queries watchlists like this:
--   SELECT * FROM watchlists WHERE LOWER(card_name) = LOWER($1)
-- This currently can't use an index because of the LOWER() function.
-- Research: PostgreSQL FUNCTIONAL INDEXES
-- A functional index on LOWER(card_name) would make this query instant.
-- Look up: CREATE INDEX ON watchlists (LOWER(card_name));
-- ============================================================
