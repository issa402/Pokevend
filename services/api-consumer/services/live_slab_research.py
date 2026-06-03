"""Live slab buy research from PriceCharting trends plus eBay active asks."""

from __future__ import annotations

import asyncio
import json
import re
from urllib.parse import quote_plus
from dataclasses import dataclass
from pathlib import Path
from typing import Any

import httpx
from dotenv import load_dotenv

from repositories.ebay_repo import EbayRepo
from services.pricecharting_market import PriceChartingMover, fetch_big_movers, fetch_grade_prices, money_to_float

BLOCKED_TITLE_TERMS = (
    "keychain", "mini slab", "stand", "case only", "display", "proxy", "custom",
    "reprint", "digital", "code card", "booster box", "booster pack", "sealed",
    "box topper", "topper", "jumbo", "oversized", "acrylic", "lot of",
)
DEFAULT_FEE_RATE = 0.14


@dataclass(frozen=True)
class CardTarget:
    title: str
    set_name: str
    url: str
    required_terms: tuple[str, ...]
    identity_terms: tuple[str, ...]
    target_slab_tier: str
    reference_value: float
    trend_change: float | None = None


@dataclass(frozen=True)
class LiveSlabCandidate:
    card_title: str
    set_name: str
    target_slab_tier: str
    reference_value: float
    target_buy_price: float
    asking_price: float
    all_in_cost: float
    expected_profit: float
    expected_margin_pct: float
    signal: str
    trend_change: float | None
    listing_title: str
    listing_url: str
    listing_id: str | None = None
    reference_url: str | None = None
    marketplace: str = "ebay"

    def to_dict(self) -> dict[str, Any]:
        return {
            "cardTitle": self.card_title,
            "setName": self.set_name,
            "targetSlabTier": self.target_slab_tier,
            "referenceValue": self.reference_value,
            "targetBuyPrice": self.target_buy_price,
            "askingPrice": self.asking_price,
            "allInCost": self.all_in_cost,
            "expectedProfit": self.expected_profit,
            "expectedMarginPct": self.expected_margin_pct,
            "signal": self.signal,
            "trendChange": self.trend_change,
            "listingTitle": self.listing_title,
            "listingUrl": self.listing_url,
            "listingId": self.listing_id,
            "referenceUrl": self.reference_url,
            "marketplace": self.marketplace,
        }


def load_targets_config(path: str | Path, slab_tier: str = "PSA_10") -> list[CardTarget]:
    payload = json.loads(Path(path).read_text(encoding="utf-8"))
    targets = payload.get("targets")
    if not isinstance(targets, list):
        raise ValueError("live slab target config must contain a targets array")
    parsed: list[CardTarget] = []
    for raw in targets:
        if not isinstance(raw, dict) or raw.get("enabled") is False:
            continue
        title = _required_str(raw, "title")
        set_name = _required_str(raw, "setName")
        url = _required_str(raw, "priceChartingUrl")
        target_tier = str(raw.get("targetSlabTier") or slab_tier)
        reference_value = raw.get("referenceValue")
        if not reference_value:
            reference_value = fetch_grade_prices(url).get(target_tier)
        if not reference_value:
            continue
        required = tuple(raw.get("requiredTerms") or infer_terms(title, set_name)[0])
        identity = tuple(raw.get("identityTerms") or infer_terms(title, set_name)[1])
        if not required or not identity:
            continue
        parsed.append(CardTarget(
            title=title,
            set_name=set_name,
            url=url,
            required_terms=tuple(str(term) for term in required),
            identity_terms=tuple(str(term) for term in identity),
            target_slab_tier=target_tier,
            reference_value=float(reference_value),
            trend_change=raw.get("trendChange"),
        ))
    return parsed


def _required_str(raw: dict[str, Any], key: str) -> str:
    value = raw.get(key)
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"live slab target missing string field: {key}")
    return value.strip()


def normalize_text(value: str) -> str:
    return re.sub(r"[^a-z0-9]+", " ", value.lower()).strip()


