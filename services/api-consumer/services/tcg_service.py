# ============================================================
# api-consumer/services/tcg_service.py
# Business logic: fetch TCGplayer prices → normalize → publish
# ============================================================
import logging
from typing import List

from models.schemas import CardListing
from publisher.rabbitmq_publisher import RabbitMQPublisher
from repositories.tcg_repo import TCGRepo

logger = logging.getLogger(__name__)


class TCGService:
    def __init__(self, repo: TCGRepo, publisher: RabbitMQPublisher):
        self.repo      = repo
        self.publisher = publisher

    async def scan_card(self, card_name: str) -> List[CardListing]:
        """
        Search TCGplayer catalog, fetch prices, normalize to CardListing,
        publish to 'listings' queue.
        """
        try:
            search_result = await self.repo.search_products(card_name)
        except Exception as e:
            logger.error(f"TCGplayer search failed for '{card_name}': {e}")
            return []

        listings: List[CardListing] = []
        for product in search_result.get("results", [])[:5]:  # Top 5 matches
            product_id = str(product.get("productId", ""))
            if not product_id:
                continue
            try:
                price_data = await self.repo.get_prices(product_id)
                for entry in price_data.get("results", []):
                    market_price = entry.get("marketPrice")
                    if not market_price:
                        continue
                    listing = CardListing(
                        card_name=card_name,
                        price=float(market_price),
                        marketplace="tcgplayer",
                        listing_url=f"https://www.tcgplayer.com/product/{product_id}",
                        image_url=product.get("imageUrl"),
                        condition=entry.get("subTypeName"),
                    )
                    listings.append(listing)
                    await self.publisher.publish("listings", listing.model_dump(mode="json"))
            except Exception as e:
                logger.warning(f"TCGplayer price fetch failed for product {product_id}: {e}")

        logger.info(f"TCGplayer: published {len(listings)} listings for '{card_name}'")
        return listings
