#!/usr/bin/env python3
"""Run all approved Scrapling sold-comp source configs."""

from __future__ import annotations

import argparse
import json
import sys

from services.scrapling_sold_comps_batch import run_batch


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Batch ingest sold slab comps from approved Scrapling sources")
    parser.add_argument("--config", required=True, help="Path to sold-comp source JSON config")
    parser.add_argument("--dry-run", action="store_true", help="Fetch and parse without writing to Postgres")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    print(json.dumps(run_batch(args.config, dry_run=args.dry_run), indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
