# ============================================================
# FILE: services/analytics-engine/repositories/card_repo.py
# TYPE: Repository Layer — PostgreSQL Queries for Analytics Engine
#
# WHAT IS THIS?
# All PostgreSQL queries used by the analytics engine live here.
# This is the SAME repository pattern as Go's store/card_store.go —
# just Python and psycopg2 instead of Go and pgx.
#
# FAANG CONSISTENCY: The pattern is the same across all services.
# Whether you're in Go, Python, or Java — repositories encapsulate
# all data access for a domain. New team members recognize the pattern instantly.
#
# PSYCOPG2 CONCEPTS:
#   psycopg2.connect()                     → connection to PostgreSQL
#   cursor_factory=RealDictCursor          → rows as dicts instead of tuples
#   cur.execute("SQL", (param,))           → parameterized query (safe)
#   cur.fetchall()                         → list of rows
#   conn.commit()                          → save changes to DB (required for writes)
#   "with conn:" / "with conn.cursor():"   → auto-rollback on error
#
# WHY NOT USE AN ORM (like SQLAlchemy)?
# ORMs generate SQL automatically — convenient but hides complexity.
# At FAANG, raw SQL is often preferred for analytics because:
#   - Full control over query plan and indexes
#   - EXPLAIN ANALYZE shows you exactly what runs
#   - ORM-generated SQL can be inefficient
# ============================================================
import logging
import os
from typing import Any, Dict, List, Optional

import psycopg2
import psycopg2.extras

logger = logging.getLogger(__name__)


def get_connection():
    """
    Create a new PostgreSQL connection using environment variables.
    
    IMPORTANT: Each function call creates a NEW connection.
    For analytics-engine (low frequency, batch jobs), this is fine.
    For high-traffic APIs (Go server), use a connection POOL (pgxpool in Go).
    
    psycopg2.connect() takes keyword arguments matching postgres DSN fields.
    All values come from environment variables — never hardcoded.
    """
    return psycopg2.connect(
        host=os.getenv("POSTGRES_HOST", "localhost"),
        port=os.getenv("POSTGRES_PORT", "5432"),
        dbname=os.getenv("POSTGRES_DB", "pokemontool"),
        user=os.getenv("POSTGRES_USER", "pokemontool_user"),
        password=os.getenv("POSTGRES_PASSWORD", "pokemontool_pass"),
    )


