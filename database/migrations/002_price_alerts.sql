-- ============================================================
-- FILE: database/migrations/002_price_alerts.sql
-- TYPE: Migration — Add Price Alert Settings Table
--
-- WHAT IS THIS?
-- A user can set a price alert: "notify me when Charizard drops below $80."
-- This table stores those settings.
--
-- WHEN TO RUN:
--   docker exec -i pokemontool_postgres psql -U pokemontool_user -d pokemontool < database/migrations/002_price_alerts.sql
--
-- PRACTICE TASK #1 from practice_tasks.md
-- Fill in the CREATE TABLE, indexes, and test with \d price_alerts_settings
-- The commented-out solution is at the bottom of this file.
-- ============================================================

-- ── YOUR IMPLEMENTATION GOES HERE ─────────────────────────────
-- Schema to create:
CREATE TABLE IF NOT EXISTS price_alerts_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    card_name TEXT NOT NULL,
    threshold DECIMAL(10,2) NOT NULL,
    direction TEXT NOT NULL CHECK(direction IN ('BELOW', 'ABOVE')),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, card_name, direction)
)
--   price_alerts_settings table with:
--     id UUID PK, user_id FK, card_name TEXT, threshold DECIMAL,
--     direction TEXT ("BELOW"/"ABOVE"), is_active BOOL default true,
--     created_at TIMESTAMPTZ, UNIQUE(user_id, card_name, direction)
--
-- Two indexes to create:
--   1. idx_price_alerts_user ON (user_id)
--   2. idx_price_alerts_active — PARTIAL INDEX on (user_id, is_active) WHERE is_active = true
CREATE INDEX IF NOT EXISTS idx_price_alerts_active 
    ON price_alerts_settings(card_name, is_actice)
    WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_alerts_user_unread 
    ON alerts(user_id, is_read)
    WHERE is_read = false;


-- TODO: Write your CREATE TABLE and CREATE INDEX statements here
-- (delete this comment and put your SQL below)


-- ============================================================
-- SOLUTION (peek only when stuck):
-- ============================================================
-- CREATE TABLE IF NOT EXISTS price_alerts_settings (
--     id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
--     user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
--     card_name    TEXT NOT NULL,
--     threshold    DECIMAL(10,2) NOT NULL,
--     direction    TEXT NOT NULL CHECK(direction IN ('BELOW', 'ABOVE')),
--     is_active    BOOLEAN NOT NULL DEFAULT true,
--     created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
--     UNIQUE(user_id, card_name, direction)
-- );
--
-- CREATE INDEX IF NOT EXISTS idx_price_alerts_user
--     ON price_alerts_settings(user_id);
--
-- CREATE INDEX IF NOT EXISTS idx_price_alerts_active
--     ON price_alerts_settings(user_id, is_active)
--     WHERE is_active = true;
--
-- -- Verify:
-- -- SELECT table_name FROM information_schema.tables WHERE table_name = 'price_alerts_settings';
-- -- \d price_alerts_settings
