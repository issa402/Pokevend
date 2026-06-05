"""Authenticated eBay Seller Hub Product Research connector.

This module never stores credentials or cookies in the database. Browser login
state stays in a local Playwright profile directory controlled by env vars.
"""

from __future__ import annotations

import os
import re
import time
from dataclasses import asdict, dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from statistics import mean
from typing import Iterable, Literal
from urllib.parse import urlencode

from services.slab_parser import parse_slab

def _default_profile_dir(module_file: Path) -> Path:
    parents = module_file.resolve().parents
    project_root = parents[3] if len(parents) > 3 else parents[1]
    return project_root / ".local" / "ebay-seller-hub-profile"


DEFAULT_PROFILE_DIR = _default_profile_dir(Path(__file__))
SELLER_HUB_RESEARCH_URL = "https://www.ebay.com/sh/research"
LOGIN_BROWSER_ARGS = [
    "--disable-blink-features=AutomationControlled",
    "--no-first-run",
    "--disable-dev-shm-usage",
]
GRADE_TIERS = [
    "PSA_10", "PSA_9", "PSA_8", "PSA_7", "PSA_6", "PSA_5", "PSA_4", "PSA_3", "PSA_2", "PSA_1",
    "CGC_10", "CGC_9_5", "CGC_9", "CGC_8_5", "CGC_8", "CGC_7_5", "CGC_7", "CGC_6", "CGC_5", "CGC_4", "CGC_3", "CGC_2", "CGC_1",
    "BGS_10", "BGS_9_5", "BGS_9", "BGS_8_5", "BGS_8", "BGS_7_5", "BGS_7", "BGS_6", "BGS_5", "BGS_4", "BGS_3", "BGS_2", "BGS_1",
]


@dataclass(frozen=True)
class SellerHubRow:
    title: str
    price: float | None = None
    shipping: float | None = None
    bids: int | None = None
    watchers: int | None = None
    promoted: bool | None = None
    start_date: str | None = None
    slab_tier: str | None = None


@dataclass(frozen=True)
class SellerHubResearchMetrics:
    external_card_id: str | None
    card_name: str
    set_name: str | None
    card_number: str | None
    language_preference: str
    slab_tier: str
    tab_name: Literal["ACTIVE", "SOLD"]
    keywords: str
    day_range: int
    avg_listing_price: float | None = None
    min_listing_price: float | None = None
    max_listing_price: float | None = None
    avg_shipping_price: float | None = None
    free_shipping_pct: float | None = None
    promoted_listing_pct: float | None = None
    total_listings: int | None = None
    avg_watchers: float | None = None
    max_watchers: int | None = None
    avg_bids: float | None = None
    max_bids: int | None = None
    top_rows: list[SellerHubRow] = field(default_factory=list)
    source_url: str | None = None
    researched_at: datetime = field(default_factory=lambda: datetime.now(timezone.utc))

    def to_record(self) -> dict:
        data = asdict(self)
        data["top_rows"] = [asdict(row) for row in self.top_rows]
        data["researched_at"] = self.researched_at.isoformat()
        return data


def seller_hub_profile_dir() -> Path:
    return Path(os.getenv("EBAY_SELLER_HUB_PROFILE_DIR", str(DEFAULT_PROFILE_DIR))).expanduser().resolve()


def _browser_launch_options(*, headless: bool) -> dict:
    options: dict = {
        "headless": headless,
        "viewport": {"width": 1440, "height": 1000},
        "args": LOGIN_BROWSER_ARGS,
        "user_agent": os.getenv(
            "EBAY_SELLER_HUB_USER_AGENT",
            "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
        ),
        "locale": os.getenv("EBAY_SELLER_HUB_LOCALE", "en-US"),
        "timezone_id": os.getenv("EBAY_SELLER_HUB_TZ", "America/New_York"),
    }
    executable = os.getenv("EBAY_SELLER_HUB_BROWSER_EXECUTABLE")
    channel = os.getenv("EBAY_SELLER_HUB_BROWSER_CHANNEL")
    if executable:
        options["executable_path"] = str(Path(executable).expanduser())
    elif channel:
        options["channel"] = channel
    return options


