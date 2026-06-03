"""Scrapling-based sold-comps extractor for slab opportunity valuation.

This module intentionally avoids hardcoding one marketplace. Each source gets
its own selector config so operators can respect that site's terms and tune the
parser without changing core scoring code.
"""

from __future__ import annotations

import re
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Iterable
from urllib.parse import urljoin

from scrapling.fetchers import Fetcher


PRICE_PATTERN = re.compile(r"(?P<amount>\d[\d,]*(?:\.\d{1,2})?)")


@dataclass(frozen=True)
class SoldCompSelectorConfig:
    row_selector: str
    title_selector: str
    price_selector: str
    sold_at_selector: str | None = None
    link_selector: str | None = None


@dataclass(frozen=True)
class SoldCompContext:
    card_name: str
    grader: str
    grade: str
    slab_tier: str
    marketplace: str
    set_name: str | None = None
    external_card_id: str | None = None
    language_preference: str = "ANY"


class ScraplingSoldCompExtractor:
    def fetch(self, url: str, selector_config: SoldCompSelectorConfig, context: SoldCompContext) -> list[dict]:
        page = Fetcher.get(url, stealthy_headers=True)
        rows = page.css(selector_config.row_selector)
        comps: list[dict] = []
        for row in rows:
            title = _first_text(row, selector_config.title_selector)
            price_text = _first_text(row, selector_config.price_selector)
            sold_at_text = _first_text(row, selector_config.sold_at_selector) if selector_config.sold_at_selector else ""
            link = _first_attr(row, selector_config.link_selector, "href") if selector_config.link_selector else None
            sold_price = parse_price(price_text)
            if not title or sold_price is None:
                continue
            comps.append({
                "external_card_id": context.external_card_id,
                "card_name": context.card_name,
                "set_name": context.set_name,
                "language_preference": context.language_preference,
                "grader": context.grader,
                "grade": context.grade,
                "slab_tier": context.slab_tier,
                "cert_number": parse_cert_number(title),
                "marketplace": context.marketplace,
                "sold_price": sold_price,
                "shipping_price": 0,
                "sold_at": parse_sold_at(sold_at_text),
                "listing_url": urljoin(url, link) if link else url,
                "title": title,
            })
        return comps


def parse_price(value: str | None) -> float | None:
    if not value:
        return None
    match = PRICE_PATTERN.search(value.replace(",", ""))
    if not match:
        return None
    return float(match.group("amount"))


def parse_sold_at(value: str | None) -> datetime:
    if not value:
        return datetime.now(timezone.utc)
    text = value.strip().replace("Sold", "").strip()
    for fmt in ("%b %d, %Y", "%B %d, %Y", "%m/%d/%Y", "%Y-%m-%d"):
        try:
            return datetime.strptime(text, fmt).replace(tzinfo=timezone.utc)
        except ValueError:
            continue
    return datetime.now(timezone.utc)


def parse_cert_number(title: str) -> str | None:
    match = re.search(r"\b(?:cert|certification|cert\s*#)\s*[:#]?\s*(\d{5,})\b", title, re.IGNORECASE)
    return match.group(1) if match else None


def _first_text(row, selector: str | None) -> str:
    if not selector:
        return ""
    result = row.css(selector)
    if not result:
        return ""
    value = result.get()
    return " ".join(str(value or "").split())


def _first_attr(row, selector: str | None, attr: str) -> str | None:
    if not selector:
        return None
    result = row.css(selector)
    if not result:
        return None
    selected = result if hasattr(result, "attrib") else result[0]
    value = selected.attrib.get(attr)
    return str(value) if value else None
