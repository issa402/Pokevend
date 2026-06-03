-- ============================================================
-- FILE: database/migrations/012_inventory_store_listing_status.sql
-- TYPE: Migration — explicit inventory-to-store publish gate
--
-- Purpose:
-- - Keep PokemonTool inventory as the source of truth.
-- - Let users choose which inventory rows are ready for Odoo/store sync.
-- - Avoid auto-publishing every owned card.
-- ============================================================

ALTER TABLE inventory
    ADD COLUMN IF NOT EXISTS store_listing_status VARCHAR(30) DEFAULT 'NOT_LISTED',
    ADD COLUMN IF NOT EXISTS store_price DECIMAL(10, 2),
    ADD COLUMN IF NOT EXISTS store_listing_notes TEXT,
    ADD COLUMN IF NOT EXISTS store_listed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS store_synced_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_inventory_store_listing_status
    ON inventory(store_listing_status, updated_at DESC);