def build_keywords(card_name: str, set_name: str | None, card_number: str | None, slab_tier: str) -> str:
    parts = [card_name]
    inferred_number = _infer_card_number(card_name)
    if card_number and str(card_number).strip().lower().lstrip("#") != (inferred_number or "").lower().lstrip("#"):
        parts.append(str(card_number))
    if set_name:
        parts.append(set_name.replace("Pokémon", "Pokemon"))
    label = slab_tier.replace("_", " ")
    parts.append(label)
    return " ".join(part for part in parts if part).strip()


def research_url(keywords: str, *, tab_name: str = "ACTIVE", day_range: int = 30, offset: int = 0, limit: int = 50) -> str:
    now_ms = int(time.time() * 1000)
    start_ms = now_ms - (day_range * 24 * 60 * 60 * 1000)
    query = urlencode({
        "marketplace": "EBAY-US",
        "keywords": keywords,
        "dayRange": day_range,
        "endDate": now_ms,
        "startDate": start_ms,
        "categoryId": 0,
        "offset": offset,
        "limit": limit,
        "tabName": tab_name.upper(),
        "tz": os.getenv("EBAY_SELLER_HUB_TZ", "America/New_York"),
    })
    return f"{SELLER_HUB_RESEARCH_URL}?{query}"


def parse_research_text(
    text: str,
    *,
    external_card_id: str | None,
    card_name: str,
    set_name: str | None,
    card_number: str | None,
    language_preference: str,
    slab_tier: str,
    tab_name: Literal["ACTIVE", "SOLD"],
    keywords: str,
    day_range: int,
    source_url: str | None = None,
) -> SellerHubResearchMetrics:
    lines = [_clean_line(line) for line in (text or "").splitlines()]
    lines = [line for line in lines if line]
    parsed_rows = _parse_listing_rows(lines, slab_tier=slab_tier)
    rows = _filter_identity_rows(
        parsed_rows,
        card_name=card_name,
        set_name=set_name,
        card_number=card_number,
        slab_tier=slab_tier,
    )
    prices = [row.price for row in rows if row.price is not None]
    shipping_values = [row.shipping for row in rows if row.shipping is not None]
    watcher_values = [row.watchers for row in rows if row.watchers is not None]
    bid_values = [row.bids for row in rows if row.bids is not None]
    promoted_known = [row.promoted for row in rows if row.promoted is not None]

    return SellerHubResearchMetrics(
        external_card_id=external_card_id,
        card_name=card_name,
        set_name=set_name,
        card_number=card_number,
        language_preference=language_preference,
        slab_tier=slab_tier,
        tab_name=tab_name,
        keywords=keywords,
        day_range=day_range,
        avg_listing_price=_avg(prices) or _metric_money(lines, "Avg listing price"),
        min_listing_price=(min(prices) if prices else None) or _price_range(lines)[0],
        max_listing_price=(max(prices) if prices else None) or _price_range(lines)[1],
        avg_shipping_price=_avg(shipping_values) or _metric_money(lines, "Avg shipping"),
        free_shipping_pct=_free_shipping_pct(shipping_values) if rows else _metric_percent(lines, "Free shipping"),
        promoted_listing_pct=_pct_true(promoted_known) if rows else _metric_percent(lines, "Promoted listings"),
        total_listings=len(rows) if rows else _metric_int(lines, "Total active listings"),
        avg_watchers=_avg(watcher_values),
        max_watchers=max(watcher_values) if watcher_values else None,
        avg_bids=_avg(bid_values),
        max_bids=max(bid_values) if bid_values else None,
        top_rows=rows[:50],
        source_url=source_url,
    )


