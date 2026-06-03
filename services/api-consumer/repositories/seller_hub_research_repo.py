"""Persistence for authenticated Seller Hub research metrics."""

from __future__ import annotations

import json
from typing import Iterable

from repositories.slab_comp_repo import get_connection
from services.seller_hub_research import SellerHubResearchMetrics


class SellerHubResearchRepo:
    def upsert_many(self, metrics: Iterable[SellerHubResearchMetrics]) -> int:
        rows = [metric.to_record() for metric in metrics]
        if not rows:
            return 0
        with get_connection() as conn:
            with conn.cursor() as cur:
                for row in rows:
                    row = {**row, "top_rows_json": json.dumps(row["top_rows"], sort_keys=True)}
                    cur.execute(
                        """
                        INSERT INTO seller_hub_research_metrics
                            (external_card_id, card_name, set_name, card_number, language_preference,
                             slab_tier, tab_name, keywords, day_range, avg_listing_price,
                             min_listing_price, max_listing_price, avg_shipping_price, free_shipping_pct,
                             promoted_listing_pct, total_listings, avg_watchers, max_watchers,
                             avg_bids, max_bids, top_rows, source_url, researched_at, updated_at)
                        VALUES
                            (%(external_card_id)s, %(card_name)s, %(set_name)s, %(card_number)s, %(language_preference)s,
                             %(slab_tier)s, %(tab_name)s, %(keywords)s, %(day_range)s, %(avg_listing_price)s,
                             %(min_listing_price)s, %(max_listing_price)s, %(avg_shipping_price)s, %(free_shipping_pct)s,
                             %(promoted_listing_pct)s, %(total_listings)s, %(avg_watchers)s, %(max_watchers)s,
                             %(avg_bids)s, %(max_bids)s, %(top_rows_json)s::jsonb, %(source_url)s, %(researched_at)s, NOW())
                        ON CONFLICT (COALESCE(external_card_id, ''), card_name, COALESCE(set_name, ''), COALESCE(card_number, ''), language_preference, slab_tier, tab_name, day_range, keywords)
                        DO UPDATE SET
                            avg_listing_price = EXCLUDED.avg_listing_price,
                            min_listing_price = EXCLUDED.min_listing_price,
                            max_listing_price = EXCLUDED.max_listing_price,
                            avg_shipping_price = EXCLUDED.avg_shipping_price,
                            free_shipping_pct = EXCLUDED.free_shipping_pct,
                            promoted_listing_pct = EXCLUDED.promoted_listing_pct,
                            total_listings = EXCLUDED.total_listings,
                            avg_watchers = EXCLUDED.avg_watchers,
                            max_watchers = EXCLUDED.max_watchers,
                            avg_bids = EXCLUDED.avg_bids,
                            max_bids = EXCLUDED.max_bids,
                            top_rows = EXCLUDED.top_rows,
                            source_url = EXCLUDED.source_url,
                            researched_at = EXCLUDED.researched_at,
                            updated_at = NOW()
                        """,
                        row,
                    )
            conn.commit()
        return len(rows)
