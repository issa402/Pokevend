"""
============================================================
PokémonTool — Pydantic Data Models for api-consumer
============================================================
Defines the canonical shapes of data flowing between services.
Pydantic validates types and provides easy JSON serialization.
============================================================
"""

from pydantic import BaseModel, Field
from typing import Optional
from datetime import datetime


class CardListing(BaseModel):
    """
    Represents a single card listing found on any marketplace.
    This is the standard internal format shared across all services.
    """
    card_name:   str
    price:       float
    marketplace: str                  # "ebay", "tcgplayer", "facebook", "mercari"
    listing_url: Optional[str] = None
    image_url:   Optional[str] = None
    seller:      Optional[str] = None
    condition:   Optional[str] = "Unknown"   # NM, LP, MP, HP, Damaged
    listing_id:  Optional[str] = None        # Platform's own item ID
    set_name:    Optional[str] = None
    location:    Optional[str] = None        # For local marketplace listings
    discovered_at: datetime = Field(default_factory=datetime.utcnow)

    def to_queue_dict(self) -> dict:
        """Serializes to a dict safe for RabbitMQ JSON encoding."""
        return {
            "cardName":   self.card_name,
            "price":      self.price,
            "marketplace": self.marketplace,
            "listingUrl": self.listing_url,
            "imageUrl":   self.image_url,
            "seller":     self.seller,
            "condition":  self.condition,
            "listingId":  self.listing_id,
            "setName":    self.set_name,
            "location":   self.location,
            "discoveredAt": self.discovered_at.isoformat(),
        }


class PricePoint(BaseModel):
    """A single price data point for building price history charts."""
    card_name:    str
    marketplace:  str
    price:        float
    sale_velocity: Optional[float] = None  # Estimated sales per day
    timestamp:    datetime = Field(default_factory=datetime.utcnow)


class NotificationEvent(BaseModel):
    """Represents a parsed eBay Notification API webhook event."""
    topic:       str
    event_id:    str
    item_id:     Optional[str] = None
    card_name:   Optional[str] = None
    price:       Optional[float] = None
    event_time:  Optional[datetime] = None
    raw_payload: Optional[dict] = None
