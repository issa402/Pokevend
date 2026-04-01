# ============================================================
# FILE: services/api-consumer/services/ebay_service.py
# TYPE: Service Layer — eBay Business Logic
#
# WHAT IS THIS?
# The eBay service is responsible for the business logic of
# fetching eBay listings and publishing them.
#
# SERVICE LAYER RULES (SAME AS GO):
#   ✅ Normalize/transform raw API data into domain models
#   ✅ Decide WHAT to publish (filter bad data)
#   ✅ Handle business validation (price > 0, etc.)
#   ✅ Log business-level events
#   ❌ NO direct eBay API calls (that's EbayRepo's job)
#   ❌ NO HTTP routes (that's api/webhook.py's job)
#   ❌ NO database queries (that's a repository's job)
#
# DATA FLOW:
#   EbayRepo.search_listings() → raw eBay JSON
#   EbayService.scan_card()    → normalize to CardListing + filter
#   RabbitMQPublisher.publish() → send to "listings" queue for Go
#
# PYTHON CONCEPTS:
#   async def, await, type hints (List[CardListing]), 
#   list comprehension, logging, try/except
# ============================================================
import logging
from datetime import datetime
from typing import List

from models.schemas import CardListing
from publisher.rabbitmq_publisher import RabbitMQPublisher
from repositories.ebay_repo import EbayRepo

# Logger scoped to this module — in logs you'll see "[api-consumer.services.ebay_service]"
logger = logging.getLogger(__name__)


class EbayService:
    """
    FAANG PATTERN: Service class with injected dependencies.
    Receives repo and publisher in __init__ — never creates them internally.
    This makes the service testable: pass mock_repo and mock_publisher in tests.
    
    Python DI is simpler than Go's: just pass objects to __init__.
    No frameworks needed (unlike Java's Spring @Autowired).
    """
    def __init__(self, repo: EbayRepo, publisher: RabbitMQPublisher):
        self.repo = repo            # the raw API caller
        self.publisher = publisher  # the RabbitMQ sender

    async def scan_card(self, card_name: str) -> List[CardListing]:
        """
        Main business method: search eBay → normalize → publish to RabbitMQ.
        
        Returns the list of CardListings found (used for logging/metrics).
        Side effect: each listing is published to RabbitMQ's "listings" queue.
        
        PYTHON TYPE HINTS: List[CardListing]
        This tells readers (and type checkers like mypy) that we return
        a list of CardListing objects (from models/schemas.py).
        Not enforced at runtime, but critical for maintainability at FAANG.
        """
        try:
            # await = suspend here while repo makes HTTP call to eBay API
            # Other coroutines can run during this suspension
            raw = await self.repo.search_listings(card_name)
        except Exception as e:
            # Catch ALL exceptions from the repo — network errors, auth failures, etc.
            # Return empty list (don't crash the scanner loop for one card)
            logger.error(f"eBay search failed for '{card_name}': {e}")
            raw = {"itemSummaries": []}

        listings: List[CardListing] = []


        test_listing = CardListing(
            card_name=card_name, 
            price =45.00,
            marketplace="ebay",
            listing_url="http://test-snipe.com",
            scraped_at=datetime.utcnow(),
        )
        listings.append(test_listing)

        # Iterate over eBay API response "itemSummaries" array
        for item in raw.get("itemSummaries", []):
            # dict.get(key, default): safe access — returns default if key missing
            # Nested: item.get("price", {}).get("value", 0) = safely access nested dict
            price = float(item.get("price", {}).get("value", 0))

            # Business rule: ignore listings with no price or zero price
            # These are likely auction items or data errors
            if price <= 0:
                continue  # skip this item, move to next

            # PYDANTIC MODEL CREATION: automatic validation
            # Pydantic raises ValidationError if types don't match
            # e.g., if price is a string "not a number", it fails here (caught by outer try)
            listing = CardListing(
                card_name=card_name,
                price=price,
                marketplace="ebay",
                listing_url=item.get("itemWebUrl", ""),
                image_url=item.get("image", {}).get("imageUrl"),  # None if not present
                condition=item.get("condition"),
                scraped_at=datetime.utcnow(),
            )
            listings.append(listing)
        if listings:
            for l in listings:
                payload = l.model_dump(mode = "json") 
                await self.publisher.publish("listings", payload)
        logger.info(f"eBay: published {len(listings)} listings for '{card_name}'")

            # Publish to RabbitMQ — Go's notification_worker.go consumes this
            # model_dump(mode="json") = serialize Pydantic model to JSON-compatible dict
            # datetime objects serialized as ISO 8601 strings
            

        
        return listings

    async def publish_batch(self, routing_key: str, messages: List[dict]):
        if not messages:
            return 
        try:
            await self.publisher.publish_batch(routing_key, messages)
            logeer.info(f"Count of messages {len(messages)}")
        except Exception as e:
            logger.error(f"Failed to publish batch to {routing_key}: {e}")

# ============================================================
# TODO #1 (Practice): Add price range filtering
# eBay returns listings across all prices — including $0.99 lots
# and $5000 PSA 10 graded cards.
# Add configuration: min_price and max_price (read from os.getenv)
# Filter listings outside this range before publishing.
# Business rule example: only track listings between $5 and $2000
# Add these as __init__ parameters with sensible defaults.

# TODO #2 (Practice): Add deduplication by listing ID
# eBay returns the same listing if you search multiple times
# (listings stay active for weeks). We'd publish duplicates to RabbitMQ.
# Fix: maintain a Python set of seen listing IDs (item["itemId"]).
# Before publishing, check if listing ID was already published in this session.
# For persistence across restarts: store seen IDs in Redis with a TTL of 24 hours.
# HINT: asyncio-safe Redis client = redis.asyncio (part of redis-py)
# ============================================================
