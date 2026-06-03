#!/usr/bin/env python3
"""Ingest sold slab comps from an operator-approved page using Scrapling.

Use this only on sources you are allowed to access. Prefer official APIs where
available, respect robots.txt/terms/rate limits, and keep credentials out of
command history and git-tracked files.
"""

from __future__ import annotations

import argparse
import json
import sys
from dataclasses import asdict

from repositories.slab_comp_repo import SlabCompRepo
from services.scrapling_sold_comps import (
    ScraplingSoldCompExtractor,
    SoldCompContext,
    SoldCompSelectorConfig,
)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Scrape and ingest sold slab comps with Scrapling")
    parser.add_argument("--url", required=True)
    parser.add_argument("--marketplace", required=True, help="Source name, for example ebay_sold or pricecharting")
    parser.add_argument("--card-name", required=True)
    parser.add_argument("--set-name")
    parser.add_argument("--external-card-id")
    parser.add_argument("--grader", required=True, choices=["PSA", "CGC", "BGS", "SGC", "TAG", "ACE"])
    parser.add_argument("--grade", required=True)
    parser.add_argument("--slab-tier", required=True, help="Example: PSA_10, CGC_9_5, BGS_10")
    parser.add_argument("--language-preference", default="ANY")
    parser.add_argument("--row-selector", required=True)
    parser.add_argument("--title-selector", required=True)
    parser.add_argument("--price-selector", required=True)
    parser.add_argument("--sold-at-selector")
    parser.add_argument("--link-selector")
    parser.add_argument("--dry-run", action="store_true")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    config = SoldCompSelectorConfig(
        row_selector=args.row_selector,
        title_selector=args.title_selector,
        price_selector=args.price_selector,
        sold_at_selector=args.sold_at_selector,
        link_selector=args.link_selector,
    )
    context = SoldCompContext(
        card_name=args.card_name,
        set_name=args.set_name,
        external_card_id=args.external_card_id,
        grader=args.grader,
        grade=args.grade,
        slab_tier=args.slab_tier,
        marketplace=args.marketplace,
        language_preference=args.language_preference,
    )
    comps = ScraplingSoldCompExtractor().fetch(args.url, config, context)
    serializable = [
        {key: (value.isoformat() if hasattr(value, "isoformat") else value) for key, value in comp.items()}
        for comp in comps
    ]
    if args.dry_run:
        print(json.dumps({"count": len(comps), "comps": serializable}, indent=2, sort_keys=True))
        return 0
    inserted = SlabCompRepo().upsert_many(comps)
    print(json.dumps({"inserted": inserted}, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
