ALTER TABLE card_listings
    ADD COLUMN IF NOT EXISTS language_preference VARCHAR(20) DEFAULT 'BOTH';

UPDATE card_listings
   SET language_preference = 'BOTH'
 WHERE language_preference IS NULL OR language_preference = '';

CREATE INDEX IF NOT EXISTS idx_card_listings_external_language_tier
    ON card_listings(external_card_id, language_preference, slab_tier, price, discovered_at DESC);