async def open_login_browser(profile_dir: Path | None = None) -> str:
    from playwright.async_api import async_playwright

    profile = profile_dir or seller_hub_profile_dir()
    profile.mkdir(parents=True, exist_ok=True)
    async with async_playwright() as playwright:
        context = await playwright.chromium.launch_persistent_context(
            str(profile),
            **_browser_launch_options(headless=False),
        )
        page = context.pages[0] if context.pages else await context.new_page()
        await page.goto(SELLER_HUB_RESEARCH_URL, wait_until="domcontentloaded")
        print(f"Opened eBay Seller Hub login/research browser with profile: {profile}")
        print("Log into eBay in that browser, confirm Product Research loads, then close the browser window.")
        print("If Google/social login says the browser is not secure, use direct eBay email/username login or set EBAY_SELLER_HUB_BROWSER_CHANNEL=chrome with a system Chrome install.")
        await page.wait_for_timeout(120_000)
        await context.close()
    return str(profile)


async def fetch_research_text(keywords: str, *, tab_name: str = "ACTIVE", day_range: int = 30, headless: bool = True) -> tuple[str, str]:
    from playwright.async_api import async_playwright

    profile = seller_hub_profile_dir()
    if not profile.exists():
        raise RuntimeError(f"Seller Hub browser profile does not exist: {profile}. Run login setup first.")
    url = research_url(keywords, tab_name=tab_name, day_range=day_range)
    async with async_playwright() as playwright:
        context = await playwright.chromium.launch_persistent_context(
            str(profile),
            **_browser_launch_options(headless=headless),
        )
        page = context.pages[0] if context.pages else await context.new_page()
        await page.goto(url, wait_until="domcontentloaded")
        await page.wait_for_timeout(int(os.getenv("EBAY_SELLER_HUB_WAIT_MS", "5000")))
        body_text = await page.locator("body").inner_text(timeout=15_000)
        await context.close()
    if _looks_unauthenticated(body_text):
        raise RuntimeError("Seller Hub research page looks unauthenticated. Run login setup again.")
    return body_text, url


async def research_one_grade(
    *,
    external_card_id: str | None,
    card_name: str,
    set_name: str | None,
    card_number: str | None,
    language_preference: str,
    slab_tier: str,
    tab_name: Literal["ACTIVE", "SOLD"] = "ACTIVE",
    day_range: int = 30,
    headless: bool = True,
) -> SellerHubResearchMetrics:
    keywords = build_keywords(card_name, set_name, card_number, slab_tier)
    text, url = await fetch_research_text(keywords, tab_name=tab_name, day_range=day_range, headless=headless)
    return parse_research_text(
        text,
        external_card_id=external_card_id,
        card_name=card_name,
        set_name=set_name,
        card_number=card_number,
        language_preference=language_preference,
        slab_tier=slab_tier,
        tab_name=tab_name,
        keywords=keywords,
        day_range=day_range,
        source_url=url,
    )


def grade_tiers(limit: int | None = None) -> list[str]:
    tiers = GRADE_TIERS[:]
    return tiers[:limit] if limit else tiers


def _clean_line(line: str) -> str:
    return " ".join((line or "").replace("\u00a0", " ").split())


def _money(value: str) -> float | None:
    match = re.search(r"\$\s*([0-9][0-9,]*(?:\.\d{1,2})?)", value or "")
    return float(match.group(1).replace(",", "")) if match else None


def _intish(value: str) -> int | None:
    text = (value or "").strip()
    if text in {"", "-"}:
        return None
    match = re.search(r"\d+", text.replace(",", ""))
    return int(match.group(0)) if match else None


def _metric_money(lines: Iterable[str], label: str) -> float | None:
    previous = ""
    for line in lines:
        if label.lower() in line.lower():
            return _money(line) or _money(previous)
        previous = line
    return None


def _metric_int(lines: Iterable[str], label: str) -> int | None:
    previous = ""
    for line in lines:
        if label.lower() in line.lower():
            return _intish(line) or _intish(previous)
        previous = line
    return None


def _metric_percent(lines: Iterable[str], label: str) -> float | None:
    previous = ""
    for line in lines:
        if label.lower() in line.lower():
            return _percent(line) or _percent(previous)
        previous = line
    return None


