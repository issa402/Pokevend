"""Batch runner for approved Scrapling sold-comp sources.

The config is deliberately explicit per source. Each source chooses its URL,
selectors, and card/slab identity so bad generic scraping does not poison comps.
"""

from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from repositories.slab_comp_repo import SlabCompRepo
from services.scrapling_sold_comps import (
    ScraplingSoldCompExtractor,
    SoldCompContext,
    SoldCompSelectorConfig,
)


@dataclass(frozen=True)
class SoldCompSource:
    name: str
    enabled: bool
    url: str
    marketplace: str
    card_name: str
    set_name: str | None
    external_card_id: str | None
    grader: str
    grade: str
    slab_tier: str
    language_preference: str
    selectors: SoldCompSelectorConfig


def load_source_config(path: str | Path) -> list[SoldCompSource]:
    payload = json.loads(Path(path).read_text(encoding="utf-8"))
    sources = payload.get("sources")
    if not isinstance(sources, list):
        raise ValueError("sold-comp config must contain a sources array")
    return [_parse_source(source) for source in sources]


def run_batch(path: str | Path, *, dry_run: bool = False) -> dict[str, Any]:
    sources = load_source_config(path)
    extractor = ScraplingSoldCompExtractor()
    parsed: list[dict[str, Any]] = []
    fetched = 0
    per_source: list[dict[str, Any]] = []

    for source in sources:
        if not source.enabled:
            per_source.append({"name": source.name, "enabled": False, "parsed": 0})
            continue
        fetched += 1
        comps = extractor.fetch(
            source.url,
            source.selectors,
            SoldCompContext(
                card_name=source.card_name,
                set_name=source.set_name,
                external_card_id=source.external_card_id,
                grader=source.grader,
                grade=source.grade,
                slab_tier=source.slab_tier,
                marketplace=source.marketplace,
                language_preference=source.language_preference,
            ),
        )
        parsed.extend(comps)
        per_source.append({"name": source.name, "enabled": True, "parsed": len(comps)})

    inserted = 0 if dry_run else SlabCompRepo().upsert_many(parsed)
    return {
        "sourcesSeen": len(sources),
        "sourcesFetched": fetched,
        "compsParsed": len(parsed),
        "compsInserted": inserted,
        "dryRun": dry_run,
        "sources": per_source,
    }


def _parse_source(raw: Any) -> SoldCompSource:
    if not isinstance(raw, dict):
        raise ValueError("each sold-comp source must be an object")
    if raw.get("enabled") is False:
        return SoldCompSource(
            name=str(raw.get("name") or "disabled"),
            enabled=False,
            url="",
            marketplace="",
            card_name="",
            set_name=None,
            external_card_id=None,
            grader="",
            grade="",
            slab_tier="",
            language_preference="ANY",
            selectors=SoldCompSelectorConfig(row_selector="", title_selector="", price_selector=""),
        )

    selectors = _required_object(raw, "selectors")
    return SoldCompSource(
        name=_required_str(raw, "name"),
        enabled=True,
        url=_required_str(raw, "url"),
        marketplace=_required_str(raw, "marketplace"),
        card_name=_required_str(raw, "cardName"),
        set_name=_optional_str(raw, "setName"),
        external_card_id=_optional_str(raw, "externalCardId"),
        grader=_required_str(raw, "grader"),
        grade=_required_str(raw, "grade"),
        slab_tier=_required_str(raw, "slabTier"),
        language_preference=str(raw.get("languagePreference") or "ANY"),
        selectors=SoldCompSelectorConfig(
            row_selector=_required_str(selectors, "row"),
            title_selector=_required_str(selectors, "title"),
            price_selector=_required_str(selectors, "price"),
            sold_at_selector=_optional_str(selectors, "soldAt"),
            link_selector=_optional_str(selectors, "link"),
        ),
    )


def _required_object(raw: dict[str, Any], key: str) -> dict[str, Any]:
    value = raw.get(key)
    if not isinstance(value, dict):
        raise ValueError(f"sold-comp source missing object field: {key}")
    return value


def _required_str(raw: dict[str, Any], key: str) -> str:
    value = raw.get(key)
    if not isinstance(value, str) or not value.strip():
        raise ValueError(f"sold-comp source missing string field: {key}")
    return value.strip()


def _optional_str(raw: dict[str, Any], key: str) -> str | None:
    value = raw.get(key)
    if value is None:
        return None
    if not isinstance(value, str):
        raise ValueError(f"sold-comp source field must be string: {key}")
    value = value.strip()
    return value or None
