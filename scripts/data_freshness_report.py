#!/usr/bin/env python3
"""
data_freshness_report.py

This script answers a business-trust question:

Can Pokemon market data be trusted right now, or is it stale/missing?

Why this matters:
- A container can be running while its business data is old.
- "API is up" does not mean "recommendations are safe to trust."
- Finance-style platforms care about freshness because stale data can lead
  to bad decisions while the system still looks healthy.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from dataclasses import asdict, dataclass
from datetime import datetime, timezone
from typing import Any

import psycopg2
import psycopg2.extras
from dotenv import load_dotenv
from psycopg2 import sql


PROJECT_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
ENV_FILE = os.path.join(PROJECT_ROOT, ".env")
DEFAULT_HOST_MODE_POSTGRES_HOST = "localhost"


@dataclass
class FreshnessCheck:
    """One table freshness result."""

    table: str
    timestamp_column: str
    warning_minutes: int
    critical_minutes: int
    newest_seen: str | None
    age_minutes: float | None
    status: str
    business_risk: str


def parse_args() -> argparse.Namespace:
    """Parse command-line flags like --json."""
    parser = argparse.ArgumentParser(description="Pokemon data freshness report")
    parser.add_argument(
        "--json",
        action="store_true",
        help="Print machine-readable JSON instead of human-readable text.",
    )
    return parser.parse_args()


def load_environment() -> None:
    """Load database settings from Pokemon/.env."""
    load_dotenv(ENV_FILE)


def running_inside_docker() -> bool:
    """Return True when this script itself is running inside a container."""
    return os.path.exists("/.dockerenv")


def postgres_host() -> str:
    """
    Choose the correct Postgres hostname.

    Inside Docker Compose, "postgres" resolves through Docker DNS.
    From your Ubuntu host, use "localhost" because Compose publishes
    Postgres to 127.0.0.1:5432.
    """
    configured_host = os.getenv("POSTGRES_HOST", DEFAULT_HOST_MODE_POSTGRES_HOST)

    if configured_host == "postgres" and not running_inside_docker():
        return DEFAULT_HOST_MODE_POSTGRES_HOST

    return configured_host


def connect_db():
    """Connect to Pokemon PostgreSQL using environment variables."""
    host = postgres_host()
    port = os.getenv("POSTGRES_PORT", "5432")
    database = os.getenv("POSTGRES_DB", "pokemontool")
    url = (
        f"postgres://{os.getenv('POSTGRES_USER', 'pokemontool_user')}:"
        f"{os.getenv('POSTGRES_PASSWORD', 'pokemontool_pass')}@"
        f"{host}:"
        f"{port}/"
        f"{database}"
    )
    return psycopg2.connect(url, cursor_factory=psycopg2.extras.RealDictCursor)


def table_exists(conn, table: str) -> bool:
    """Return True when the target table exists in the public schema."""
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT EXISTS (
                SELECT 1
                FROM information_schema.tables
                WHERE table_schema = 'public'
                  AND table_name = %s
            ) AS exists
            """,
            (table,),
        )
        return bool(cur.fetchone()["exists"])


def newest_timestamp(conn, table: str, column: str) -> Any:
    """Return the newest timestamp in a table, or None if no rows exist."""
    with conn.cursor() as cur:
        query = sql.SQL("SELECT MAX({column}) AS newest_seen FROM {table}").format(
            column=sql.Identifier(column),
            table=sql.Identifier(table),
        )
        cur.execute(query)
        return cur.fetchone()["newest_seen"]


def freshness_status(age_minutes: float, warning_minutes: int, critical_minutes: int) -> str:
    """Turn age in minutes into fresh/warning/critical."""
    if age_minutes > critical_minutes:
        return "critical"

    if age_minutes > warning_minutes:
        return "warning"

    return "fresh"


