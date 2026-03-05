# ============================================================
# analytics-engine/models/schemas.py — Pydantic domain models
# ============================================================
from pydantic import BaseModel
from typing import Optional
from datetime import date, datetime


class TrendResult(BaseModel):
    """Computed trend for a card after analysis."""
    card_id: str
    card_name: str
    trend_label: str      # "RISING" | "FALLING" | "STABLE"
    trending_score: int   # -100 to 100
    pct_change_7d: float
    avg_price: float
    computed_at: datetime = datetime.utcnow()


class DealResult(BaseModel):
    """A card deal where listing price is significantly below market."""
    card_name: str
    set_name: Optional[str] = None
    image_url: Optional[str] = None
    market_price: float
    best_price: float
    savings: float
    savings_pct: float
    listing_url: str
    marketplace: str
    reason: Optional[str] = None
    deal_date: date = date.today()
