"""
============================================================
PokémonTool — Market Scanner (Deal of the Day)
============================================================
Scans stored listing data to find today's best value opportunities:
  1. Pulls all listings from MongoDB collected in last 24h
  2. Compares each listing's price against the 30-day average
  3. Sorts by % below market and selects top deals
  4. Writes "Deal of the Day" document to MongoDB
  5. Identifies arbitrage: cheap on FB/Mercari, worth more on TCGplayer/eBay

Runs daily at 6 AM UTC via the scheduler in main.py.
============================================================
"""

import logging
from datetime import datetime, timedelta
from typing import List

log = logging.getLogger(__name__)


class MarketScanner:
    """Finds the best deals among today's listings vs historical averages."""

    def __init__(self, db):
        self.db          = db
        self.listings_col= db["listings"]     # Raw listings from all sources
        self.cards_col   = db["cards"]         # Card metadata with avg prices
        self.deals_col   = db["deals"]         # Daily deals output

    def run(self):
        """Compute today's Deal of the Day and store in MongoDB."""
        log.info("Computing Deal of the Day...")
        try:
            today  = datetime.utcnow().strftime("%Y-%m-%d")
            cutoff = datetime.utcnow() - timedelta(hours=24)

            # Pull listings found in the last 24 hours
            recent_listings = list(self.listings_col.find(
                {"discoveredAt": {"$gte": cutoff}},
                sort=[("discoveredAt", -1)]
            ))

            if not recent_listings:
                log.info("No recent listings found — skipping Deal of the Day")
                return

            deals = []
            for listing in recent_listings:
                deal = self._evaluate_listing(listing)
                if deal:
                    deals.append(deal)

            if not deals:
                log.info("No deals found today")
                return

            # Sort by savings percentage — best deals first
            deals.sort(key=lambda d: d["savingsPct"], reverse=True)
            top_deals = deals[:10]  # Keep top 10

            # Upsert today's deals document
            self.deals_col.update_one(
                {"date": today},
                {"$set": {
                    "date":        today,
                    "deals":       top_deals,
                    "generatedAt": datetime.utcnow(),
                }},
                upsert=True,
            )

            log.info(f"Deal of the Day: {len(top_deals)} deals saved for {today}")
        except Exception as e:
            log.error(f"Market scan failed: {e}", exc_info=True)

    def _evaluate_listing(self, listing: dict) -> dict:
        """
        Checks if a listing is below its card's 30-day average price.
        Returns a deal dict if the savings exceed 10%, else None.
        """
        card_name = listing.get("cardName", "")
        price     = float(listing.get("price", 0))
        if not card_name or price <= 0:
            return None

        # Look up the card's 30-day average from our analytics data
        card = self.cards_col.find_one(
            {"name": {"$regex": card_name, "$options": "i"}},
            {"avgPrice30d": 1, "name": 1, "imageUrl": 1}
        )

        if not card or not card.get("avgPrice30d"):
            return None

        market_price = float(card["avgPrice30d"])
        if market_price <= 0:
            return None

        savings     = market_price - price
        savings_pct = (savings / market_price) * 100

        # Only flag as a deal if it's at least 10% below market average
        if savings_pct < 10:
            return None

        # Additional check: is it listed on a cheaper marketplace (FB/Mercari)?
        marketplace = listing.get("marketplace", "")
        reason = f"Listed {savings_pct:.1f}% below 30-day average on {marketplace}"

        if marketplace in ("facebook", "mercari"):
            reason += " — potential arbitrage opportunity (list on eBay or TCGplayer for profit)"

        return {
            "cardName":    card.get("name", card_name),
            "imageUrl":    card.get("imageUrl", ""),
            "marketPrice": market_price,
            "bestPrice":   price,
            "savings":     round(savings, 2),
            "savingsPct":  round(savings_pct, 2),
            "listingUrl":  listing.get("listingUrl", ""),
            "marketplace": marketplace,
            "reason":      reason,
        }
