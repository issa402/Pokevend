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
import hashlib
import html
import logging
import re
from datetime import datetime
from typing import List, Optional
from urllib.parse import quote_plus, urlencode

from models.schemas import CardListing
from publisher.rabbitmq_publisher import RabbitMQPublisher
from repositories.ebay_repo import EbayRepo
from services.slab_parser import parse_slab
from scrapling.fetchers import Fetcher

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
        card_number: Optional[str] = None,
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
            slab_query = _slab_query(slab_tier) if asset_type == "SLAB" else ""
            language = _normalize_language(language_preference)
            queries = _search_queries(
                card_name=card_name,
                set_name=set_name,
                card_number=card_number,
                asset_type=asset_type,
                slab_query=slab_query,
            )
            query = queries[0]
            used_web_fallback = False
            raw_items = []
            seen_item_ids = set()
            for search_query in queries:
                raw_page = await self.repo.search_listings(search_query, limit=200, max_pages=max_pages)
                for item in raw_page.get("itemSummaries", []):
                    item_id = item.get("itemId") or item.get("itemWebUrl") or item.get("title")
                    if item_id in seen_item_ids:
                        continue
                    seen_item_ids.add(item_id)
                    raw_items.append(item)
                if asset_type != "SLAB" and raw_items:
                    break
            raw = {"itemSummaries": raw_items}
            if asset_type == "SLAB" and not raw.get("itemSummaries"):
                raw = {"itemSummaries": _scrape_ebay_active_listings(query)}
                used_web_fallback = True
        except Exception as e:
            # Catch ALL exceptions from the repo — network errors, auth failures, etc.
            # Slab scans still try the web fallback because Browse auth can fail while public eBay search works.
            logger.error(f"eBay search failed for '{card_name}': {e}")
            if asset_type == "SLAB":
                raw = {"itemSummaries": _scrape_ebay_active_listings(query)}
                used_web_fallback = True
            else:
                raw = {"itemSummaries": []}
                used_web_fallback = False

        listings = _filter_listing_items(
            raw.get("itemSummaries", []),
            card_name=card_name,
            external_card_id=external_card_id,
            set_name=set_name,
            card_number=card_number,
            asset_type=asset_type,
            slab_tier=slab_tier,
            language=language,
        )
        if asset_type == "SLAB" and not listings and not used_web_fallback:
            fallback_items = _scrape_ebay_active_listings(query)
            listings = _filter_listing_items(
                fallback_items,
                card_name=card_name,
                external_card_id=external_card_id,
                set_name=set_name,
                card_number=card_number,
                asset_type=asset_type,
                slab_tier=slab_tier,
                language=language,
            )
        if publish and listings:
            for l in listings:
                payload = l.model_dump(mode = "json") 
                await self.publisher.publish("listings", payload)
        logger.info(f"eBay: published {len(listings)} listings for '{card_name}' ({language})")

            # Publish to RabbitMQ — Go's notification_worker.go consumes this
            # model_dump(mode="json") = serialize Pydantic model to JSON-compatible dict
            # datetime objects serialized as ISO 8601 strings
            

        
        return listings

    async def import_listing_text(
        self,
        text: str,
        card_name: str,
        external_card_id: Optional[str] = None,
        set_name: Optional[str] = None,
        card_number: Optional[str] = None,
        asset_type: str = "SLAB",
        slab_tier: Optional[str] = None,
        publish: bool = True,
        language_preference: str = "BOTH",
    ) -> List[CardListing]:
        language = _normalize_language(language_preference)
        items = _parse_ebay_copied_text(
            text,
            card_name=card_name,
            external_card_id=external_card_id,
            set_name=set_name,
        )
        listings = _filter_listing_items(
            items,
            card_name=card_name,
            external_card_id=external_card_id,
            set_name=set_name,
            card_number=card_number,
            asset_type=asset_type,
            slab_tier=slab_tier,
            language=language,
        )
        if publish and listings:
            for listing in listings:
                await self.publisher.publish("listings", listing.model_dump(mode="json"))
        logger.info(
            "eBay text import: published %s/%s listings for %r (%s)",
            len(listings),
            len(items),
            card_name,
            language,
        )
        return listings

    async def publish_batch(self, routing_key: str, messages: List[dict]):
        if not messages:
            return 
        try:
            await self.publisher.publish_batch(routing_key, messages)
            logger.info(f"Count of messages {len(messages)}")
        except Exception as e:
            logger.error(f"Failed to publish batch to {routing_key}: {e}")


