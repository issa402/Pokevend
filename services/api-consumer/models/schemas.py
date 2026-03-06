# ============================================================
# FILE: services/api-consumer/models/schemas.py
# TYPE: Models Layer — Pydantic Domain Models
#
# WHAT IS THIS?
# Pydantic models define the shape and validation rules for data
# moving between layers. They're the Python equivalent of Go's models/.
#
# PYDANTIC V2 FUNDAMENTALS:
#   - Declare class fields with type hints → automatic validation
#   - Attempting to create a model with wrong types raises ValidationError
#   - .model_dump() converts to dict for JSON serialization
#   - .model_dump(mode="json") ensures datetime/UUID are JSON-serializable strings
#   - Auto-generates JSON schema for FastAPI's /docs UI
#
# WHY PYDANTIC OVER RAW DICTS?
#   dict: {"price": "not a number"} → no error until you try to use it
#   Pydantic: price: float → ValidationError immediately on creation
#
# FAANG STANDARD: Type-safe data contracts between services
# At Stripe, all internal data is typed with Pydantic (Python) or
# protobuf definitions that generate typed code.
#
# PYTHON TYPES YOU MUST KNOW:
#   str, int, float, bool     — primitives
#   Optional[str]             — can be str or None
#   List[CardListing]         — list of CardListing objects
#   datetime, date            — from datetime module
#   Union[int, str]           — either int or str
# ============================================================
from pydantic import BaseModel, HttpUrl
from typing import Optional
from datetime import datetime


class CardListing(BaseModel):
    """
    A single marketplace listing for a Pokémon card.
    Published to RabbitMQ and consumed by Go's notification_worker.go.
    
    BaseModel: inheriting from BaseModel gives automatic:
      - Field validation on __init__
      - .model_dump() serialization
      - .model_json_schema() JSON schema generation
      - Equality comparison (two CardListings with same data are equal)
    
    This is the MESSAGE CONTRACT between Python and Go.
    Both sides must agree on the field names and types.
    In Go: see worker/notification_worker.go's Listing struct.
    The json field names must match (snake_case in both).
    """

    # REQUIRED fields — must be provided, no default
    card_name: str              # "Charizard Base Set"
    price: float                # must be a number (Pydantic converts "12.5" → 12.5)
    marketplace: str            # "ebay" | "tcgplayer" | "facebook" | "mercari"
    listing_url: str            # URL to the actual listing

    # OPTIONAL fields — can be None (= NULL in SQL, null in JSON)
    # Optional[str] = Union[str, None] = either a string or nothing
    image_url: Optional[str] = None
    condition: Optional[str] = None      # "NM", "LP", "MP", "HP", "Damaged"
    set_name: Optional[str] = None
    seller_rating: Optional[float] = None

    # DEFAULT value: if not provided, use datetime.utcnow() at creation time
    # Note: datetime.utcnow() is called ONCE per field definition (class level)
    # To call it per-instance: use default_factory
    scraped_at: datetime = datetime.utcnow()


class PricePoint(BaseModel):
    """
    Historical price data for a card on a specific date.
    Written by analytics-engine to price_history table.
    Read by Go's store/card_store.go for chart data.
    
    Represents one observation: "Charizard was $450 on eBay on 2024-01-15."
    """
    card_id: str        # matches cards.card_id in PostgreSQL
    card_name: str      # for logging/debugging
    price: float
    marketplace: str
    recorded_at: datetime = datetime.utcnow()

# ============================================================
# TODO #1 (Practice): Add a Pydantic validator
# Pydantic V2 supports field validators using @field_validator decorator.
# Add a validator to CardListing that:
#   - Strips whitespace from card_name
#   - Converts card_name to title case ("charizard" → "Charizard")
# HINT:
#   from pydantic import field_validator
#   @field_validator("card_name")
#   @classmethod
#   def normalize_card_name(cls, v: str) -> str:
#       return v.strip().title()
# This ensures consistent card names across all sources.

# TODO #2 (Practice): Add a MarketplaceListing union type
# Different marketplaces have different required fields.
# eBay requires listing_url; Facebook requires location.
# Create marketplace-specific subclasses:
#   class EbayListing(CardListing): ebay_item_id: str
#   class FacebookListing(CardListing): location: str; is_local: bool
# Then create: AnyListing = Union[EbayListing, FacebookListing]
# Research: Pydantic discriminated unions — more advanced typing
# ============================================================