class CardRepo:
    """
    All PostgreSQL queries for card analytics.
    Methods grouped by operation type: reads and writes.
    
    PATTERN: Methods that read don't commit. Methods that write must commit.
    psycopg2 uses transactions automatically — every write needs conn.commit()
    or it gets rolled back when the connection closes.
    """

    def get_all_cards(self) -> List[Dict[str, Any]]:
        """
        Return all cards for analysis.
        
        CONTEXT MANAGER ("with" statement):
        "with psycopg2.connect() as conn:" automatically closes connection on exit.
        "with conn.cursor() as cur:" automatically closes cursor on exit.
        This prevents connection leaks — a common production bug.
        
        cursor_factory=psycopg2.extras.RealDictCursor:
        Returns rows as dicts: {"card_id": "ch-1", "name": "Charizard"}
        Without this: rows are tuples: ("ch-1", "Charizard") — harder to use
        """
        with get_connection() as conn:
            with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
                # cur.execute(sql, params) — always use %s for parameters, never f-strings
                # No parameters here: SELECT all cards
                cur.execute("SELECT card_id, name, set_name FROM cards ORDER BY name")
                # cur.fetchall() = returns all rows as a list of dicts
                return cur.fetchall()

    def get_price_history(self, card_id: str, days: int = 7) -> List[Dict[str, Any]]:
        """
        Return price history for a specific card over the last N days.
        
        PARAMETERIZED QUERY: %s placeholders, passed as a tuple (card_id, days)
        CRITICAL: NEVER use f-string/format() for SQL — SQL injection vulnerability!
        
          UNSAFE (NEVER DO THIS):
            cur.execute(f"SELECT ... WHERE card_id='{card_id}'")
            # If card_id = "'; DROP TABLE cards; --" → database gets wiped!
          
          SAFE (always do this):
            cur.execute("SELECT ... WHERE card_id=%s", (card_id,))
            # psycopg2 escapes the value properly
        
        Note the trailing comma in (card_id,) — this makes it a TUPLE, not a group.
        (card_id) without comma = just parentheses around a string.
        """
        with get_connection() as conn:
            with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
                cur.execute(
                    # INTERVAL: PostgreSQL-specific syntax for time arithmetic
                    # NOW() - INTERVAL '7 days' = all data from the last 7 days
                    """
                    SELECT date, avg_price, price_ebay, price_tcgplayer, sale_count
                    FROM price_history
                    WHERE card_id = %s
                      AND date >= NOW() - INTERVAL '%s days'
                    ORDER BY date ASC
                    """,
                    (card_id, days),  # (card_id, days) tuple → %s, %s in SQL
                )
                return cur.fetchall()

    def update_trend(self, card_id: str, label: str, score: int, pct_change: float):
        """
        Update a card's trend fields after analysis.
        
        COMMIT IS REQUIRED: After executing an UPDATE/INSERT/DELETE,
        you MUST call conn.commit() to persist the change.
        Without commit, the change is in a transaction that gets rolled back.
        
        last_updated=NOW():
        PostgreSQL's NOW() is evaluated server-side at query execution time.
        More reliable than setting it in Python (no timezone confusion).
        """
        with get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    UPDATE cards
                    SET trend_label = %s,
                        trending_score = %s,
                        pct_change_7d = %s,
                        last_updated = NOW()
                    WHERE card_id = %s
                    """,
                    (label, score, pct_change, card_id),
                )
            conn.commit()  # MUST commit writes!

    def upsert_deal(self, deal) -> None:
        """
        Insert a deal record for today, or skip if already exists.
        ON CONFLICT DO NOTHING = idempotent insert.
        
        IDEMPOTENCY: Running this function twice with the same input produces
        the same result — no duplicates, no errors.
        This is critical for analytics jobs that might run multiple times.
        
        The deal parameter accepts any Pydantic model with the required fields.
        Typed as Any to accept models.DealResult without importing it here.
        """
        with get_connection() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    INSERT INTO deals
                        (card_name, set_name, image_url, market_price, best_price,
                         savings, savings_pct, listing_url, marketplace, reason, deal_date)
                    VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                    ON CONFLICT DO NOTHING
                    """,
                    (
                        deal.card_name, deal.set_name, deal.image_url,
                        deal.market_price, deal.best_price, deal.savings,
                        deal.savings_pct, deal.listing_url, deal.marketplace,
                        deal.reason, deal.deal_date,
                    ),
                )
            conn.commit()

# ============================================================
# TODO #1 (Practice): Switch to a connection pool using psycopg2.pool
# Currently, each method call opens and closes a new connection.
# For frequent analytics runs, you'd use a connection pool:
#   from psycopg2 import pool
#   connection_pool = pool.SimpleConnectionPool(1, 10, **db_config)
# Then: conn = connection_pool.getconn() and connection_pool.putconn(conn)
# Research: psycopg2 SimpleConnectionPool vs ThreadedConnectionPool
# HINT: the ThreadedConnectionPool is thread-safe (needed for concurrent runs)

# TODO #2 (Practice): Add upsert_price_history method
# The analytics engine should write daily price snapshots.
# Add: upsert_price_history(card_id, date, avg_price, price_ebay, price_tcg, sale_count)
# SQL: INSERT INTO price_history ... ON CONFLICT (card_id, date) DO UPDATE SET ...
# This should be called by a new PriceHistoryAnalyzer after computing daily averages.
# Research: ON CONFLICT DO UPDATE vs ON CONFLICT DO NOTHING
# ============================================================
