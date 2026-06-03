"""CLI for authenticated eBay Seller Hub Product Research."""

from __future__ import annotations

import argparse
import asyncio
import json

from dotenv import load_dotenv

from repositories.seller_hub_research_repo import SellerHubResearchRepo
from services.seller_hub_research import grade_tiers, open_login_browser, research_one_grade, seller_hub_profile_dir


def main() -> None:
    load_dotenv()
    parser = argparse.ArgumentParser(description="Authenticated eBay Seller Hub Product Research")
    sub = parser.add_subparsers(dest="command", required=True)

    sub.add_parser("login", help="Open a persistent local browser profile for eBay login")

    run = sub.add_parser("run", help="Run Seller Hub research for one card")
    run.add_argument("--card-name", required=True)
    run.add_argument("--external-card-id")
    run.add_argument("--set-name")
    run.add_argument("--card-number")
    run.add_argument("--language", default="BOTH")
    run.add_argument("--tab", choices=["ACTIVE", "SOLD"], default="ACTIVE")
    run.add_argument("--day-range", type=int, default=30)
    run.add_argument("--slab-tier", action="append", dest="slab_tiers")
    run.add_argument("--grade-limit", type=int, default=None)
    run.add_argument("--headful", action="store_true")
    run.add_argument("--persist", action="store_true")

    args = parser.parse_args()
    if args.command == "login":
        asyncio.run(open_login_browser(seller_hub_profile_dir()))
        return

    tiers = args.slab_tiers or grade_tiers(args.grade_limit)
    metrics = []
    for tier in tiers:
        metrics.append(asyncio.run(research_one_grade(
            external_card_id=args.external_card_id,
            card_name=args.card_name,
            set_name=args.set_name,
            card_number=args.card_number,
            language_preference=args.language,
            slab_tier=tier,
            tab_name=args.tab,
            day_range=args.day_range,
            headless=not args.headful,
        )))
    inserted = SellerHubResearchRepo().upsert_many(metrics) if args.persist else 0
    print(json.dumps({"count": len(metrics), "persisted": inserted, "metrics": [metric.to_record() for metric in metrics]}, indent=2, default=str))


if __name__ == "__main__":
    main()
