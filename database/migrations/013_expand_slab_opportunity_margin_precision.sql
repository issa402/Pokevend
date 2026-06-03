-- High-end slab flips can produce very large expected margin percentages when a live ask is far below PriceCharting reference value.
ALTER TABLE slab_opportunities
    ALTER COLUMN expected_margin_pct TYPE DECIMAL(10, 2);
