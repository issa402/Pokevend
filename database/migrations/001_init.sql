-- ============================================================
-- PokémonTool — PostgreSQL Initial Migration
-- File: 001_init.sql
-- Run automatically by Docker on first startup
-- ============================================================

-- Enable UUID generation extension
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================
-- USERS TABLE
-- Stores vendor accounts with hashed passwords
-- ============================================================
CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           VARCHAR(255) UNIQUE NOT NULL,
    password_hash   VARCHAR(255) NOT NULL,          -- bcrypt hash, never plaintext
    display_name    VARCHAR(100),
    zip_code        VARCHAR(20),                    -- Used for nearby show search
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- ============================================================
-- API_KEYS TABLE
-- Stores encrypted third-party API credentials per user
-- Keys are encrypted with AES-256 before insertion
-- ============================================================
CREATE TABLE IF NOT EXISTS api_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform        VARCHAR(50) NOT NULL,            -- 'ebay', 'tcgplayer', 'eventbrite'
    encrypted_key   TEXT NOT NULL,                  -- AES-256 encrypted value
    key_label       VARCHAR(100),                   -- User-friendly label e.g. "My eBay Key"
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, platform)                        -- One key per platform per user
);

-- ============================================================
-- WATCHLIST TABLE
-- Cards the vendor is actively monitoring for price changes
-- ============================================================
CREATE TABLE IF NOT EXISTS watchlists (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    card_name       VARCHAR(255) NOT NULL,
    set_name        VARCHAR(255),
    target_buy_price DECIMAL(10, 2),                -- Alert if price drops BELOW this
    target_sell_price DECIMAL(10, 2),               -- Alert if price rises ABOVE this
    notes           TEXT,
    added_at        TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_watchlists_user_id ON watchlists(user_id);

-- ============================================================
-- ALERTS TABLE
-- History of all triggered notifications for a user
-- ============================================================
CREATE TABLE IF NOT EXISTS alerts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    card_name       VARCHAR(255),
    alert_type      VARCHAR(50) NOT NULL,           -- 'PRICE_DROP', 'PRICE_SPIKE', 'NEW_LISTING', 'TREND_CHANGE', 'DEAL_OF_DAY'
    message         TEXT NOT NULL,
    marketplace     VARCHAR(50),                    -- 'ebay', 'tcgplayer', 'facebook', 'mercari'
    price           DECIMAL(10, 2),
    listing_url     TEXT,
    is_read         BOOLEAN DEFAULT FALSE,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alerts_user_id ON alerts(user_id);
CREATE INDEX IF NOT EXISTS idx_alerts_is_read ON alerts(is_read);

-- ============================================================
-- INVENTORY TABLE
-- Personal card collection owned by the vendor
-- ============================================================
CREATE TABLE IF NOT EXISTS inventory (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    card_name       VARCHAR(255) NOT NULL,
    set_name        VARCHAR(255),
    card_number     VARCHAR(50),                    -- e.g. "4/102"
    condition       VARCHAR(20),                    -- NM, LP, MP, HP, Damaged
    quantity        INTEGER DEFAULT 1,
    purchase_price  DECIMAL(10, 2),                 -- What you paid
    current_value   DECIMAL(10, 2),                 -- Last fetched market price
    notes           TEXT,
    acquired_at     TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_inventory_user_id ON inventory(user_id);

-- ============================================================
-- SHOWS TABLE
-- Upcoming Pokemon TCG events/shows fetched from Eventbrite
-- Cached here to avoid hammering the Eventbrite API
-- ============================================================
CREATE TABLE IF NOT EXISTS shows (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    eventbrite_id   VARCHAR(100) UNIQUE,
    name            VARCHAR(255) NOT NULL,
    venue_name      VARCHAR(255),
    address         TEXT,
    city            VARCHAR(100),
    state           VARCHAR(50),
    zip_code        VARCHAR(20),
    latitude        DECIMAL(10, 6),
    longitude       DECIMAL(10, 6),
    start_date      TIMESTAMPTZ,
    end_date        TIMESTAMPTZ,
    event_url       TEXT,
    description     TEXT,
    fetched_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_shows_start_date ON shows(start_date);

-- ============================================================
-- Auto-update 'updated_at' on row changes (trigger function)
-- ============================================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_inventory_updated_at
    BEFORE UPDATE ON inventory
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================================
-- CARDS TABLE
-- Central registry of known Pokemon cards.
-- Populated and updated by the Python analytics engine.
-- Replaces what was previously in MongoDB.
-- ============================================================
CREATE TABLE IF NOT EXISTS cards (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id         VARCHAR(100) UNIQUE NOT NULL,  -- TCGplayer product ID or custom ID
    name            VARCHAR(255) NOT NULL,
    set_name        VARCHAR(255),
    set_code        VARCHAR(20),
    image_url       TEXT,
    trending_score  INTEGER DEFAULT 0,              -- -100 (falling) to +100 (rising)
    trend_label     VARCHAR(20) DEFAULT 'STABLE',   -- 'RISING', 'FALLING', 'STABLE'
    pct_change_7d   DECIMAL(8, 2),                  -- % price change over 7 days
    avg_price_7d    DECIMAL(10, 2),                 -- 7-day average price
    avg_price_30d   DECIMAL(10, 2),                 -- 30-day average price
    price_ebay      DECIMAL(10, 2),                 -- Latest eBay sold price
    price_tcgplayer DECIMAL(10, 2),                 -- Latest TCGplayer market price
    price_facebook  DECIMAL(10, 2),                 -- Latest Facebook Marketplace price
    price_mercari   DECIMAL(10, 2),                 -- Latest Mercari price
    last_updated    TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cards_name        ON cards(name);
CREATE INDEX IF NOT EXISTS idx_cards_trend_label ON cards(trend_label);
CREATE INDEX IF NOT EXISTS idx_cards_trending_score ON cards(trending_score DESC);

-- ============================================================
-- PRICE_HISTORY TABLE
-- Daily price snapshots per card — powers the Chart.js charts.
-- Each row = one card on one date.
-- ============================================================
CREATE TABLE IF NOT EXISTS price_history (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id         VARCHAR(100) NOT NULL,           -- Matches cards.card_id
    date            DATE NOT NULL,
    avg_price       DECIMAL(10, 2),
    price_ebay      DECIMAL(10, 2),
    price_tcgplayer DECIMAL(10, 2),
    sale_count      INTEGER,                         -- Number of sales observed that day
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(card_id, date)                            -- One row per card per day
);

CREATE INDEX IF NOT EXISTS idx_price_history_card_id ON price_history(card_id);
CREATE INDEX IF NOT EXISTS idx_price_history_date    ON price_history(date DESC);

-- ============================================================
-- CARD_LISTINGS TABLE
-- Raw listings from all marketplaces (eBay, TCGplayer, FB, Mercari).
-- Published by Python and Go workers via RabbitMQ and written here.
-- Analytics engine reads this table to compute trends and deals.
-- ============================================================
CREATE TABLE IF NOT EXISTS card_listings (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    card_name       VARCHAR(255) NOT NULL,
    marketplace     VARCHAR(50) NOT NULL,            -- 'ebay', 'tcgplayer', 'facebook', 'mercari'
    price           DECIMAL(10, 2) NOT NULL,
    listing_url     TEXT,
    image_url       TEXT,
    seller          VARCHAR(255),
    condition       VARCHAR(50),
    listing_id      VARCHAR(255),                   -- Platform's own ID (for deduplication)
    set_name        VARCHAR(255),
    location        VARCHAR(255),                   -- For local FB/Mercari listings
    discovered_at   TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(marketplace, listing_id)                  -- Prevent duplicate listings
);

CREATE INDEX IF NOT EXISTS idx_listings_card_name    ON card_listings(card_name);
CREATE INDEX IF NOT EXISTS idx_listings_marketplace  ON card_listings(marketplace);
CREATE INDEX IF NOT EXISTS idx_listings_discovered   ON card_listings(discovered_at DESC);

-- ============================================================
-- DEALS TABLE
-- "Deal of the Day" rows written by the Python analytics engine.
-- The Node.js server reads from this table for the /api/deals endpoint.
-- ============================================================
CREATE TABLE IF NOT EXISTS deals (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deal_date       DATE NOT NULL,
    card_name       VARCHAR(255) NOT NULL,
    set_name        VARCHAR(255),
    image_url       TEXT,
    market_price    DECIMAL(10, 2) NOT NULL,         -- 30-day average price
    best_price      DECIMAL(10, 2) NOT NULL,         -- Lowest active listing price
    savings         DECIMAL(10, 2),                  -- market_price - best_price
    savings_pct     DECIMAL(6, 2),                   -- % below market average
    listing_url     TEXT,
    marketplace     VARCHAR(50),
    reason          TEXT,                            -- Human-readable explanation
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_deals_deal_date ON deals(deal_date DESC);

