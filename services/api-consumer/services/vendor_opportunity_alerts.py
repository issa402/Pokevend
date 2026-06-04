"""Create targeted alerts for watchlisted cards with evidence-backed exact listings."""

from __future__ import annotations

from repositories.slab_comp_repo import get_connection


def vendor_alert_message(
    card_name: str,
    slab_tier: str,
    asking_price: float,
    market_value: float,
    expected_profit: float,
    expected_margin_pct: float,
) -> str:
    return (
        f"Vendor opportunity: {card_name} {slab_tier} exact listing at ${asking_price:.2f}; "
        f"reference value ${market_value:.2f}, expected profit ${expected_profit:.2f}, "
        f"margin {expected_margin_pct:.1f}%. Matching sold-market evidence is available."
    )


def create_vendor_opportunity_alerts() -> int:
    created = 0
    with get_connection() as conn:
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT DISTINCT ON (w.user_id, o.marketplace, o.listing_id)
                       w.user_id, o.card_name, o.slab_tier, o.asking_price::float,
                       o.estimated_market_value::float, o.expected_profit::float,
                       o.expected_margin_pct::float, o.marketplace, o.listing_url, o.listing_id
                FROM slab_opportunities o
                JOIN watchlists w ON lower(w.card_name) = lower(o.card_name)
                LEFT JOIN LATERAL latest_seller_hub_metric(
                    o.external_card_id, o.card_name, o.slab_tier, 'SOLD'
                ) sold_metric ON TRUE
                WHERE o.marketplace = 'ebay'
                  AND o.listing_id IS NOT NULL
                  AND o.asking_price > 0
                  AND o.expected_profit > 0
                  AND o.expected_margin_pct >= 20
                  AND (
                      COALESCE(sold_metric.total_listings, 0) > 0
                      OR COALESCE(sold_metric.avg_listing_price, 0) > 0
                      OR COALESCE(sold_metric.avg_bids, 0) > 0
                  )
                ORDER BY w.user_id, o.marketplace, o.listing_id, o.deal_score DESC
                """
            )
            for row in cur.fetchall():
                user_id, card_name, slab_tier, asking, market_value, profit, margin, marketplace, listing_url, listing_id = row
                message = vendor_alert_message(card_name, slab_tier, asking, market_value, profit, margin)
                cur.execute(
                    """
                    INSERT INTO alerts
                        (user_id, card_name, alert_type, message, marketplace, price, listing_url, listing_id)
                    VALUES (%s, %s, 'VENDOR_OPPORTUNITY', %s, %s, %s, %s, %s)
                    ON CONFLICT (user_id, marketplace, listing_id)
                    WHERE listing_id IS NOT NULL AND listing_id <> ''
                    DO UPDATE SET
                        alert_type = EXCLUDED.alert_type,
                        message = EXCLUDED.message,
                        price = EXCLUDED.price,
                        listing_url = EXCLUDED.listing_url,
                        is_read = false,
                        created_at = NOW()
                    """,
                    (user_id, card_name, message, marketplace, asking, listing_url, listing_id),
                )
                created += 1
        conn.commit()
    return created
