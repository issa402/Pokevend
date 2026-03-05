# ============================================================
# api-consumer/models/schemas.py — Pydantic domain models
# Used for data validation and serialization across all layers
# ============================================================
from pydantic import BaseModel, HttpUrl
from typing import Optional
from datetime import datetime


class CardListing(BaseModel):
    """A single marketplace listing for a Pokémon card."""
    card_name: str
    set_name: Optional[str] = None
    price: float
    marketplace: str          # "ebay" | "tcgplayer" | "facebook" | "mercari"
    listing_url: str
    image_url: Optional[str] = None
    condition: Optional[str] = None
    seller_rating: Optional[float] = None
    scraped_at: datetime = datetime.utcnow()


class PricePoint(BaseModel):
    """Historical price entry for a card."""
    card_id: str
    card_name: str
    price: float
    marketplace: str
    recorded_at: datetime = datetime.utcnow()
