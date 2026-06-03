"""Persist live slab research into slab_opportunities."""

from __future__ import annotations

import json
import re
from typing import Iterable

from repositories.slab_comp_repo import get_connection
from services.live_slab_research import LiveSlabCandidate


def to_opportunity_record(candidate: LiveSlabCandidate) -> dict:
    decision = "candidate" if candidate.signal == "BUY_CANDIDATE" else "watch"
    deal_score = score_deal(candidate)
    is_research_target = candidate.signal == "RESEARCH_TARGET"
    if is_research_target:
        reason = (
            f"RESEARCH_TARGET: {candidate.card_title} {candidate.target_slab_tier} is moving on PriceCharting; "
            f"target buy <= ${candidate.target_buy_price:.2f}, reference ${candidate.reference_value:.2f}. "
            "Use this row to scan eBay for the exact card and grade."
        )
    else:
        reason = (
            f"{candidate.signal}: {candidate.card_title} {candidate.target_slab_tier}; "
            f"ask ${candidate.asking_price:.2f}, target buy <= ${candidate.target_buy_price:.2f}, "
            f"all-in ${candidate.all_in_cost:.2f}, reference ${candidate.reference_value:.2f}, "
            f"expected profit ${candidate.expected_profit:.2f}, "
            f"margin {candidate.expected_margin_pct:.2f}%."
        )
    evidence = {
        "signal": candidate.signal,
        "source": "live_slab_research",
        "referenceSource": "pricecharting",
        "referenceUrl": candidate.reference_url,
        "targetBuyPrice": candidate.target_buy_price,
        "trendChange": candidate.trend_change,
        "listingTitle": candidate.listing_title,
        "strictIdentityMatched": not is_research_target,
        "needsEbayScan": is_research_target,
        "ebaySearchUrl": candidate.listing_url if is_research_target else None,
    }
    return {
        "external_card_id": None,
        "card_name": candidate.card_title,
        "set_name": candidate.set_name,
        "grader": grader_from_tier(candidate.target_slab_tier),
        "grade": grade_from_tier(candidate.target_slab_tier),
        "slab_tier": candidate.target_slab_tier,
        "marketplace": candidate.marketplace,
        "listing_id": candidate.listing_id or extract_ebay_listing_id(candidate.listing_url),
        "listing_url": candidate.listing_url,
        "title": candidate.listing_title,
        "asking_price": candidate.asking_price,
        "shipping_price": 0.0,
        "estimated_fees": round(candidate.all_in_cost - candidate.asking_price, 2),
        "all_in_cost": candidate.all_in_cost,
        "estimated_market_value": candidate.reference_value,
        "expected_profit": candidate.expected_profit,
        "expected_margin_pct": candidate.expected_margin_pct,
        "liquidity_score": 0,
        "confidence_score": confidence_score(candidate),
        "risk_score": 30 if candidate.signal == "RESEARCH_TARGET" else 15,
        "deal_score": deal_score,
        "decision": decision,
        "reason": reason,
        "evidence": evidence,
    }


def score_deal(candidate: LiveSlabCandidate) -> int:
    if candidate.signal == "RESEARCH_TARGET":
        trend_score = min(60, int((candidate.trend_change or 0) / 10)) if candidate.trend_change else 0
        value_score = min(80, int(candidate.reference_value / 100))
        return max(40, min(180, 55 + trend_score + value_score))
    if candidate.expected_profit <= 0 or candidate.expected_margin_pct <= 0:
        return 0
    profit_score = min(100, int(candidate.expected_profit / 25))
    margin_score = min(100, int(candidate.expected_margin_pct * 2))
    trend_score = min(40, int((candidate.trend_change or 0) / 25)) if candidate.trend_change else 0
    return max(0, min(300, profit_score + margin_score + trend_score + 70))


def confidence_score(candidate: LiveSlabCandidate) -> int:
    if candidate.signal == "BUY_CANDIDATE":
        return 70
    if candidate.signal == "RESEARCH_TARGET":
        return 45
    return 55


def grader_from_tier(slab_tier: str) -> str | None:
    return slab_tier.split("_", 1)[0] if slab_tier else None


def grade_from_tier(slab_tier: str) -> str | None:
    if not slab_tier or "_" not in slab_tier:
        return None
    return slab_tier.split("_", 1)[1].replace("_", ".")


def extract_ebay_listing_id(url: str) -> str | None:
    match = re.search(r"/itm/(\d+)", url or "")
    return match.group(1) if match else None


class LiveSlabOpportunityRepo:
    def upsert_many(self, candidates: Iterable[LiveSlabCandidate]) -> int:
        rows = [to_opportunity_record(candidate) for candidate in candidates]
        if not rows:
            return 0
        with get_connection() as conn:
            with conn.cursor() as cur:
                for row in rows:
                    row = {**row, "evidence_json": json.dumps(row["evidence"], sort_keys=True)}
                    cur.execute(
                        """
                        INSERT INTO slab_opportunities
                            (external_card_id, card_name, set_name, grader, grade, slab_tier,
                             marketplace, listing_id, listing_url, title, asking_price,
                             shipping_price, estimated_fees, all_in_cost, estimated_market_value,
                             expected_profit, expected_margin_pct, liquidity_score, confidence_score,
                             risk_score, deal_score, decision, reason, evidence, updated_at)
                        VALUES
                            (%(external_card_id)s, %(card_name)s, %(set_name)s, %(grader)s, %(grade)s, %(slab_tier)s,
                             %(marketplace)s, %(listing_id)s, %(listing_url)s, %(title)s, %(asking_price)s,
                             %(shipping_price)s, %(estimated_fees)s, %(all_in_cost)s, %(estimated_market_value)s,
                             %(expected_profit)s, %(expected_margin_pct)s, %(liquidity_score)s, %(confidence_score)s,
                             %(risk_score)s, %(deal_score)s, %(decision)s, %(reason)s, %(evidence_json)s::jsonb, NOW())
                        ON CONFLICT (marketplace, listing_id) WHERE listing_id IS NOT NULL
                        DO UPDATE SET
                            asking_price = EXCLUDED.asking_price,
                            estimated_fees = EXCLUDED.estimated_fees,
                            all_in_cost = EXCLUDED.all_in_cost,
                            estimated_market_value = EXCLUDED.estimated_market_value,
                            expected_profit = EXCLUDED.expected_profit,
                            expected_margin_pct = EXCLUDED.expected_margin_pct,
                            confidence_score = EXCLUDED.confidence_score,
                            risk_score = EXCLUDED.risk_score,
                            deal_score = EXCLUDED.deal_score,
                            decision = CASE
                                WHEN slab_opportunities.decision IN ('approved', 'rejected') THEN slab_opportunities.decision
                                ELSE EXCLUDED.decision
                            END,
                            reason = EXCLUDED.reason,
                            evidence = EXCLUDED.evidence,
                            updated_at = NOW()
                        """,
                        row,
                    )
            conn.commit()
        return len(rows)
