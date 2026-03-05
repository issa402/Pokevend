# ============================================================
# analytics-engine/analyzers/deal_finder.py
# Business logic ONLY: identify below-market deals
# ============================================================
import logging
from typing import List

from models.schemas import DealResult
from repositories.card_repo import CardRepo
from repositories.listing_repo import ListingRepo

logger = logging.getLogger(__name__)

DEAL_THRESHOLD_PCT = 15.0  # A listing is a "deal" if it's 15%+ below market


class DealFinder:
    def __init__(self, card_repo: CardRepo, listing_repo: ListingRepo):
        self.card_repo    = card_repo
        self.listing_repo = listing_repo

    def run(self) -> List[DealResult]:
        """Find today's deals and persist them to the deals table."""
        raw_deals = self.listing_repo.get_below_market_listings(DEAL_THRESHOLD_PCT)
        results   = []
        for row in raw_deals:
            savings = row["market_price"] - row["price"]
            pct     = (savings / row["market_price"]) * 100
            deal = DealResult(
                card_name=row["card_name"],
                set_name=row.get("set_name"),
                image_url=row.get("image_url"),
                market_price=float(row["market_price"]),
                best_price=float(row["price"]),
                savings=round(savings, 2),
                savings_pct=round(pct, 1),
                listing_url=row["listing_url"],
                marketplace=row["marketplace"],
                reason=f"{pct:.0f}% below TCGplayer market price",
            )
            self.card_repo.upsert_deal(deal)
            results.append(deal)

        logger.info(f"Deal finder: found {len(results)} deals today")
        return results
