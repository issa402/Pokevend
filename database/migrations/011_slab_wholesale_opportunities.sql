-- ============================================================
-- FILE: database/migrations/011_slab_wholesale_opportunities.sql
-- TYPE: Migration — wholesale slab sourcing and flip opportunities
--
-- Purpose:
-- - Store sold slab comps separately from active listings.
-- - Store ranked wholesale slab opportunities with deterministic valuation.
-- - Add optional slab metadata to owned inventory so approved buys can flow
--   from opportunity -> inventory -> Odoo product sync.
-- ============================================================

ALTER TABLE inventory
    ADD COLUMN IF NOT EXISTS asset_type VARCHAR(20) DEFAULT 'RAW',
    ADD COLUMN IF NOT EXISTS grader VARCHAR(20),
    ADD COLUMN IF NOT EXISTS grade VARCHAR(10),
    ADD COLUMN IF NOT EXISTS slab_tier VARCHAR(50),
    ADD COLUMN IF NOT EXISTS cert_number VARCHAR(100),
    ADD COLUMN IF NOT EXISTS target_sale_price DECIMAL(10, 2);

CREATE INDEX IF NOT EXISTS idx_inventory_slab_identity
    ON inventory(external_card_id, slab_tier)
    WHERE external_card_id IS NOT NULL AND slab_tier IS NOT NULL;

CREATE TABLE IF NOT EXISTS slab_comps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_card_id TEXT,
    card_name VARCHAR(255) NOT NULL,
    set_name VARCHAR(255),
    language_preference VARCHAR(20) DEFAULT 'ANY',
    grader VARCHAR(20) NOT NULL,
    grade VARCHAR(10) NOT NULL,
    slab_tier VARCHAR(50) NOT NULL,
    cert_number VARCHAR(100),
    marketplace VARCHAR(50) NOT NULL,
    sold_price DECIMAL(10, 2) NOT NULL,
    shipping_price DECIMAL(10, 2) DEFAULT 0,
    sold_at TIMESTAMPTZ NOT NULL,
    listing_url TEXT,
    title TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_slab_comps_identity
    ON slab_comps(external_card_id, slab_tier, language_preference, sold_at DESC);

CREATE INDEX IF NOT EXISTS idx_slab_comps_name_tier
    ON slab_comps(LOWER(card_name), slab_tier, sold_at DESC);

CREATE TABLE IF NOT EXISTS slab_opportunities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_card_id TEXT,
    card_name VARCHAR(255) NOT NULL,
    set_name VARCHAR(255),
    grader VARCHAR(20),
    grade VARCHAR(10),
    slab_tier VARCHAR(50),
    marketplace VARCHAR(50),
    listing_id VARCHAR(255),
    listing_url TEXT,
    title TEXT,
    asking_price DECIMAL(10, 2) NOT NULL,
    shipping_price DECIMAL(10, 2) DEFAULT 0,
    estimated_fees DECIMAL(10, 2) DEFAULT 0,
    all_in_cost DECIMAL(10, 2) NOT NULL,
    estimated_market_value DECIMAL(10, 2) NOT NULL,
    expected_profit DECIMAL(10, 2) NOT NULL,
    expected_margin_pct DECIMAL(6, 2) NOT NULL,
    liquidity_score INTEGER NOT NULL,
    confidence_score INTEGER NOT NULL,
    risk_score INTEGER NOT NULL,
    deal_score INTEGER NOT NULL,
    decision VARCHAR(30) NOT NULL DEFAULT 'candidate',
    reason TEXT,
    evidence JSONB DEFAULT '{}'::jsonb,
    approved_inventory_id UUID REFERENCES inventory(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_slab_opportunities_listing
    ON slab_opportunities(marketplace, listing_id)
    WHERE listing_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_slab_opportunities_rank
    ON slab_opportunities(decision, deal_score DESC, expected_margin_pct DESC, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_slab_opportunities_identity
    ON slab_opportunities(external_card_id, slab_tier, decision);
