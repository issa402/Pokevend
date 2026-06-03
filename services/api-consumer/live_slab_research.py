#!/usr/bin/env python3
"""Run live slab buy research from PriceCharting + eBay."""

from __future__ import annotations

import argparse
import asyncio
import json
import sys

from repositories.live_slab_opportunity_repo import LiveSlabOpportunityRepo
from services.live_slab_research import run_live_research


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Find live slab buy candidates from trend data and active eBay asks")
    parser.add_argument("--mover-limit", type=int, default=10)
    parser.add_argument("--min-profit", type=float, default=25.0)
    parser.add_argument("--min-margin-pct", type=float, default=20.0)
    parser.add_argument("--target-config", help="Optional curated target JSON config")
    parser.add_argument("--no-sell-research", action="store_true", help="Only output BUY_CANDIDATE rows that meet thresholds")
    parser.add_argument("--persist", action="store_true", help="Write results into slab_opportunities")
    return parser.parse_args()


async def async_main() -> int:
    args = parse_args()
    candidates = await run_live_research(
        mover_limit=args.mover_limit,
        min_profit=args.min_profit,
        min_margin_pct=args.min_margin_pct,
        target_config=args.target_config,
        include_sell_research=not args.no_sell_research,
    )
    inserted = LiveSlabOpportunityRepo().upsert_many(candidates) if args.persist else 0
    print(json.dumps({"count": len(candidates), "persisted": inserted, "candidates": [candidate.to_dict() for candidate in candidates]}, indent=2, sort_keys=True))
    return 0


def main() -> int:
    return asyncio.run(async_main())


if __name__ == "__main__":
    sys.exit(main())