def _filter_listing_items(
    items: list[dict],
    *,
    card_name: str,
    external_card_id: Optional[str],
    set_name: Optional[str],
    card_number: Optional[str],
    asset_type: str,
    slab_tier: Optional[str],
    language: str,
) -> List[CardListing]:
    listings: List[CardListing] = []
    for item in items:
        title = item.get("title", "") or ""
        if not _looks_like_target_card(title, card_name):
            continue
        if set_name and not _matches_set(title, set_name):
            continue
        if card_number and not _matches_card_number(title, card_number):
            continue
        if not _matches_language(title, language):
            continue

        slab = parse_slab(item.get("title", ""), item.get("condition"))
        if _blocked_listing(title, is_slab=slab["is_slab"]):
            continue

        price = float(item.get("price", {}).get("value", 0))
        if price <= 0:
            continue

        if asset_type == "RAW" and slab["is_slab"]:
            continue
        if asset_type == "SLAB":
            if not slab["is_slab"]:
                continue
            if slab_tier and slab["slab_tier"] != slab_tier:
                continue

        listings.append(CardListing(
            card_name=card_name,
            external_card_id=external_card_id,
            price=price,
            marketplace="ebay",
            listing_url=item.get("itemWebUrl", ""),
            listing_id=item.get("itemId"),
            listing_title=title,
            image_url=item.get("image", {}).get("imageUrl"),
            condition=item.get("condition"),
            set_name=set_name,
            is_slab=slab["is_slab"],
            grader=slab["grader"],
            grade=slab["grade"],
            slab_tier=slab["slab_tier"],
            cert_number=slab["cert_number"],
            language_preference=language,
            scraped_at=datetime.utcnow(),
        ))
    return listings


def _scrape_ebay_active_listings(query: str) -> list[dict]:
    url = "https://www.ebay.com/sch/i.html?" + urlencode({"_nkw": query, "_sop": "15"})
    try:
        page = Fetcher.get(url, stealthy_headers=True)
    except Exception as exc:
        logger.warning("eBay web fallback failed for %r: %s", query, exc)
        return []

    raw_html = str(getattr(page, "html_content", "") or getattr(page, "body", "") or "")
    pattern = re.compile(
        r'href="(?P<link>https://www\.ebay\.com/itm/[^"?]+)[^"]*"(?P<body>.{0,5000}?)'
        r'<span class="su-styled-text primary default">(?P<title>.*?)</span>(?P<tail>.{0,2500}?)'
        r'<span class="su-styled-text primary bold large-1 s-card__price">(?P<price>.*?)</span>',
        re.IGNORECASE | re.DOTALL,
    )
    listings: list[dict] = []
    seen: set[str] = set()
    for match in pattern.finditer(raw_html):
        title = _strip_markup(match.group("title"))
        price = _price_value(match.group("price"))
        link = html.unescape(match.group("link"))
        if not title or price is None or link in seen:
            continue
        seen.add(link)
        listings.append({
            "title": title,
            "price": {"value": price},
            "itemWebUrl": link,
            "itemId": _item_id(link),
            "condition": _strip_markup(match.group("tail")),
        })
    return listings