def infer_terms(title: str, set_name: str) -> tuple[tuple[str, ...], tuple[str, ...]]:
    normalized_title = normalize_text(title)
    required = tuple(term for term in re.findall(r"[a-z0-9]+", normalized_title) if len(term) > 2 and term not in {"pokemon", "edition", "1st"})[:3]
    identity_terms = []
    number_match = re.search(r"#\s*([a-zA-Z0-9-]+)", title)
    if number_match:
        identity_terms.append(number_match.group(1).lower())
    for token in re.findall(r"[a-z0-9]+", normalize_text(set_name)):
        if token in {"pokemon", "japanese", "promo", "cards", "card"}:
            continue
        if len(token) > 3:
            identity_terms.append(token)
    return required, tuple(dict.fromkeys(identity_terms))


def clean_display_text(value: str) -> str:
    text = (value or "").replace("\\n", " ").replace("\\t", " ")
    return re.sub(r"\s+", " ", text).strip()


def target_from_mover(mover: PriceChartingMover, slab_tier: str = "PSA_10") -> CardTarget | None:
    title = clean_display_text(mover.title)
    set_name = clean_display_text(mover.set_name)
    grades = fetch_grade_prices(mover.url)
    reference_value = grades.get(slab_tier)
    if not reference_value:
        return None
    required, identity = infer_terms(title, set_name)
    if not required or not identity:
        return None
    return CardTarget(
        title=title,
        set_name=set_name,
        url=mover.url,
        required_terms=required,
        identity_terms=identity,
        target_slab_tier=slab_tier,
        reference_value=reference_value,
        trend_change=mover.change_amount,
    )


def is_credible_slab_listing(title: str, target: CardTarget) -> bool:
    normalized = normalize_text(title)
    normalized_set = normalize_text(target.set_name)
    if any(term in normalized for term in BLOCKED_TITLE_TERMS):
        return False
    if "pop series 5" in normalized_set and any(term in normalized for term in ("celebrations", "classic collection", "25th")):
        return False
    if target.target_slab_tier == "PSA_10" and not any(term in normalized for term in ("psa 10", "psa10", "gem mt 10")):
        return False
    if not all(normalize_text(term) in normalized for term in target.required_terms):
        return False
    return any(normalize_text(term) in normalized for term in target.identity_terms)


def classify_signal(expected_profit: float, expected_margin_pct: float, min_profit: float = 0.0, min_margin_pct: float = 0.0) -> str:
    if expected_profit >= min_profit and expected_margin_pct >= min_margin_pct:
        return "BUY_CANDIDATE"
    return "SELL_RESEARCH"


def target_buy_price(reference_value: float, min_profit: float = 25.0, min_margin_pct: float = 20.0, fee_rate: float = DEFAULT_FEE_RATE) -> float:
    profit_limited_all_in = max(0.0, reference_value - min_profit)
    margin_limited_all_in = reference_value / (1 + (min_margin_pct / 100)) if min_margin_pct > -100 else reference_value
    max_all_in = min(profit_limited_all_in, margin_limited_all_in)
    return round(max_all_in / (1 + fee_rate), 2)


def score_listing_candidate(reference_value: float, asking_price: float, fee_rate: float = DEFAULT_FEE_RATE) -> dict[str, float]:
    all_in = round(asking_price * (1 + fee_rate), 2)
    profit = round(reference_value - all_in, 2)
    margin = round((profit / all_in) * 100, 2) if all_in else 0.0
    return {"allInCost": all_in, "expectedProfit": profit, "expectedMarginPct": margin}


def ebay_search_url(target: CardTarget) -> str:
    query = quote_plus(f"{target.title} {target.set_name} PSA 10 Pokemon")
    return f"https://www.ebay.com/sch/i.html?_nkw={query}&_sop=15"


