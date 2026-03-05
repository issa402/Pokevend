# ============================================================
# api-consumer/services/ebay_service.py
# Business logic: fetch eBay listings → normalize → publish
# Calls EbayRepo for data, RabbitMQPublisher to dispatch.
# ============================================================
import logging
from datetime import datetime
from typing import List

from models.schemas import CardListing
from publisher.rabbitmq_publisher import RabbitMQPublisher
from repositories.ebay_repo import EbayRepo

logger = logging.getLogger(__name__)


class EbayService:
    def __init__(self, repo: EbayRepo, publisher: RabbitMQPublisher):
        self.repo      = repo
        self.publisher = publisher

    async def scan_card(self, card_name: str) -> List[CardListing]:
        """
        Fetch eBay listings for a card, normalize to CardListing,
        publish each to RabbitMQ 'listings' queue for Go to process.
        """
        try:
            raw = await self.repo.search_listings(card_name)
        except Exception as e:
            logger.error(f"eBay search failed for '{card_name}': {e}")
            return []

        listings: List[CardListing] = []
        for item in raw.get("itemSummaries", []):
            price = float(item.get("price", {}).get("value", 0))
            if price <= 0:
                continue
            listing = CardListing(
                card_name=card_name,
                price=price,
                marketplace="ebay",
                listing_url=item.get("itemWebUrl", ""),
                image_url=item.get("image", {}).get("imageUrl"),
                condition=item.get("condition"),
                scraped_at=datetime.utcnow(),
            )
            listings.append(listing)

            # Publish to RabbitMQ for Go notification worker to process
            await self.publisher.publish("listings", listing.model_dump(mode="json"))

        logger.info(f"eBay: published {len(listings)} listings for '{card_name}'")
        return listings