_SKIP_IMPORT_LINES = {
    "shop ebay live events",
    "see events",
    "opens in a new window or tab",
    "pre-owned",
    "new (other)",
    "buy it now",
    "or best offer",
    "located in united states",
    "located in united kingdom",
    "located in canada",
    "located in germany",
    "located in italy",
    "located in japan",
    "authenticity guarantee",
    "in the psa vault",
    "free returns",
    "don't miss what's live",
}


def _parse_ebay_copied_text(
    text: str,
    *,
    card_name: str,
    external_card_id: Optional[str],
    set_name: Optional[str],
) -> list[dict]:
    lines = [_clean_import_line(line) for line in (text or "").splitlines()]
    lines = [line for line in lines if line]
    listings: list[dict] = []
    seen: set[str] = set()
    current_title: Optional[str] = None

    for line in lines:
        price = _import_line_price_value(line)
        if price is not None and current_title:
            key = _manual_listing_id(current_title, price, card_name, external_card_id)
            if key not in seen:
                seen.add(key)
                listings.append({
                    "title": current_title,
                    "price": {"value": price},
                    "itemWebUrl": _manual_listing_url(current_title),
                    "itemId": key,
                    "condition": "Imported from copied eBay search text",
                })
            current_title = None
            continue

        candidate = _title_from_import_line(line)
        if candidate:
            current_title = candidate

    return listings


def _clean_import_line(line: str) -> str:
    line = _strip_markup(line).replace("Opens in a new window or tab", "").strip()
    return " ".join(line.split())


def _title_from_import_line(line: str) -> Optional[str]:
    lowered = line.lower().strip()
    if not line or lowered in _SKIP_IMPORT_LINES:
        return None
    if lowered.startswith(("image ", "+$", "+$", "free delivery", "free shipping", "shipping estimate")):
        return None
    if " image 1 of " in lowered or " image 2 of " in lowered:
        return None
    if "watchers" in lowered or "delivery" in lowered or "coupon" in lowered:
        return None
    if _import_line_price_value(line) is not None:
        return None
    if len(line) < 8:
        return None
    return line


def _manual_listing_id(title: str, price: float, card_name: str, external_card_id: Optional[str]) -> str:
    identity = f"{external_card_id or ''}|{card_name}|{title}|{price:.2f}".lower()
    return "manual:" + hashlib.sha1(identity.encode("utf-8")).hexdigest()[:24]


def _manual_listing_url(title: str) -> str:
    return "https://www.ebay.com/sch/i.html?_nkw=" + quote_plus(title)

def _strip_markup(value: str | None) -> str:
    if not value:
        return ""
    text = re.sub(r"<[^>]+>", " ", html.unescape(value))
    return " ".join(text.split())


def _import_line_price_value(value: str | None) -> float | None:
    text = _strip_markup(value).replace(",", "")
    if "$" not in text:
        return None
    match = re.search(r"\$\s*(\d+(?:\.\d{1,2})?)", text)
    return float(match.group(1)) if match else None


def _price_value(value: str | None) -> float | None:
    text = _strip_markup(value).replace(",", "")
    match = re.search(r"(\d+(?:\.\d{1,2})?)", text)
    return float(match.group(1)) if match else None


def _item_id(url: str) -> str | None:
    match = re.search(r"/itm/(\d+)", url)
    return match.group(1) if match else None


def _search_text(value: Optional[str]) -> str:
    return (value or "").replace("Pokémon", "Pokemon").replace("pokémon", "pokemon").strip()


def _search_queries(
    *,
    card_name: str,
    set_name: Optional[str],
    card_number: Optional[str],
    asset_type: str,
    slab_query: str,
) -> list[str]:
    base_parts = [_search_text(card_name)]
    if set_name:
        base_parts.append(_search_text(set_name))
    number = (card_number or "").strip()
    queries: list[str] = []

    def add(parts: list[str]) -> None:
        query = " ".join(part for part in parts if part).strip()
        if query and query not in queries:
            queries.append(query)

    if asset_type != "SLAB":
        add(base_parts)
        if number:
            add([_search_text(card_name), number, _search_text(set_name)])
        return queries

    add(base_parts + ([slab_query] if slab_query else ["graded"]))
    if number:
        add([_search_text(card_name), number, _search_text(set_name), slab_query])
        add([_search_text(card_name), f"#{number}", _search_text(set_name), slab_query])
        if slab_query:
            add([slab_query, _search_text(card_name), number, _search_text(set_name)])
    return queries


