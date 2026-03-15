#!/usr/bin/env python3
"""
============================================================
FILE: scripts/seed_cards.py
TYPE: Automation Script — Database Seeder for Pokemon Cards
============================================================

WHAT IS THIS?
A standalone Python script that populates the cards table
with real Pokemon card data. Run it once to give the app
something to display.

WHAT THIS TEACHES:
  - Python scripting (not a web service — a one-shot script)
  - argparse: command-line argument parsing (standard library)
  - psycopg2: direct PostgreSQL connection from Python
  - sys.exit(): proper exit codes for scripts (0=success, 1=error)
  - __main__ guard: if __name__ == "__main__" pattern
  - Batch INSERT for efficiency

PYTHON SCRIPTING vs SERVICE:
  Service (FastAPI, analytics-engine/main.py):
    - Long-running process
    - Handles requests in a loop
    - Has startup/shutdown lifecycle
  Script (this file):
    - Runs once, exits
    - Takes arguments from command line
    - Returns an exit code (0=ok, 1=fail)
    - Called from Bash or CI/CD

HOW TO RUN:
  # Basic (seeds 50 popular cards)
  python3 scripts/seed_cards.py

  # Specify count
  python3 scripts/seed_cards.py --count 100

  # Verbose output
  python3 scripts/seed_cards.py --verbose

  # Connect to a different DB
  python3 scripts/seed_cards.py --db-url "postgres://user:pass@host:5432/db"

ARGPARSE: Python's built-in CLI argument library
  parser.add_argument("--count", type=int, default=50)
  args = parser.parse_args()
  args.count  # → 50 (or whatever the user passed)
  This is how all professional Python CLI tools work.
============================================================
"""

import argparse
import logging
import os
import sys

import psycopg2
import psycopg2.extras
from dotenv import load_dotenv

# Load .env from project root (script is in /scripts, .env is in /)
# os.path.dirname(__file__) = directory of THIS file (/scripts)
# os.path.join(..., "..") = go up one level (project root)
load_dotenv(os.path.join(os.path.dirname(__file__), "..", ".env"))

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
    datefmt="%H:%M:%S",
)
logger = logging.getLogger(__name__)

# ── SEED DATA ─────────────────────────────────────────────
# These are real cards from Base Set (set_name "Base Set", set code "base1")
# card_id format: {set_code}-{card_number} — consistent with TCGplayer
SEED_CARDS = [
    # (card_id, name, set_name, rarity, price_tcgplayer)
    ("base1-4",  "Charizard",    "Base Set", "Rare Holo",   350.00),
    ("base1-2",  "Blastoise",    "Base Set", "Rare Holo",   85.00),
    ("base1-15", "Venusaur",     "Base Set", "Rare Holo",   70.00),
    ("base1-25", "Pikachu",      "Base Set", "Common",      25.00),
    ("base1-6",  "Charizard",    "Base Set", "Rare Holo",   350.00),
    ("base2-4",  "Charizard",    "Jungle",   "Rare Holo",   95.00),
    ("xy1-1",    "Venusaur-EX",  "XY",       "Ultra Rare",  15.00),
    ("swsh1-20", "Zacian V",     "Sword & Shield", "Rare Holo V", 8.00),
    ("swsh1-18", "Zamazenta V",  "Sword & Shield", "Rare Holo V", 6.00),
    ("sm1-1",    "Decidueye-GX", "Sun & Moon",     "Ultra Rare",  5.00),
    ("base1-1",  "Alakazam",     "Base Set", "Rare Holo",   45.00),
    ("base1-3",  "Chansey",      "Base Set", "Rare Holo",   35.00),
    ("base1-5",  "Clefairy",     "Base Set", "Rare Holo",   55.00),
    ("base1-7",  "Computer Search","Base Set","Rare",        150.00),
    ("base1-16", "Zapdos",       "Base Set", "Rare Holo",   45.00),
    ("base1-17", "Electrode",    "Base Set", "Rare Holo",   25.00),
    ("base1-18", "Hitmonchan",   "Base Set", "Rare Holo",   30.00),
    ("base1-19", "Hypno",        "Base Set", "Rare Holo",   30.00),
    ("base1-20", "Kangaskhan",   "Base Set", "Rare Holo",   35.00),
    ("base1-21", "Lapras",       "Base Set", "Rare Holo",   60.00),
    ("base1-22", "Machamp",      "Base Set", "Rare Holo",   30.00),
    ("base1-23", "Magneton",     "Base Set", "Rare Holo",   25.00),
    ("base1-24", "Mewtwo",       "Base Set", "Rare Holo",   80.00),
    ("base1-26", "Raichu",       "Base Set", "Rare Holo",   40.00),
    ("base1-9",  "Venomoth",     "Base Set", "Rare Holo",   20.00),
    ("base1-10", "Vileplume",    "Base Set", "Rare Holo",   25.00),
    ("base1-11", "Jigglypuff",   "Base Set", "Common",      15.00),
    ("base1-12", "Poliwrath",    "Base Set", "Rare Holo",   30.00),
    ("base1-13", "Ninetales",    "Base Set", "Rare Holo",   50.00),
    ("base1-14", "Nidoking",     "Base Set", "Rare Holo",   35.00),
]


