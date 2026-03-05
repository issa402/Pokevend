# ============================================================
# analytics-engine/repositories/card_repo.py
# ALL PostgreSQL queries for card analytics — nothing else here.
# ============================================================
import logging
import os
from typing import List, Dict, Any, Optional

import psycopg2
import psycopg2.extras

logger = logging.getLogger(__name__)


def get_connection():
    return psycopg2.connect(
        host=os.getenv("POSTGRES_HOST", "localhost"),
        port=os.getenv("POSTGRES_PORT", "5432"),
        dbname=os.getenv("POSTGRES_DB", "pokemontool"),
        user=os.getenv("POSTGRES_USER", "pokemontool_user"),
        password=os.getenv("POSTGRES_PASSWORD", "pokemontool_pass"),
    )


class CardRepo:
    """All card-related PostgreSQL queries for the analytics engine."""

    def get_all_cards(self) -> List[Dict[str, Any]]:
        with get_connection() as conn:
            with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
                cur.execute("SELECT card_id, name, set_name FROM cards")
                return cur.fetchall()

    def get_price_history(self, card_id: str, days: int = 7) -> List[Dict[str, Any]]:
        with get_connection() as conn:
            with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
                cur.execute(
                    """SELECT date, avg_price, price_ebay, price_tcgplayer
                       FROM price_history
                       WHERE card_id=%s AND date >= NOW() - INTERVAL '%s days'
                       ORDER BY date ASC""",
                    (card_id, days),
                )
                return cur.fetchall()

    def update_trend(self, card_id: str, label: str, score: int, pct_change: float):
        with get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    """UPDATE cards SET trend_label=%s, trending_score=%s, pct_change_7d=%s, last_updated=NOW()
                       WHERE card_id=%s""",
                    (label, score, pct_change, card_id),
                )
            conn.commit()

    def upsert_deal(self, deal) -> None:
        with get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    """INSERT INTO deals
                       (card_name,set_name,image_url,market_price,best_price,savings,savings_pct,listing_url,marketplace,reason,deal_date)
                       VALUES (%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s)
                       ON CONFLICT DO NOTHING""",
                    (deal.card_name, deal.set_name, deal.image_url, deal.market_price,
                     deal.best_price, deal.savings, deal.savings_pct,
                     deal.listing_url, deal.marketplace, deal.reason, deal.deal_date),
                )
            conn.commit()
