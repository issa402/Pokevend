-- Helper functions for attaching latest Seller Hub Product Research data to Finder rows.
-- They store no credentials; they only read seller_hub_research_metrics snapshots.
CREATE OR REPLACE FUNCTION latest_seller_hub_metric(
    p_external_card_id TEXT,
    p_card_name TEXT,
    p_slab_tier TEXT,
    p_tab_name TEXT
)
RETURNS SETOF seller_hub_research_metrics
LANGUAGE sql
STABLE
AS $$
    SELECT m.*
    FROM seller_hub_research_metrics m
    WHERE m.tab_name = p_tab_name
      AND m.slab_tier = COALESCE(p_slab_tier, '')
      AND (
          (NULLIF(p_external_card_id, '') IS NOT NULL AND m.external_card_id = NULLIF(p_external_card_id, ''))
          OR (NULLIF(p_external_card_id, '') IS NULL AND lower(m.card_name) = lower(p_card_name))
      )
    ORDER BY m.researched_at DESC
    LIMIT 1
$$;

CREATE OR REPLACE FUNCTION seller_hub_snapshot(metric seller_hub_research_metrics)
RETURNS jsonb
LANGUAGE sql
STABLE
AS $$
    SELECT CASE
        WHEN metric.id IS NULL THEN NULL
        ELSE jsonb_build_object(
            'keywords', metric.keywords,
            'tabName', metric.tab_name,
            'dayRange', metric.day_range,
            'avgListingPrice', metric.avg_listing_price,
            'minListingPrice', metric.min_listing_price,
            'maxListingPrice', metric.max_listing_price,
            'avgShippingPrice', metric.avg_shipping_price,
            'freeShippingPct', metric.free_shipping_pct,
            'promotedListingPct', metric.promoted_listing_pct,
            'totalListings', metric.total_listings,
            'avgWatchers', metric.avg_watchers,
            'maxWatchers', metric.max_watchers,
            'avgBids', metric.avg_bids,
            'maxBids', metric.max_bids,
            'sourceUrl', metric.source_url,
            'researchedAt', metric.researched_at,
            'topRows', metric.top_rows
        )
    END
$$;
