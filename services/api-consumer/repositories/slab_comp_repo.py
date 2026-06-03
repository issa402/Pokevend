"""PostgreSQL repository for sold slab comps collected by Scrapling."""

from __future__ import annotations

import os
from typing import Any, Iterable

import psycopg2
import psycopg2.extras


def get_connection():
    return psycopg2.connect(
        host=os.getenv("POSTGRES_HOST", "localhost"),
        port=os.getenv("POSTGRES_PORT", "5432"),
        dbname=os.getenv("POSTGRES_DB", "pokemontool"),
        user=os.getenv("POSTGRES_USER", "pokemontool_user"),
        password=os.getenv("POSTGRES_PASSWORD", "pokemontool_pass"),
    )


class SlabCompRepo:
    def upsert_many(self, comps: Iterable[dict[str, Any]]) -> int:
        rows = list(comps)
        if not rows:
            return 0
        with get_connection() as conn:
            with conn.cursor() as cur:
                psycopg2.extras.execute_batch(
                    cur,
                    """
                    INSERT INTO slab_comps
                        (external_card_id, card_name, set_name, language_preference,
                         grader, grade, slab_tier, cert_number, marketplace,
                         sold_price, shipping_price, sold_at, listing_url, title)
                    VALUES
                        (%(external_card_id)s, %(card_name)s, %(set_name)s, %(language_preference)s,
                         %(grader)s, %(grade)s, %(slab_tier)s, %(cert_number)s, %(marketplace)s,
                         %(sold_price)s, %(shipping_price)s, %(sold_at)s, %(listing_url)s, %(title)s)
                    """,
                    rows,
                )
            conn.commit()
        return len(rows)