def get_db_connection(db_url: str) -> psycopg2.extensions.connection:
    """
    Connect to PostgreSQL using a connection URL.
    
    Connection URL format: postgres://user:password@host:port/dbname
    psycopg2 also accepts: keyword arguments (host=, port=, etc.)
    
    Both are valid — URL is more concise for scripts.
    """
    try:
        conn = psycopg2.connect(db_url)
        logger.info("✓ Connected to database")
        return conn
    except psycopg2.OperationalError as e:
        logger.error(f"Failed to connect to database: {e}")
        logger.error("Is the database running? Try: docker compose up -d postgres")
        sys.exit(1)  # exit code 1 = failure (Bash can check with: if ! python3 script.py; then ...)


def seed_cards(
    conn: psycopg2.extensions.connection,
    cards: list,
    verbose: bool = False
) -> int:
    """
    Insert cards into the database using UPSERT.
    Returns the count of cards inserted or updated.
    
    UPSERT = INSERT ... ON CONFLICT DO UPDATE
    Running this script twice won't duplicate cards — it updates them.
    This makes the script IDEMPOTENT (safe to run multiple times).
    
    Idempotency is critical for:
    - Scripts run as part of deployment (might run multiple times)
    - CI/CD pipelines (retry on failure)
    - Health checks that also seed initial data
    """
    SQL = """
        INSERT INTO cards (card_id, name, set_name, rarity, price_tcgplayer)
        VALUES %s
        ON CONFLICT (card_id) DO UPDATE SET
            name            = EXCLUDED.name,
            set_name        = EXCLUDED.set_name,
            rarity          = EXCLUDED.rarity,
            price_tcgplayer = EXCLUDED.price_tcgplayer,
            updated_at      = NOW()
    """
    # EXCLUDED.column = the value that WOULD have been inserted
    # This pattern enables "insert or update" atomically

    # Build the data as a list of tuples (psycopg2 batch format)
    data = [(c[0], c[1], c[2], c[3], c[4]) for c in cards]

    with conn.cursor() as cur:
        # execute_values: highly efficient batch INSERT (one round trip)
        # vs looping cur.execute(): N round trips to the DB (100x slower)
        psycopg2.extras.execute_values(cur, SQL, data, page_size=100)
        count = cur.rowcount
        conn.commit()  # REQUIRED: writes the transaction to disk

    if verbose:
        for card in cards:
            logger.info(f"  → {card[0]:15s} | {card[1]:25s} | ${card[4]:.2f}")

    return count


def main():
    """
    Main entry point for the script.
    
    Python __main__ pattern:
    if __name__ == "__main__": main()
    
    This runs main() only when executed directly (python3 script.py),
    NOT when imported by another module (import seed_cards).
    ALWAYS use this pattern in scripts so they're importable as modules.
    """
    # ── ARGPARSE: Define CLI arguments ──────────────────────
    parser = argparse.ArgumentParser(
        description="Seed the PokémonTool database with Pokemon card data",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  python3 scripts/seed_cards.py
  python3 scripts/seed_cards.py --count 20 --verbose
  python3 scripts/seed_cards.py --db-url "postgres://user:pass@localhost:5432/pokemontool"
        """,
    )
    parser.add_argument(
        "--count",
        type=int,
        default=len(SEED_CARDS),
        help=f"Number of cards to seed (default: {len(SEED_CARDS)}, max: {len(SEED_CARDS)})",
    )
    parser.add_argument(
        "--verbose", "-v",
        action="store_true",  # flag: present = True, absent = False
        help="Show each card being inserted",
    )
    parser.add_argument(
        "--db-url",
        default=os.getenv(
            "DATABASE_URL",
            f"postgres://{os.getenv('POSTGRES_USER','pokemontool_user')}:"
            f"{os.getenv('POSTGRES_PASSWORD','pokemontool_pass')}@"
            f"{os.getenv('POSTGRES_HOST','localhost')}:"
            f"{os.getenv('POSTGRES_PORT','5432')}/"
            f"{os.getenv('POSTGRES_DB','pokemontool')}"
        ),
        help="PostgreSQL connection URL",
    )
    args = parser.parse_args()

    # ── Validate arguments ─────────────────────────────────
    count = min(args.count, len(SEED_CARDS))
    cards_to_seed = SEED_CARDS[:count]

    logger.info(f"Seeding {count} cards into the database...")

    # ── Connect and seed ───────────────────────────────────
    conn = get_db_connection(args.db_url)
    try:
        inserted = seed_cards(conn, cards_to_seed, verbose=args.verbose)
        logger.info(f"✅ Done! {inserted} cards inserted/updated.")
        sys.exit(0)  # exit code 0 = success
    except Exception as e:
        logger.error(f"Seeding failed: {e}")
        sys.exit(1)
    finally:
        conn.close()  # always close the connection


if __name__ == "__main__":
    main()
