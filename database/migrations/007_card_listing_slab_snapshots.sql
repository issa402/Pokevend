ALTER TABLE card_listings
    ADD COLUMN IF NOT EXISTS external_card_id VARCHAR(100),
    ADD COLUMN IF NOT EXISTS is_slab BOOLEAN DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS grader VARCHAR(20),
    ADD COLUMN IF NOT EXISTS grade VARCHAR(10),
    ADD COLUMN IF NOT EXISTS slab_tier VARCHAR(50);

CREATE INDEX IF NOT EXISTS idx_card_listings_external_card
    ON card_listings(external_card_id);

CREATE INDEX IF NOT EXISTS idx_card_listings_slab_summary
    ON card_listings(external_card_id, slab_tier, discovered_at DESC);
