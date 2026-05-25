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