def _percent(value: str) -> float | None:
    match = re.search(r"([0-9]+(?:\.\d+)?)\s*%", value or "")
    return float(match.group(1)) if match else None


def _price_range(lines: Iterable[str]) -> tuple[float | None, float | None]:
    for line in lines:
        if " - " in line and "$" in line:
            values = re.findall(r"\$\s*([0-9][0-9,]*(?:\.\d{1,2})?)", line)
            if len(values) >= 2:
                return float(values[0].replace(",", "")), float(values[1].replace(",", ""))
    return None, None


def _parse_listing_rows(lines: list[str], *, slab_tier: str) -> list[SellerHubRow]:
    rows: list[SellerHubRow] = []
    seen: set[tuple[str, float | None]] = set()
    idx = 0
    while idx < len(lines):
        line = lines[idx]
        if not _looks_like_title(line, slab_tier):
            idx += 1
            continue
        title = line.replace(", preview full size image", "").strip()
        window = lines[idx + 1: idx + 12]
        price_index = next((pos for pos, candidate in enumerate(window) if _money(candidate) is not None), None)
        price = _money(window[price_index]) if price_index is not None else None
        if price is None:
            idx += 1
            continue
        after_price = window[price_index + 1:]
        shipping = _shipping_from_window(after_price)
        stat_cells = _stat_cells_after_price(after_price)
        bids = _intish(stat_cells[0]) if stat_cells else None
        watchers = _intish(stat_cells[1]) if len(stat_cells) > 1 else None
        start_date = next((candidate for candidate in window if re.search(r"(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec) \d{1,2}, \d{4}", candidate)), None)
        key = (title.lower(), price)
        if key in seen:
            idx += 1
            continue
        seen.add(key)
        slab = parse_slab(title, None)
        rows.append(SellerHubRow(
            title=title,
            price=price,
            shipping=shipping,
            bids=bids,
            watchers=watchers,
            promoted="promoted" in " ".join(window).lower(),
            start_date=start_date,
            slab_tier=slab["slab_tier"] if slab["is_slab"] else None,
        ))
        idx += 1
    return rows



def _filter_identity_rows(
    rows: list[SellerHubRow],
    *,
    card_name: str,
    set_name: str | None,
    card_number: str | None,
    slab_tier: str,
) -> list[SellerHubRow]:
    effective_card_number = card_number or _infer_card_number(card_name)
    return [
        row for row in rows
        if _row_matches_target(row, card_name=card_name, set_name=set_name, card_number=effective_card_number, slab_tier=slab_tier)
    ]



def _infer_card_number(value: str) -> str | None:
    text = value or ""
    fraction = re.search(r"#?\s*([a-zA-Z0-9-]+\s*/\s*[a-zA-Z0-9-]+)", text)
    if fraction:
        return fraction.group(1).replace(" ", "")
    explicit = re.search(r"#\s*([a-zA-Z0-9-]+)", text)
    return explicit.group(1) if explicit else None

def _row_matches_target(
    row: SellerHubRow,
    *,
    card_name: str,
    set_name: str | None,
    card_number: str | None,
    slab_tier: str,
) -> bool:
    if row.slab_tier and row.slab_tier != slab_tier:
        return False
    if not _title_has_card_name(row.title, card_name):
        return False
    if card_number and not _title_has_card_number(row.title, card_number):
        return False
    if set_name and not _title_has_set(row.title, set_name):
        return False
    return True


def normalize_for_identity(value: str) -> str:
    return re.sub(r"[^a-z0-9]+", " ", (value or "").lower()).strip()


def _identity_tokens(value: str) -> list[str]:
    ignored = {
        "pokemon", "tcg", "card", "cards", "holo", "foil", "rare", "reverse", "full", "art",
        "gem", "mint", "graded", "grade", "psa", "cgc", "bgs", "edition", "ed", "the", "and",
        "japanese", "english", "promo", "promos", "black", "star", "set", "ex", "gx", "v", "vmax",
    }
    tokens = re.findall(r"[a-z0-9]+", normalize_for_identity(value))
    return [token for token in tokens if len(token) > 1 and token not in ignored and not token.isdigit()]


