-- ============================================================
-- FILE: database/migrations/003_watchlist_market_metadata.sql
-- TYPE: Migration — Add PokeTCG market metadata to watchlists
--
-- Adds fields needed to tie a watchlist item to an exact external card
-- variant and remember the market context used when the target was created.
-- Existing watchlist rows remain valid.
-- ============================================================

ALTER TABLE watchlists
    ADD COLUMN IF NOT EXISTS external_card_id TEXT,
    ADD COLUMN IF NOT EXISTS card_number TEXT,
    ADD COLUMN IF NOT EXISTS rarity TEXT,
    ADD COLUMN IF NOT EXISTS image_url TEXT,
    ADD COLUMN IF NOT EXISTS market_price DECIMAL(10, 2),
    ADD COLUMN IF NOT EXISTS market_updated_at TEXT,
    ADD COLUMN IF NOT EXISTS target_discount_pct DECIMAL(5, 2),
    ADD COLUMN IF NOT EXISTS price_source TEXT DEFAULT 'manual';

CREATE INDEX IF NOT EXISTS idx_watchlists_external_card_id
    ON watchlists(external_card_id)
    WHERE external_card_id IS NOT NULL;

