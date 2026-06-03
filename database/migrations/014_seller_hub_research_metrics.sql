-- Seller Hub Product Research snapshots by card/grade/tab.
-- Authenticated browser automation writes here; no eBay credentials are stored.
CREATE TABLE IF NOT EXISTS seller_hub_research_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_card_id TEXT,
    card_name TEXT NOT NULL,
    set_name TEXT,
    card_number TEXT,
    language_preference TEXT DEFAULT 'BOTH',
    slab_tier TEXT NOT NULL,
    tab_name TEXT NOT NULL,
    keywords TEXT NOT NULL,
    day_range INTEGER NOT NULL DEFAULT 30,
    avg_listing_price NUMERIC(12,2),
    min_listing_price NUMERIC(12,2),
    max_listing_price NUMERIC(12,2),
    avg_shipping_price NUMERIC(12,2),
    free_shipping_pct NUMERIC(6,2),
    promoted_listing_pct NUMERIC(6,2),
    total_listings INTEGER,
    avg_watchers NUMERIC(10,2),
    max_watchers INTEGER,
    avg_bids NUMERIC(10,2),
    max_bids INTEGER,
    top_rows JSONB NOT NULL DEFAULT '[]'::jsonb,
    source_url TEXT,
    researched_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT seller_hub_research_metrics_tab_name_check CHECK (tab_name IN ('ACTIVE', 'SOLD'))
);

CREATE UNIQUE INDEX IF NOT EXISTS seller_hub_research_unique_snapshot
    ON seller_hub_research_metrics (COALESCE(external_card_id, ''), card_name, COALESCE(set_name, ''), COALESCE(card_number, ''), language_preference, slab_tier, tab_name, day_range, keywords);

CREATE INDEX IF NOT EXISTS seller_hub_research_card_idx
    ON seller_hub_research_metrics (external_card_id, card_name, slab_tier, tab_name, researched_at DESC);
