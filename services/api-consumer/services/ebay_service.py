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
import re
from datetime import datetime
from typing import List, Optional

from models.schemas import CardListing
from publisher.rabbitmq_publisher import RabbitMQPublisher
from repositories.ebay_repo import EbayRepo
from services.slab_parser import parse_slab

# Logger scoped to this module — in logs you'll see "[api-consumer.services.ebay_service]"
logger = logging.getLogger(__name__)

BLOCKED_TITLE_TERMS = (
    "proxy", "custom", "reprint", "digital", "sticker", "empty slab",
    "case only", "stand only", "display case", "mystery pack", "booster pack",
    "code card", "jumbo", "oversized", "coin", "sleeve", "toploader",
    "fan art", "acrylic", "deck", "collection box", "booster box",
    "sealed box", "sealed pack", "3 boxes",
)


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

    async def scan_card(
        self,
        card_name: str,
        external_card_id: Optional[str] = None,
        set_name: Optional[str] = None,
        asset_type: str = "RAW",
        slab_tier: Optional[str] = None,
        publish: bool = True,
        language_preference: str = "BOTH",
        max_pages: int = 1,
    ) -> List[CardListing]:
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
            query_parts = [card_name]
            if set_name:
                query_parts.append(set_name)
            slab_query = _slab_query(slab_tier) if asset_type == "SLAB" else ""
            if slab_query:
                query_parts.append(slab_query)
            language = _normalize_language(language_preference)
            language_query = _language_query(language)
            if language_query:
                query_parts.append(language_query)
            query = " ".join(query_parts).strip()
            raw = await self.repo.search_listings(query, limit=200, max_pages=max_pages)
        except Exception as e:
            # Catch ALL exceptions from the repo — network errors, auth failures, etc.
            # Return empty list (don't crash the scanner loop for one card)
            logger.error(f"eBay search failed for '{card_name}': {e}")
            raw = {"itemSummaries": []}

        listings: List[CardListing] = []

        # Iterate over eBay API response "itemSummaries" array
        for item in raw.get("itemSummaries", []):
            title = item.get("title", "") or ""
            if not _looks_like_target_card(title, card_name):
                continue
            if not _matches_language(title, language):
                continue
            if _blocked_listing(title):
                continue

            # dict.get(key, default): safe access — returns default if key missing
            # Nested: item.get("price", {}).get("value", 0) = safely access nested dict
            price = float(item.get("price", {}).get("value", 0))

            # Business rule: ignore listings with no price or zero price
            # These are likely auction items or data errors
            if price <= 0:
                continue  # skip this item, move to next

            slab = parse_slab(item.get("title", ""), item.get("condition"))
            if asset_type == "RAW" and slab["is_slab"]:
                continue
            if asset_type == "SLAB":
                if not slab["is_slab"]:
                    continue
                if slab_tier and slab["slab_tier"] != slab_tier:
                    continue

            # PYDANTIC MODEL CREATION: automatic validation
            # Pydantic raises ValidationError if types don't match
            # e.g., if price is a string "not a number", it fails here (caught by outer try)
            listing = CardListing(
                card_name=card_name,
                external_card_id=external_card_id,
                price=price,
                marketplace="ebay",
                listing_url=item.get("itemWebUrl", ""),
                listing_id=item.get("itemId"),
                listing_title=title,
                image_url=item.get("image", {}).get("imageUrl"),  # None if not present
                condition=item.get("condition"),
                set_name=set_name,
                is_slab=slab["is_slab"],
                grader=slab["grader"],
                grade=slab["grade"],
                slab_tier=slab["slab_tier"],
                cert_number=slab["cert_number"],
                language_preference=language,
                scraped_at=datetime.utcnow(),
            )
            listings.append(listing)
        if publish and listings:
            for l in listings:
                payload = l.model_dump(mode = "json") 
                await self.publisher.publish("listings", payload)
        logger.info(f"eBay: published {len(listings)} listings for '{card_name}' ({language})")

            # Publish to RabbitMQ — Go's notification_worker.go consumes this
            # model_dump(mode="json") = serialize Pydantic model to JSON-compatible dict
            # datetime objects serialized as ISO 8601 strings
            

        
        return listings

    async def publish_batch(self, routing_key: str, messages: List[dict]):
        if not messages:
            return 
        try:
            await self.publisher.publish_batch(routing_key, messages)
            logger.info(f"Count of messages {len(messages)}")
        except Exception as e:
            logger.error(f"Failed to publish batch to {routing_key}: {e}")


def _slab_query(slab_tier: Optional[str]) -> str:
    mapping = {
        "PSA_10": "PSA 10",
        "PSA_9": "PSA 9",
        "PSA_8": "PSA 8",
        "PSA_7": "PSA 7",
        "CGC_10": "CGC 10",
        "CGC_9_5": "CGC 9.5",
        "CGC_9": "CGC 9",
        "BGS_10": "BGS 10",
        "BGS_9_5": "BGS 9.5",
        "BGS_9": "BGS 9",
        "BGS_BLACK_LABEL": "BGS 10 Black Label",
    }
    return mapping.get(slab_tier or "", "")


def _looks_like_target_card(title: str, card_name: str) -> bool:
    title_words = set(re.findall(r"[a-z0-9]+", title.lower()))
    target_words = [w for w in re.findall(r"[a-z0-9]+", card_name.lower()) if w not in {"ex", "gx", "v", "vmax", "vstar"}]
    if not target_words:
        return True
    return all(word in title_words for word in target_words)


def _blocked_listing(title: str) -> bool:
    lowered = title.lower()
    return any(term in lowered for term in BLOCKED_TITLE_TERMS)


def _normalize_language(language: Optional[str]) -> str:
    value = (language or "BOTH").strip().upper()
    if value in {"EN", "ENGLISH"}:
        return "ENGLISH"
    if value in {"JP", "JPN", "JAPAN", "JAPANESE"}:
        return "JAPANESE"
    return "BOTH"


def _language_query(language: str) -> str:
    if language == "ENGLISH":
        return "English"
    if language == "JAPANESE":
        return "Japanese"
    return ""


def _matches_language(title: str, language: str) -> bool:
    lowered = title.lower()
    japanese_terms = (
        "japanese", "japan", "jp ", "jpn", "split earth", "e4", "e-card",
        "pokemon card game", "neo", "mysterious mountains",
    )
    english_terms = ("english", " eng ", " wotc ", "black star promo")
    is_japanese = any(term in f" {lowered} " for term in japanese_terms)
    is_english = any(term in f" {lowered} " for term in english_terms)
    if language == "JAPANESE":
        return is_japanese or not is_english
    if language == "ENGLISH":
        return is_english or not is_japanese
    return True

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
