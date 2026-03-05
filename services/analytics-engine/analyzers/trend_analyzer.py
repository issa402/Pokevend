# ============================================================
# analytics-engine/analyzers/trend_analyzer.py
# Business logic ONLY: decide if a card is RISING/FALLING/STABLE
# Reads from CardRepo, writes trend back via CardRepo.
# ============================================================
import logging
import numpy as np
from typing import List

from models.schemas import TrendResult
from repositories.card_repo import CardRepo

logger = logging.getLogger(__name__)


class TrendAnalyzer:
    """Computes trend labels and scores for all cards using price history."""

    def __init__(self, repo: CardRepo):
        self.repo = repo

    def run(self) -> List[TrendResult]:
        """Analyze all cards and write trend results back to PostgreSQL."""
        cards   = self.repo.get_all_cards()
        results = []
        for card in cards:
            try:
                result = self._analyze_card(card)
                if result:
                    self.repo.update_trend(
                        card["card_id"], result.trend_label,
                        result.trending_score, result.pct_change_7d,
                    )
                    results.append(result)
            except Exception as e:
                logger.warning(f"Trend failed for {card['name']}: {e}")
        logger.info(f"Trend analysis complete: {len(results)} cards updated")
        return results

    def _analyze_card(self, card: dict) -> TrendResult:
        history = self.repo.get_price_history(card["card_id"], days=7)
        if len(history) < 2:
            return None  # Not enough data

        prices = [float(h["avg_price"]) for h in history if h["avg_price"]]
        if not prices:
            return None

        # Linear regression slope to determine trend direction
        x      = np.arange(len(prices))
        slope  = float(np.polyfit(x, prices, 1)[0])
        avg    = float(np.mean(prices))

        pct_change = ((prices[-1] - prices[0]) / prices[0]) * 100 if prices[0] else 0.0
        score      = min(100, max(-100, int(slope / avg * 1000)))  # Normalize to -100..100

        if pct_change > 5:
            label = "RISING"
        elif pct_change < -5:
            label = "FALLING"
        else:
            label = "STABLE"

        return TrendResult(
            card_id=card["card_id"], card_name=card["name"],
            trend_label=label, trending_score=score,
            pct_change_7d=round(pct_change, 2), avg_price=round(avg, 2),
        )
