"""PriceCharting source connector for Pokemon slab trend research."""

from __future__ import annotations

import re
from dataclasses import dataclass
from urllib.parse import urljoin

from scrapling import Selector
from scrapling.fetchers import Fetcher

BASE_URL = "https://www.pricecharting.com"
BIG_MOVERS_URL = f"{BASE_URL}/big-movers?category=pokemon-cards"
SEALED_TERMS = ("booster box", "booster pack", "blister pack", "sealed", "elite trainer box")


@dataclass(frozen=True)
class PriceChartingMover:
    title: str
    set_name: str
    url: str
    loose_price: float | None
    change_amount: float | None


def money_to_float(value: object) -> float | None:
    text = str(value or "").strip()
    match = re.search(r"\$?\s*([0-9][0-9,]*(?:\.\d+)?)", text)
    if not match:
        return None
    return float(match.group(1).replace(",", ""))


def parse_big_mover_rows(html: str, limit: int = 25) -> list[PriceChartingMover]:
    page = Selector(html)
    movers: list[PriceChartingMover] = []
    for row in page.css("table tr")[1:]:
        cells = row.css("td")
        if len(cells) < 4:
            continue
        title = _text(cells[0])
        if not title or any(term in title.lower() for term in SEALED_TERMS):
            continue
        link = cells[0].css("a")
        if not link:
            continue
        href = link[0].attrib.get("href")
        movers.append(PriceChartingMover(
            title=title,
            set_name=_text(cells[1]),
            url=urljoin(BASE_URL, href or ""),
            loose_price=money_to_float(_text(cells[2])),
            change_amount=money_to_float(_text(cells[3])),
        ))
        if len(movers) >= limit:
            break
    return movers


def parse_grade_prices(html: str) -> dict[str, float]:
    page = Selector(html)
    prices = [money_to_float(_text(node)) for node in page.css("#price_data .price")]
    prices = [price for price in prices if price is not None]
    result: dict[str, float] = {}
    if len(prices) > 3:
        result["GRADE_9"] = prices[3]
    if len(prices) > 4:
        result["GRADE_9_5"] = prices[4]
    if len(prices) > 5:
        result["PSA_10"] = prices[5]
    return result


def fetch_big_movers(limit: int = 25) -> list[PriceChartingMover]:
    page = Fetcher.get(BIG_MOVERS_URL, stealthy_headers=True)
    return parse_big_mover_rows(str(page.body), limit=limit)


def fetch_grade_prices(url: str) -> dict[str, float]:
    page = Fetcher.get(url, stealthy_headers=True)
    return parse_grade_prices(str(page.body))


def _text(node) -> str:
    return " ".join(part.strip() for part in node.css("::text").getall() if part and part.strip())
