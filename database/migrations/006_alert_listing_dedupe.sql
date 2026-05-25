ALTER TABLE alerts
    ADD COLUMN IF NOT EXISTS listing_id VARCHAR(255);

CREATE UNIQUE INDEX IF NOT EXISTS idx_alerts_user_listing_once
    ON alerts(user_id, marketplace, listing_id)
    WHERE listing_id IS NOT NULL AND listing_id <> '';
