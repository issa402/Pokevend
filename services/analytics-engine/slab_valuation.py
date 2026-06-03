"""Deterministic slab valuation from sold comps.

LLMs can summarize evidence, but market value here is computed from sold comps.
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
from decimal import Decimal, ROUND_HALF_UP
from statistics import median
from typing import Iterable, Mapping, Any


@dataclass(frozen=True)
class SlabValuation:
    market_value: Decimal
    comp_count: int
    liquidity_score: int
    confidence_score: int
    trend_score: int
    filtered_prices: list[Decimal]
    reason: str


def money(value: Any) -> Decimal:
    return Decimal(str(value or "0")).quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)


def score_liquidity(comp_count: int) -> int:
    if comp_count >= 12:
        return 100
    if comp_count >= 8:
        return 85
    if comp_count >= 5:
        return 70
    if comp_count >= 3:
        return 55
    if comp_count >= 1:
        return 25
    return 0


def score_confidence(comp_count: int, market_value: Decimal, raw_count: int) -> int:
    if comp_count == 0 or market_value <= 0:
        return 0
    base = 35 + min(comp_count, 10) * 6
    if raw_count and comp_count < raw_count:
        base -= 10
    if comp_count < 3:
        base -= 20
    return max(0, min(100, base))


def score_trend(comps: Iterable[Mapping[str, Any]], filtered_prices: list[Decimal], now: datetime) -> int:
    if len(filtered_prices) < 4:
        return 0
    recent: list[Decimal] = []
    older: list[Decimal] = []
    for comp in comps:
        price = _price_from_comp(comp)
        if price <= 0 or price not in filtered_prices:
            continue
        sold_at = parse_datetime(comp.get("sold_at") or comp.get("soldAt"), now)
        age_days = max(0, (now - sold_at).days)
        if age_days <= 30:
            recent.append(price)
        elif age_days <= 120:
            older.append(price)
    if len(recent) < 2 or len(older) < 2:
        return 0
    recent_mid = Decimal(str(median(recent)))
    older_mid = Decimal(str(median(older)))
    if older_mid <= 0:
        return 0
    pct_change = ((recent_mid - older_mid) / older_mid) * Decimal("100")
    if pct_change >= 40:
        return 40
    if pct_change >= 25:
        return 30
    if pct_change >= 15:
        return 20
    if pct_change >= 7:
        return 10
    if pct_change <= -20:
        return -20
    if pct_change <= -10:
        return -10
    return 0


def _price_from_comp(comp: Mapping[str, Any]) -> Decimal:
    sold_price = money(comp.get("sold_price") or comp.get("soldPrice"))
    shipping = money(comp.get("shipping_price") or comp.get("shippingPrice"))
    return sold_price + shipping


def filter_outliers(prices: Iterable[Decimal]) -> list[Decimal]:
    values = [price for price in prices if price > 0]
    if len(values) < 4:
        return sorted(values)
    midpoint = Decimal(str(median(values)))
    low = midpoint * Decimal("0.40")
    high = midpoint * Decimal("2.50")
    return sorted(price for price in values if low <= price <= high)


def parse_datetime(value: Any, fallback: datetime) -> datetime:
    if isinstance(value, datetime):
        return value if value.tzinfo else value.replace(tzinfo=timezone.utc)
    if not value:
        return fallback
    text = str(value).strip()
    if text.endswith("Z"):
        text = text[:-1] + "+00:00"
    try:
        parsed = datetime.fromisoformat(text)
        return parsed if parsed.tzinfo else parsed.replace(tzinfo=timezone.utc)
    except ValueError:
        return fallback


def _parse_now(now: datetime | str | None) -> datetime:
    if now is None:
        return datetime.now(timezone.utc)
    if isinstance(now, datetime):
        return now if now.tzinfo else now.replace(tzinfo=timezone.utc)
    return parse_datetime(now, datetime.now(timezone.utc))


def value_from_comps(comps: Iterable[Mapping[str, Any]], now: datetime | str | None = None) -> SlabValuation:
    comp_rows = list(comps)
    raw_prices = [_price_from_comp(comp) for comp in comp_rows]
    filtered = filter_outliers(raw_prices)
    if not filtered:
        return SlabValuation(
            market_value=Decimal("0.00"),
            comp_count=0,
            liquidity_score=0,
            confidence_score=0,
            trend_score=0,
            filtered_prices=[],
            reason="No usable sold comps found.",
        )

    market = Decimal(str(median(filtered))).quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)
    comp_count = len(filtered)
    liquidity = score_liquidity(comp_count)
    confidence = score_confidence(comp_count, market, len(raw_prices))
    trend = score_trend(comp_rows, filtered, _parse_now(now))
    trend_phrase = ""
    if trend > 0:
        trend_phrase = f" Recent comps are stronger; trend score +{trend}."
    elif trend < 0:
        trend_phrase = f" Recent comps are weaker; trend score {trend}."
    reason = f"Median of {comp_count} sold slab comps after outlier filtering.{trend_phrase}"
    return SlabValuation(
        market_value=market,
        comp_count=comp_count,
        liquidity_score=liquidity,
        confidence_score=confidence,
        trend_score=trend,
        filtered_prices=filtered,
        reason=reason,
    )
