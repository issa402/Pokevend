# ============================================================
# FILE: services/api-consumer/services/tcg_service.py
# TYPE: Service Layer — TCGplayer Business Logic
#
#REPLACING WITH OPEN SOURCE TCG ORIGINAL IS COMMENTED OUT IN THE BOTTOM
# WHAT IS THIS?
# Service for fetching and publishing TCGplayer card price data.
# Mirrors EbayService but for TCGplayer's API.
# Both services follow identical structure — same pattern, different API.
#
# FAANG PATTERN: Consistent structure across similar services.
# New engineers can look at ebay_service.py to understand tcg_service.py.
# Predictability reduces onboarding time dramatically.
#
# TCGplayer vs eBay:
#   eBay = auction/listing marketplace (prices vary wildly)
#   TCGplayer = fixed-price market with official market prices
#   TCGplayer is the GOLD STANDARD price reference for trading card games.
#   Our analytics-engine uses TCGplayer price as "market price" for deal comparison.
#
# TCGplayer API:
#   Endpoint: api.tcgplayer.com/v1.37.0/pricing/product/{productId}
#   Auth: OAuth2 Bearer Token (same flow as eBay)
#   Rate limit: 300 requests/minute (be careful — log/throttle)
# ============================================================

import logging
from typing import List
from datetime import datetime
from tcgdexsdk import TCGdex, Query

from models.schemas import CardListing
from publisher.rabbitmq_publisher import RabbitMQPublisher
from repositories.tcg_repo import TCGRepo 

logger = logging.getLogger(__name__)

class TCGService:
    def __init__(self, repo : TCGRepo, publisher: RabbitMQPublisher):
        self.repo = repo
        self.publisher = publisher 

    async def scan_card(self, card_name:str) -> List[CardListing]:
        try:
            data = await self.repo.get_market_price(card_name)
        except Exception as e:
            logger.error(f"Tcg Player Not working: {e}")
            return []

        listings: List[CardListing] = []

        for result in data.get("results", []):
            price = float(result.get("marketPrice") or 0)
            if price <= 0:
                continue 
            
            listing = CardListing(
                card_name= card_name,
                price=price, 
                marketplace="tcgplayer",
                listing_url= f"https://tcgplayer.com{result.get('productId', '')}",
                scraped_at=datetime.utcnow(),
            )
            listings.append(listing)

            await self.publisher.publish("listings", listing.model_dump(mode="json"))

            logger.info(f"TCGdex:Published {len(listing)} price for '{card_name}'")
            return listings






















# ============================================================
# ARCHIVED SERVICE: Original Logic (Commented Out)
# ============================================================
# class LegacyTCGService:
#     def __init__(self, repo: TCGRepo, publisher: RabbitMQPublisher):
#         self.repo = repo
#         self.publisher = publisher
#
#     async def scan_card(self, card_name: str) -> List[CardListing]:
#         try:
#             data = await self.repo.get_market_price(card_name)
#         except Exception as e:
#             logger.error(f"TCGplayer lookup failed for '{card_name}': {e}")
#             return []
#
#         listings: List[CardListing] = []
#         for result in data.get("results", []):
#             price = float(result.get("marketPrice") or 0)
#             if price <= 0: continue
#             listing = CardListing(
#                 card_name=card_name,
#                 price=price,
#                 marketplace="tcgplayer",
#                 listing_url=f"https://www.tcgplayer.com/product/{result.get('productId', '')}",
#                 scraped_at=datetime.utcnow(),
#             )
#             listings.append(listing)
#             await self.publisher.publish("listings", listing.model_dump(mode="json"))
#         return listings
# ============================================================
# TODO #1 (Practice): Add condition-based pricing
# TCGplayer has SEPARATE prices for NM, LP, HP conditions.
# Modify get_market_price to fetch all conditions:
#   resp.json()["results"] has fields: lowPrice, midPrice, highPrice, marketPrice
# Publish a separate CardListing for each condition with different prices.
# Set listing.condition = "NM" | "LP" | "HP" on each.
# Sellers can then find deals for the specific condition they need.

# TODO #2 (Practice): Cache TCGplayer market prices in Redis
# TCGplayer prices change slowly (daily/weekly). No need to fetch on every scan.
# Add a TCGplayer-specific Redis cache in this service:
#   redis_key = f"tcg_price:{card_name.lower()}"
#   Cache with TTL of 6 hours: if cached → return without API call
# Inject redis.asyncio.Redis into TCGService.__init__() alongside repo and publisher
# This dramatically reduces TCGplayer API calls (and avoids rate limiting).
# ============================================================
