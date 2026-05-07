#!/usr/bin/env python3
"""
============================================================
FILE: scripts/db_report.py
TYPE: Automation Script — Database Health + Table Stats Report
============================================================

WHAT IS THIS?
Connects to PostgreSQL and prints a health report:
- Which tables exist
- Row counts per table
- Index usage stats
- Recent activity

WHAT THIS TEACHES:
  - Information schema: PostgreSQL's built-in metadata tables
  - WITH clauses (CTEs) in Python-embedded SQL
  - Tabular output with Python's tabulate-style formatting
  - Rich terminal output with ANSI color codes
  - Script composition: small reusable functions

PYTHON CONCEPTS:
  - f-strings with alignment: f"{name:<20}" = left-align in 20 chars
  - Dictionary comprehensions
  - enumerate(): loop with index
  - zip(): combine two lists

HOW TO RUN:
  python3 scripts/db_report.py
  python3 scripts/db_report.py --table alerts
============================================================
"""

import argparse
import logging
import os
import sys

import psycopg2
import psycopg2.extras
from dotenv import load_dotenv

load_dotenv(os.path.join(os.path.dirname(__file__), "..", ".env")) #gets me to the .env

# ANSI color codes for terminal output
# These escape codes tell the terminal to change text color
GREEN  = "\033[0;32m"
YELLOW = "\033[1;33m"
RED    = "\033[0;31m"
CYAN   = "\033[0;36m"
BOLD   = "\033[1m"
NC     = "\033[0m"  # No Color (reset)


def get_db_connection() -> psycopg2.extensions.connection:
    """Build a DB URL from environment variables and connect."""
    url = (
        f"postgres://{os.getenv('POSTGRES_USER','pokemontool_user')}:"
        f"{os.getenv('POSTGRES_PASSWORD','pokemontool_pass')}@"
        f"{os.getenv('POSTGRES_HOST','localhost')}:"
        f"{os.getenv('POSTGRES_PORT','5432')}/"
        f"{os.getenv('POSTGRES_DB','pokemontool')}"
    )
    try:
        return psycopg2.connect(url, cursor_factory=psycopg2.extras.RealDictCursor)
    except Exception as e:
        print(f"{RED}✗ Cannot connect to database: {e}{NC}")
        sys.exit(1)



def get_table_counts(conn) -> list:
    """
    Get row counts for all user tables.
    
    INFORMATION SCHEMA: PostgreSQL stores metadata about itself in
    special tables under the 'information_schema' schema.
    information_schema.tables = list of all tables in all schemas
    
    We use 'public' schema because that's where our app tables live.
    'pg_catalog' and 'information_schema' are system schemas — skip them.
    """
    with conn.cursor() as cur:
        # Get all our app tables
        cur.execute("""
            SELECT table_name
            FROM information_schema.tables
            WHERE table_schema = 'public'
              AND table_type = 'BASE TABLE'
            ORDER BY table_name
        """)
        tables = [row["table_name"] for row in cur.fetchall()]

        results = []
        for table in tables:
            # COUNT(*) each table individually
            # We can't do this in one query easily because each table is dynamic
            cur.execute(f"SELECT COUNT(*) AS count FROM {table}")
            count = cur.fetchone()["count"]
            results.append({"table": table, "rows": count})

        return results


def get_index_stats(conn) -> list:
    """
    Get index usage statistics from PostgreSQL internal stats.
    
    pg_stat_user_indexes: shows how often each index is actually used.
    idx_scan = number of times an index was used for a query.
    If idx_scan = 0 → index exists but is NEVER used → remove it (wastes space).
    """
    with conn.cursor() as cur:
        cur.execute("""
            SELECT
                indexrelname AS index_name,
                relname      AS table_name,
                idx_scan     AS times_used,
                pg_size_pretty(pg_relation_size(indexrelid)) AS size
            FROM pg_stat_user_indexes
            ORDER BY idx_scan DESC
            LIMIT 15
        """)
        return cur.fetchall()


def get_recent_alerts(conn, limit: int = 5) -> list:
    """Get the most recent alerts to show system activity."""
    with conn.cursor() as cur:
        try:
            cur.execute("""
                SELECT card_name, alert_type, is_read, created_at
                FROM alerts
                ORDER BY created_at DESC
                LIMIT %s
            """, (limit,))
            return cur.fetchall()
        except psycopg2.ProgrammingError:
            return []  # alerts table might not exist yet


def print_header(title: str) -> None:
    """Print a formatted section header."""
    print(f"\n{BOLD}{CYAN}{'─' * 50}{NC}")
    print(f"{BOLD}{CYAN}  {title}{NC}")
    print(f"{BOLD}{CYAN}{'─' * 50}{NC}")


def main():
    parser = argparse.ArgumentParser(description="PokémonTool Database Health Report")
    parser.add_argument("--table", help="Focus on a specific table")
    args = parser.parse_args()

    print(f"\n{BOLD}🔍 PokémonTool Database Report{NC}")

    conn = get_db_connection()
    print(f"{GREEN}✓ Connected to PostgreSQL{NC}")

    try:
        # ── Table Row Counts ──────────────────────────────────
        print_header("Table Row Counts")
        counts = get_table_counts(conn)

        if not counts:
            print(f"  {YELLOW}No tables found. Did you run the migration?{NC}")
            print(f"  Run: docker exec -i pokemontool_postgres psql -U pokemontool_user -d pokemontool < database/migrations/001_init.sql")
        else:
            # f"{val:<20}" = left-align val in a field 20 chars wide
            # f"{val:>8}"  = right-align val in a field 8 chars wide
            print(f"  {'Table':<30} {'Rows':>8}")
            print(f"  {'─'*30} {'─'*8}")
            for row in counts:
                color = GREEN if row["rows"] > 0 else YELLOW
                print(f"  {row['table']:<30} {color}{row['rows']:>8}{NC}")

        # ── Index Stats ───────────────────────────────────────
        print_header("Index Usage (top 15)")
        stats = get_index_stats(conn)
        if stats:
            print(f"  {'Index':<35} {'Table':<20} {'Times Used':>10} {'Size':>8}")
            print(f"  {'─'*35} {'─'*20} {'─'*10} {'─'*8}")
            for row in stats:
                color = RED if row["times_used"] == 0 else GREEN
                print(
                    f"  {row['index_name']:<35} "
                    f"{row['table_name']:<20} "
                    f"{color}{row['times_used']:>10}{NC} "
                    f"{row['size']:>8}"
                )
            print(f"\n  {YELLOW}⚠  Indexes with 0 uses may be unused — consider removing them{NC}")
        else:
            print(f"  {YELLOW}No indexes found (run migration first){NC}")

        # ── Recent Activity ───────────────────────────────────
        print_header("Recent Alerts")
        alerts = get_recent_alerts(conn)
        if alerts:
            for a in alerts:
                status = f"{GREEN}read{NC}" if a["is_read"] else f"{YELLOW}UNREAD{NC}"
                print(f"  [{status}] {a['alert_type']:<15} {a['card_name']:<20} {a['created_at']}")
        else:
            print(f"  {YELLOW}No alerts yet. Scanner needs to run first.{NC}")

        print(f"\n{GREEN}✅ Report complete{NC}\n")

    finally:
        conn.close()


if __name__ == "__main__":
    main()
