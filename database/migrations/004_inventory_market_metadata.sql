-- ============================================================
-- FILE: database/migrations/004_inventory_market_metadata.sql
-- TYPE: Migration — Add PokeTCG market metadata to inventory
--
-- Inventory already had current_value. These columns tie owned inventory
-- to exact external card variants and remember the market source.
-- ============================================================

ALTER TABLE inventory
    ADD COLUMN IF NOT EXISTS external_card_id TEXT,
    ADD COLUMN IF NOT EXISTS rarity TEXT,
    ADD COLUMN IF NOT EXISTS image_url TEXT,
    ADD COLUMN IF NOT EXISTS market_updated_at TEXT,
    ADD COLUMN IF NOT EXISTS price_source TEXT DEFAULT 'manual';

CREATE INDEX IF NOT EXISTS idx_inventory_external_card_id
    ON inventory(external_card_id)
    WHERE external_card_id IS NOT NULL;