def check_freshness(
    conn,
    table: str,
    column: str,
    warning_minutes: int,
    critical_minutes: int,
    business_risk: str,
) -> FreshnessCheck:
    """Check whether one table has recent enough data for business use."""
    if not table_exists(conn, table):
        return FreshnessCheck(
            table=table,
            timestamp_column=column,
            warning_minutes=warning_minutes,
            critical_minutes=critical_minutes,
            newest_seen=None,
            age_minutes=None,
            status="missing_table",
            business_risk=business_risk,
        )

    newest = newest_timestamp(conn, table, column)

    if newest is None:
        return FreshnessCheck(
            table=table,
            timestamp_column=column,
            warning_minutes=warning_minutes,
            critical_minutes=critical_minutes,
            newest_seen=None,
            age_minutes=None,
            status="no_data",
            business_risk=business_risk,
        )

    now = datetime.now(timezone.utc)

    if newest.tzinfo is None:
        newest = newest.replace(tzinfo=timezone.utc)

    age_minutes = (now - newest).total_seconds() / 60

    return FreshnessCheck(
        table=table,
        timestamp_column=column,
        warning_minutes=warning_minutes,
        critical_minutes=critical_minutes,
        newest_seen=newest.isoformat(),
        age_minutes=round(age_minutes, 2),
        status=freshness_status(age_minutes, warning_minutes, critical_minutes),
        business_risk=business_risk,
    )


def checks_to_run() -> list[tuple[str, str, int, int, str]]:
    """Define the data sources that support business decisions."""
    return [
        ("price_history", "created_at", 60, 180, "Pricing and trend decisions may use stale market data."),
        ("card_listings", "discovered_at", 60, 240, "Deal detection may miss current market opportunities."),
        ("alerts", "created_at", 1440, 2880, "User alerting may be inactive or unproven."),
        ("deals", "created_at", 1440, 2880, "Deal-of-the-day recommendations may be stale."),
        ("cards", "last_updated", 60, 180, "Current card prices and trend labels may be stale."),
    ]


def overall_status(results: list[FreshnessCheck]) -> str:
    """Summarize all table checks into one report-level status."""
    statuses = {result.status for result in results}

    if statuses & {"critical", "missing_table", "no_data"}:
        return "critical"

    if "warning" in statuses:
        return "warning"

    return "fresh"


def build_report(results: list[FreshnessCheck]) -> dict[str, Any]:
    """Build the machine-readable report used by JSON/API/dashboard paths."""
    return {
        "service": "pokemon",
        "check": "data_freshness",
        "overall_status": overall_status(results),
        "postgres_host_used": postgres_host(),
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "checks": [asdict(result) for result in results],
    }


def print_human_report(report: dict[str, Any]) -> None:
    """Print the report for a person reading the terminal."""
    print("Pokemon data freshness report")
    print(f"overall_status: {report['overall_status']}")
    print(f"postgres_host_used: {report['postgres_host_used']}")
    print()

    for check in report["checks"]:
        print(f"{check['table']}.{check['timestamp_column']}")
        print(f"  status: {check['status']}")
        print(f"  newest_seen: {check['newest_seen']}")
        print(f"  age_minutes: {check['age_minutes']}")
        print(f"  warning_minutes: {check['warning_minutes']}")
        print(f"  critical_minutes: {check['critical_minutes']}")
        print(f"  business_risk: {check['business_risk']}")
        print()


def main() -> int:
    """Run the full freshness report."""
    args = parse_args()
    load_environment()

    try:
        conn = connect_db()
    except Exception as exc:
        print(
            "Cannot connect to database "
            f"host={postgres_host()} "
            f"port={os.getenv('POSTGRES_PORT', '5432')} "
            f"db={os.getenv('POSTGRES_DB', 'pokemontool')}: "
            f"{exc!r}",
            file=sys.stderr,
        )
        return 1

    try:
        results = [
            check_freshness(conn, table, column, warning, critical, risk)
            for table, column, warning, critical, risk in checks_to_run()
        ]
    finally:
        conn.close()

    report = build_report(results)

    if args.json:
        print(json.dumps(report, indent=2))
    else:
        print_human_report(report)

    if report["overall_status"] in {"warning", "critical"}:
        return 2

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
