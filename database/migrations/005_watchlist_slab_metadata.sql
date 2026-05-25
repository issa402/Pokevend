ALTER TABLE watchlists
    ADD COLUMN IF NOT EXISTS asset_type VARCHAR(20) DEFAULT 'RAW',
    ADD COLUMN IF NOT EXISTS grader VARCHAR(20),
    ADD COLUMN IF NOT EXISTS grade VARCHAR(10),
    ADD COLUMN IF NOT EXISTS slab_tier VARCHAR(50);

UPDATE watchlists
   SET asset_type = 'RAW'
 WHERE asset_type IS NULL;

CREATE INDEX IF NOT EXISTS idx_watchlists_asset_match
    ON watchlists(asset_type, slab_tier, external_card_id);