def research_target_candidate(target: CardTarget, min_profit: float = 25.0, min_margin_pct: float = 20.0, fee_rate: float = DEFAULT_FEE_RATE) -> LiveSlabCandidate:
    return LiveSlabCandidate(
        card_title=target.title,
        set_name=target.set_name,
        target_slab_tier=target.target_slab_tier,
        reference_value=target.reference_value,
        target_buy_price=target_buy_price(target.reference_value, min_profit, min_margin_pct, fee_rate),
        asking_price=0.0,
        all_in_cost=0.0,
        expected_profit=0.0,
        expected_margin_pct=0.0,
        signal="RESEARCH_TARGET",
        trend_change=target.trend_change,
        listing_title=f"Research target from PriceCharting: {target.title} {target.target_slab_tier}",
        listing_url=ebay_search_url(target),
        listing_id=f"pricecharting:{normalize_text(target.title)}:{normalize_text(target.set_name)}:{target.target_slab_tier}",
        reference_url=target.url,
        marketplace="pricecharting",
    )


class LiveSlabResearcher:
    def __init__(self, ebay_repo: EbayRepo | None = None, fee_rate: float = DEFAULT_FEE_RATE):
        self.ebay_repo = ebay_repo or EbayRepo()
        self.fee_rate = fee_rate

    async def research(self, mover_limit: int = 10, min_profit: float = 25.0, min_margin_pct: float = 20.0, target_config: str | Path | None = None, include_sell_research: bool = True) -> list[LiveSlabCandidate]:
        token = await self.ebay_repo.get_token()
        targets = [target for mover in fetch_big_movers(limit=mover_limit) if (target := target_from_mover(mover))]
        if target_config:
            targets.extend(load_targets_config(target_config))
        candidates: list[LiveSlabCandidate] = []
        async with httpx.AsyncClient(timeout=30) as client:
            for target in targets:
                candidates.append(research_target_candidate(target, min_profit, min_margin_pct, self.fee_rate))
                listings = await self._search_ebay(client, token, target)
                for item in listings:
                    title = item.get("title") or ""
                    if not is_credible_slab_listing(title, target):
                        continue
                    asking = money_to_float((item.get("price") or {}).get("value"))
                    if asking is None:
                        continue
                    score = score_listing_candidate(target.reference_value, asking, self.fee_rate)
                    signal = classify_signal(score["expectedProfit"], score["expectedMarginPct"], min_profit, min_margin_pct)
                    if signal != "BUY_CANDIDATE" and not include_sell_research:
                        continue
                    candidates.append(LiveSlabCandidate(
                        card_title=target.title,
                        set_name=target.set_name,
                        target_slab_tier=target.target_slab_tier,
                        reference_value=target.reference_value,
                        target_buy_price=target_buy_price(target.reference_value, min_profit, min_margin_pct, self.fee_rate),
                        asking_price=asking,
                        all_in_cost=score["allInCost"],
                        expected_profit=score["expectedProfit"],
                        expected_margin_pct=score["expectedMarginPct"],
                        signal=signal,
                        trend_change=target.trend_change,
                        listing_title=title,
                        listing_url=item.get("itemWebUrl") or "",
                        listing_id=item.get("itemId"),
                        reference_url=target.url,
                        marketplace="ebay",
                    ))
        return sorted(candidates, key=lambda c: (c.signal == "BUY_CANDIDATE", c.expected_profit, c.trend_change or 0), reverse=True)

    async def _search_ebay(self, client: httpx.AsyncClient, token: str, target: CardTarget) -> list[dict[str, Any]]:
        query = f"{target.title} {target.set_name} PSA 10 Pokemon"
        response = await client.get(
            f"{self.ebay_repo.base_url}/buy/browse/v1/item_summary/search",
            headers={"Authorization": f"Bearer {token}"},
            params={
                "q": query,
                "limit": 100,
                "filter": "buyingOptions:{FIXED_PRICE}",
                "sort": "price",
                "auto_correct": "KEYWORD",
            },
        )
        response.raise_for_status()
        return response.json().get("itemSummaries", [])


def load_pokemon_env() -> None:
    load_dotenv(Path(__file__).resolve().parents[3] / ".env")


async def run_live_research(mover_limit: int = 10, min_profit: float = 25.0, min_margin_pct: float = 20.0, target_config: str | Path | None = None, include_sell_research: bool = True) -> list[LiveSlabCandidate]:
    load_pokemon_env()
    return await LiveSlabResearcher().research(mover_limit=mover_limit, min_profit=min_profit, min_margin_pct=min_margin_pct, target_config=target_config, include_sell_research=include_sell_research)