def _title_has_card_name(title: str, card_name: str) -> bool:
    title_tokens = set(re.findall(r"[a-z0-9]+", normalize_for_identity(title)))
    name_tokens = _identity_tokens(card_name)
    if not name_tokens:
        return True
    # Keep character matching strict; every meaningful name token must be present.
    return all(token in title_tokens for token in name_tokens)


def _title_has_set(title: str, set_name: str) -> bool:
    set_tokens = _identity_tokens(set_name)
    if not set_tokens:
        return True
    title_tokens = set(re.findall(r"[a-z0-9]+", normalize_for_identity(title)))
    return all(token in title_tokens for token in set_tokens)


def _title_has_card_number(title: str, card_number: str) -> bool:
    number = str(card_number or "").strip().lower().lstrip("#")
    if not number:
        return True
    title_lower = title.lower()
    if "/" in number:
        numerator, denominator = number.split("/", 1)
        exact_pattern = rf"(?<!\d){re.escape(numerator)}\s*/\s*{re.escape(denominator)}(?!\d)"
        if re.search(exact_pattern, title_lower):
            return True
        # Reject conflicting fractions when the target has a denominator.
        for found_num, found_den in re.findall(r"(?<!\d)([a-z0-9]+)\s*/\s*([a-z0-9]+)(?!\d)", title_lower):
            if found_den == denominator and found_num != numerator:
                return False
        return re.search(rf"#\s*{re.escape(numerator)}(?!\d)", title_lower) is not None
    compact = re.escape(number)
    explicit_numbers = re.findall(r"#\s*([a-z0-9-]+)", title_lower)
    if explicit_numbers and number not in explicit_numbers:
        return False
    return re.search(rf"(?<![a-z0-9])#?\s*{compact}(?![a-z0-9])", title_lower) is not None


def _stat_cells_after_price(cells: list[str]) -> list[str]:
    stats: list[str] = []
    for cell in cells:
        lower = cell.lower()
        if "$" in cell or "%" in cell or "shipping" in lower:
            continue
        if re.search(r"(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec) \d{1,2}, \d{4}", cell):
            break
        if cell == "-" or re.fullmatch(r"\d+", cell.replace(",", "")):
            stats.append(cell)
        if len(stats) >= 2:
            break
    return stats


def _looks_like_title(line: str, slab_tier: str) -> bool:
    lower = line.lower()
    if len(line) < 12 or "$" in line:
        return False
    if lower in {"listing", "actions", "listing price", "bids", "watchers", "promoted listing", "start date"}:
        return False
    grader = slab_tier.split("_", 1)[0].lower()
    grade = slab_tier.split("_", 1)[1].replace("_", ".") if "_" in slab_tier else ""
    return grader in lower and grade in lower


def _shipping_from_window(window: list[str]) -> float | None:
    for line in window:
        lower = line.lower()
        if "free shipping" in lower:
            return 0.0
        if "shipping" in lower:
            value = _money(line)
            if value is not None:
                return value
    return None


def _avg(values: list[float | int]) -> float | None:
    return round(float(mean(values)), 2) if values else None


def _pct_true(values: list[bool]) -> float | None:
    if not values:
        return None
    return round((sum(1 for value in values if value) / len(values)) * 100, 2)


def _free_shipping_pct(values: list[float]) -> float | None:
    if not values:
        return None
    return round((sum(1 for value in values if value == 0.0) / len(values)) * 100, 2)


def _looks_unauthenticated(text: str) -> bool:
    lower = (text or "").lower()
    blocked_markers = [
        "sign in",
        "please verify yourself",
        "verify yourself to continue",
        "pardon our interruption",
        "access denied",
        "browser or app may not be secure",
    ]
    return any(marker in lower for marker in blocked_markers) and "research products" not in lower
