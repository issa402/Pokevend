"""Wholesale slab opportunity scoring.

This module is intentionally deterministic. It scores opportunities from active
listings and sold comps without letting an LLM invent price truth.
"""

from __future__ import annotations

from dataclasses import dataclass, asdict
from decimal import Decimal, ROUND_HALF_UP
from typing import Any, Mapping

from slab_valuation import SlabValuation, money, value_from_comps

MIN_PROFIT = Decimal("25.00")
MIN_MARGIN_PCT = Decimal("20.00")
DEFAULT_FEE_RATE = Decimal("0.14")


@dataclass(frozen=True)
class SlabOpportunityScore:
    external_card_id: str | None
    card_name: str
    set_name: str | None
    grader: str | None
    grade: str | None
    slab_tier: str | None
    marketplace: str | None
    listing_id: str | None
    listing_url: str | None
    title: str | None
    asking_price: Decimal
    shipping_price: Decimal
    estimated_fees: Decimal
    all_in_cost: Decimal
    estimated_market_value: Decimal
    expected_profit: Decimal
    expected_margin_pct: Decimal
    liquidity_score: int
    confidence_score: int
    risk_score: int
    deal_score: int
    decision: str
    reason: str
    evidence: dict[str, Any]

    def to_record(self) -> dict[str, Any]:
        record = asdict(self)
        for key, value in list(record.items()):
            if isinstance(value, Decimal):
                record[key] = float(value)
        return record


def margin_pct(profit: Decimal, cost: Decimal) -> Decimal:
    if cost <= 0:
        return Decimal("0.00")
    return ((profit / cost) * Decimal("100")).quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)


def margin_score(margin: Decimal) -> int:
    if margin >= 80:
        return 100
    if margin >= 60:
        return 85
    if margin >= 40:
        return 70
    if margin >= 25:
        return 50
    if margin >= 15:
        return 25
    return 0


def risk_score(listing: Mapping[str, Any], valuation: SlabValuation) -> int:
    risk = 0
    if not listing.get("external_card_id") and not listing.get("externalCardId"):
        risk += 30
    if not listing.get("slab_tier") and not listing.get("slabTier"):
        risk += 40
    if valuation.comp_count < 3:
        risk += 25
    title = str(listing.get("listing_title") or listing.get("title") or "")
    if any(word in title.lower() for word in ["reprint", "proxy", "custom", "digital"]):
        risk += 60
    return min(100, risk)


def score_listing(
    listing: Mapping[str, Any],
    comps: list[Mapping[str, Any]],
    fee_rate: Decimal = DEFAULT_FEE_RATE,
) -> SlabOpportunityScore | None:
    is_slab = bool(listing.get("is_slab") or listing.get("isSlab"))
    slab_tier = listing.get("slab_tier") or listing.get("slabTier")
    price = money(listing.get("price") or listing.get("asking_price") or listing.get("askingPrice"))
    if not is_slab or not slab_tier or price <= 0:
        return None

    valuation = value_from_comps(comps)
    shipping = money(listing.get("shipping_price") or listing.get("shippingPrice"))
    fees = (price * fee_rate).quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)
    all_in = price + shipping + fees
    profit = valuation.market_value - all_in
    margin = margin_pct(profit, all_in)
    risk = risk_score(listing, valuation)
    score = margin_score(margin) + valuation.liquidity_score + valuation.confidence_score + valuation.trend_score - risk
    decision = "candidate" if profit >= MIN_PROFIT and margin >= MIN_MARGIN_PCT and score > 80 else "watch"

    reason = (
        f"{margin}% expected margin, ${profit} expected profit, "
        f"confidence {valuation.confidence_score}/100 from {valuation.comp_count} sold comps."
    )

    return SlabOpportunityScore(
        external_card_id=listing.get("external_card_id") or listing.get("externalCardId"),
        card_name=listing.get("card_name") or listing.get("cardName") or "Unknown Card",
        set_name=listing.get("set_name") or listing.get("setName"),
        grader=listing.get("grader"),
        grade=listing.get("grade"),
        slab_tier=slab_tier,
        marketplace=listing.get("marketplace"),
        listing_id=listing.get("listing_id") or listing.get("listingId"),
        listing_url=listing.get("listing_url") or listing.get("listingUrl"),
        title=listing.get("listing_title") or listing.get("title"),
        asking_price=price,
        shipping_price=shipping,
        estimated_fees=fees,
        all_in_cost=all_in,
        estimated_market_value=valuation.market_value,
        expected_profit=profit,
        expected_margin_pct=margin,
        liquidity_score=valuation.liquidity_score,
        confidence_score=valuation.confidence_score,
        risk_score=risk,
        deal_score=max(0, min(300, score)),
        decision=decision,
        reason=reason,
        evidence={
            "valuationReason": valuation.reason,
            "compCount": valuation.comp_count,
            "trendScore": valuation.trend_score,
            "filteredCompPrices": [float(price) for price in valuation.filtered_prices],
        },
    )
