"""Automate authenticated Seller Hub research for the best exact Finder targets."""

from __future__ import annotations

import argparse
import asyncio
import json
import logging
from dataclasses import dataclass

from repositories.seller_hub_research_repo import SellerHubResearchRepo
from repositories.slab_comp_repo import get_connection
from services.live_slab_research import load_pokemon_env
from services.seller_hub_research import research_one_grade

logger = logging.getLogger("seller_hub_research_batch")


@dataclass(frozen=True)
class ResearchTarget:
    card_name: str
    set_name: str | None
    slab_tier: str
    external_card_id: str | None


def normalize_tabs(raw_tabs: str) -> list[str]:
    tabs = list(dict.fromkeys(tab.strip().upper() for tab in raw_tabs.split(",") if tab.strip()))
    unsupported = [tab for tab in tabs if tab not in {"ACTIVE", "SOLD"}]
    if unsupported:
        raise ValueError(f"unsupported Seller Hub tabs: {', '.join(unsupported)}")
    if not tabs:
        raise ValueError("at least one Seller Hub tab is required")
    return tabs


def load_targets(limit: int) -> list[ResearchTarget]:
    with get_connection() as conn:
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT card_name, MAX(set_name), slab_tier, MAX(external_card_id)
                FROM slab_opportunities
                WHERE marketplace = 'ebay'
                  AND listing_id IS NOT NULL
                  AND asking_price > 0
                  AND decision IN ('candidate', 'watch')
                  AND slab_tier IS NOT NULL
                GROUP BY card_name, slab_tier
                ORDER BY MAX(deal_score) DESC, MAX(updated_at) DESC
                LIMIT %s
                """,
                (max(1, min(limit, 100)),),
            )
            return [ResearchTarget(*row) for row in cur.fetchall()]


async def research_metric_with_retry(
    target: ResearchTarget,
    tab: str,
    day_range: int,
    headless: bool,
    attempts: int = 2,
):
    last_error: Exception | None = None
    for attempt in range(attempts):
        try:
            return await research_one_grade(
                external_card_id=target.external_card_id,
                card_name=target.card_name,
                set_name=target.set_name,
                card_number=None,
                language_preference="BOTH",
                slab_tier=target.slab_tier,
                tab_name=tab,
                day_range=day_range,
                headless=headless,
            )
        except Exception as exc:
            last_error = exc
            if attempt + 1 < attempts:
                await asyncio.sleep(3)
    assert last_error is not None
    raise last_error


async def research_cycle(limit: int, tabs: list[str], day_range: int, headless: bool) -> dict[str, int]:
    targets = load_targets(limit)
    repo = SellerHubResearchRepo()
    persisted = 0
    failed = 0
    for target in targets:
        for tab in tabs:
            try:
                metric = await research_metric_with_retry(target, tab, day_range, headless)
                persisted += repo.upsert_many([metric])
            except Exception as exc:
                failed += 1
                logger.warning("Seller Hub research failed for %s %s %s: %s", target.card_name, target.slab_tier, tab, exc)
            await asyncio.sleep(1)
    return {"targets": len(targets), "attempted": len(targets) * len(tabs), "persisted": persisted, "failed": failed}


async def run(args: argparse.Namespace) -> None:
    tabs = normalize_tabs(args.tabs)
    while True:
        result = await research_cycle(args.limit, tabs, args.day_range, not args.headful)
        print(json.dumps(result, sort_keys=True), flush=True)
        if args.interval_minutes <= 0:
            return
        await asyncio.sleep(args.interval_minutes * 60)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Automated authenticated Seller Hub research for exact Finder listings")
    parser.add_argument("--limit", type=int, default=10, help="Maximum unique card and grade targets per cycle")
    parser.add_argument("--tabs", default="ACTIVE,SOLD", help="Comma-separated Seller Hub tabs")
    parser.add_argument("--day-range", type=int, default=30)
    parser.add_argument("--interval-minutes", type=int, default=0, help="Repeat interval; zero runs one cycle")
    parser.add_argument("--headful", action="store_true", help="Show the browser while researching")
    return parser


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(name)s] %(levelname)s: %(message)s")
    load_pokemon_env()
    asyncio.run(run(build_parser().parse_args()))
