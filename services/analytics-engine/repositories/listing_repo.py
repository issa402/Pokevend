# ============================================================
# analytics-engine/repositories/listing_repo.py
# Queries for card listings — raw marketplace data in PostgreSQL
# ============================================================
import logging
from typing import List, Dict, Any

import psycopg2
import psycopg2.extras
from repositories.card_repo import get_connection

logger = logging.getLogger(__name__)


class ListingRepo:
    """Queries for reading card_listings table."""

    def get_listings_by_card(self, card_name: str, limit: int = 100) -> List[Dict[str, Any]]:
        with get_connection() as conn:
            with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
                cur.execute(
                    """SELECT price, marketplace, listing_url, condition, discovered_at
                       FROM card_listings WHERE LOWER(card_name)=LOWER(%s)
                       ORDER BY discovered_at DESC LIMIT %s""",
                    (card_name, limit),
                )
                return cur.fetchall()

    def get_below_market_listings(self, threshold_pct: float = 20.0) -> List[Dict[str, Any]]:
        """Find listings priced significantly below card market price — deal candidates."""
        with get_connection() as conn:
            with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
                cur.execute(
                    """SELECT cl.card_name, cl.set_name, cl.price, cl.marketplace,
                              cl.listing_url, cl.image_url,
                              c.price_tcgplayer AS market_price
                       FROM card_listings cl
                       JOIN cards c ON LOWER(cl.card_name)=LOWER(c.name)
                       WHERE c.price_tcgplayer IS NOT NULL
                         AND cl.price < c.price_tcgplayer * ((100 - %s) / 100.0)
                         AND cl.discovered_at > NOW() - INTERVAL '24 hours'
                       ORDER BY ((c.price_tcgplayer - cl.price) / c.price_tcgplayer) DESC
                       LIMIT 20""",
                    (threshold_pct,),
                )
                return cur.fetchall()

    def get_recent_slab_listings(self, hours: int = 24, limit: int = 200) -> List[Dict[str, Any]]:
        """Return recent active slab listings for wholesale opportunity scoring."""
        with get_connection() as conn:
            with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
                cur.execute(
                    """
                    SELECT card_name, set_name, external_card_id, marketplace, price,
                           listing_url, image_url, condition, listing_id, discovered_at,
                           is_slab, grader, grade, slab_tier, listing_title
                    FROM card_listings
                    WHERE is_slab = TRUE
                      AND price IS NOT NULL
                      AND slab_tier IS NOT NULL
                      AND discovered_at > NOW() - (%s::text || ' hours')::interval
                    ORDER BY discovered_at DESC
                    LIMIT %s
                    """,
                    (hours, limit),
                )
                return cur.fetchall()

    def get_slab_comps(self, external_card_id: str | None, card_name: str, slab_tier: str, days: int = 90) -> List[Dict[str, Any]]:
        """Return sold comps for the same card/slab tier."""
        with get_connection() as conn:
            with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
                cur.execute(
                    """
                    SELECT sold_price, shipping_price, sold_at, marketplace, listing_url, title
                    FROM slab_comps
                    WHERE slab_tier = %s
                      AND sold_at > NOW() - (%s::text || ' days')::interval
                      AND (
                        (%s IS NOT NULL AND external_card_id = %s)
                        OR LOWER(card_name) = LOWER(%s)
                      )
                    ORDER BY sold_at DESC
                    LIMIT 50
                    """,
                    (slab_tier, days, external_card_id, external_card_id, card_name),
                )
                return cur.fetchall()

    def upsert_slab_opportunity(self, opportunity: Dict[str, Any]) -> None:
        """Create/update one scored slab opportunity by marketplace listing ID."""
        with get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    INSERT INTO slab_opportunities
                        (external_card_id, card_name, set_name, grader, grade, slab_tier,
                         marketplace, listing_id, listing_url, title, asking_price,
                         shipping_price, estimated_fees, all_in_cost, estimated_market_value,
                         expected_profit, expected_margin_pct, liquidity_score, confidence_score,
                         risk_score, deal_score, decision, reason, evidence, updated_at)
                    VALUES
                        (%(external_card_id)s, %(card_name)s, %(set_name)s, %(grader)s, %(grade)s, %(slab_tier)s,
                         %(marketplace)s, %(listing_id)s, %(listing_url)s, %(title)s, %(asking_price)s,
                         %(shipping_price)s, %(estimated_fees)s, %(all_in_cost)s, %(estimated_market_value)s,
                         %(expected_profit)s, %(expected_margin_pct)s, %(liquidity_score)s, %(confidence_score)s,
                         %(risk_score)s, %(deal_score)s, %(decision)s, %(reason)s, %(evidence)s::jsonb, NOW())
                    ON CONFLICT (marketplace, listing_id) WHERE listing_id IS NOT NULL
                    DO UPDATE SET
                        asking_price = EXCLUDED.asking_price,
                        shipping_price = EXCLUDED.shipping_price,
                        estimated_fees = EXCLUDED.estimated_fees,
                        all_in_cost = EXCLUDED.all_in_cost,
                        estimated_market_value = EXCLUDED.estimated_market_value,
                        expected_profit = EXCLUDED.expected_profit,
                        expected_margin_pct = EXCLUDED.expected_margin_pct,
                        liquidity_score = EXCLUDED.liquidity_score,
                        confidence_score = EXCLUDED.confidence_score,
                        risk_score = EXCLUDED.risk_score,
                        deal_score = EXCLUDED.deal_score,
                        decision = CASE
                            WHEN slab_opportunities.decision IN ('approved', 'rejected') THEN slab_opportunities.decision
                            ELSE EXCLUDED.decision
                        END,
                        reason = EXCLUDED.reason,
                        evidence = EXCLUDED.evidence,
                        updated_at = NOW()
                    """,
                    opportunity,
                )
            conn.commit()