def _slab_query(slab_tier: Optional[str]) -> str:
    mapping = {
        "PSA_10": "PSA 10",
        "PSA_9": "PSA 9",
        "PSA_8": "PSA 8",
        "PSA_7": "PSA 7",
        "PSA_6": "PSA 6",
        "PSA_5": "PSA 5",
        "PSA_4": "PSA 4",
        "PSA_3": "PSA 3",
        "PSA_2": "PSA 2",
        "PSA_1": "PSA 1",
        "CGC_10": "CGC 10",
        "CGC_9_5": "CGC 9.5",
        "CGC_9": "CGC 9",
        "CGC_8_5": "CGC 8.5",
        "CGC_8": "CGC 8",
        "CGC_7_5": "CGC 7.5",
        "CGC_7": "CGC 7",
        "CGC_6": "CGC 6",
        "CGC_5": "CGC 5",
        "CGC_4": "CGC 4",
        "CGC_3": "CGC 3",
        "CGC_2": "CGC 2",
        "CGC_1": "CGC 1",
        "BGS_10": "BGS 10",
        "BGS_9_5": "BGS 9.5",
        "BGS_9": "BGS 9",
        "BGS_8_5": "BGS 8.5",
        "BGS_8": "BGS 8",
        "BGS_7_5": "BGS 7.5",
        "BGS_7": "BGS 7",
        "BGS_6": "BGS 6",
        "BGS_5": "BGS 5",
        "BGS_4": "BGS 4",
        "BGS_3": "BGS 3",
        "BGS_2": "BGS 2",
        "BGS_1": "BGS 1",
        "BGS_BLACK_LABEL": "BGS 10 Black Label",
    }
    return mapping.get(slab_tier or "", "")


def _looks_like_target_card(title: str, card_name: str) -> bool:
    title_words = set(re.findall(r"[a-z0-9]+", title.lower()))
    target_words = [w for w in re.findall(r"[a-z0-9]+", card_name.lower()) if w not in {"ex", "gx", "v", "vmax", "vstar"}]
    if not target_words:
        return True
    return all(word in title_words for word in target_words)


def _matches_card_number(title: str, card_number: str) -> bool:
    number = str(card_number or "").strip().lstrip("0")
    if not number:
        return True
    fractions = re.findall(r"(?<!\d)(\d{1,4})\s*/\s*(\d{1,4})(?!\d)", title)
    if fractions:
        return any(numerator.lstrip("0") == number for numerator, _ in fractions)
    hash_numbers = re.findall(r"#\s*(\d{1,4})(?!\d)", title)
    if hash_numbers:
        return any(value.lstrip("0") == number for value in hash_numbers)
    return True


def _matches_set(title: str, set_name: str) -> bool:
    title_text = _normalized_words(title)
    set_tokens = [
        token for token in _normalized_words(set_name)
        if token not in {"pokemon", "tcg", "card", "cards", "the", "ex"}
    ]
    if not set_tokens:
        return True
    return all(token in title_text for token in set_tokens)


def _normalized_words(value: str) -> set[str]:
    normalized = value.replace("Pokémon", "Pokemon").replace("pokémon", "pokemon")
    return set(re.findall(r"[a-z0-9]+", normalized.lower()))


def _blocked_listing(title: str, *, is_slab: bool = False) -> bool:
    lowered = title.lower()
    for term in BLOCKED_TITLE_TERMS:
        if term == "deck" and is_slab:
            continue
        if term in lowered:
            return True
    return False


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
