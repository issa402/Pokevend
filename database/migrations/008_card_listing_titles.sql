ALTER TABLE card_listings
    ADD COLUMN IF NOT EXISTS listing_title TEXT;

CREATE INDEX IF NOT EXISTS idx_card_listings_external_tier_price
    ON card_listings(external_card_id, slab_tier, price, discovered_at DESC);
