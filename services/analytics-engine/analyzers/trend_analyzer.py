# ============================================================
# FILE: services/analytics-engine/analyzers/trend_analyzer.py
# TYPE: Analyzer (Business Logic Layer) — Card Trend Computation
#
# WHAT IS THIS?
# The TrendAnalyzer computes whether card prices are RISING, FALLING, or STABLE.
# This is the "brain" of the analytics engine.
#
# WHERE IT FITS IN THE ARCHITECTURE:
#   main.py (scheduler)
#     → TrendAnalyzer.run()        ← you are here
#       → CardRepo.get_all_cards() (repository layer — SQL)
#       → CardRepo.get_price_history() (repository layer — SQL)
#       → TrendAnalyzer._analyze_card() (business logic — math)
#       → CardRepo.update_trend() (repository layer — SQL UPDATE)
#
# THE ALGORITHM:
#   1. Fetch 7-day price history for a card (from price_history table)
#   2. Run linear regression on the prices (finds the "trend line")
#   3. slope > 0 → prices generally going up
#   4. If 7-day % change > +5%: RISING with positive score
#   5. If 7-day % change < -5%: FALLING with negative score
#   6. Else: STABLE with score near 0
#
# FAANG DATA SCIENCE PATTERN: Small ML built into the service
# You don't need a separate ML platform for simple algorithms.
# Linear regression is a standard math function (in numpy).
# Complex models (transformers, neural nets) would go in a separate ML service.
#
# PYTHON CONCEPTS:
#   numpy (numerical computing), class methods, private methods (_analyze_card),
#   list comprehension, Optional return type
# ============================================================
import logging
import numpy as np                          # numerical computing library
from typing import List, Optional

from models.schemas import TrendResult
from repositories.card_repo import CardRepo

logger = logging.getLogger(__name__)


class TrendAnalyzer:
    """
    Analyzes price history and computes trend labels and scores for all cards.
    
    SINGLE RESPONSIBILITY: This class does ONE thing — compute trends.
    It doesn't write HTTP responses, it doesn't publish to RabbitMQ.
    It reads price history, computes math, writes trend data back.
    
    Receives CardRepo via injection — testable with a mock repo.
    """

    def __init__(self, repo: CardRepo):
        self.repo = repo

    def run(self) -> List[TrendResult]:
        """
        Main entry point: analyze all cards and update their trends in PostgreSQL.
        Called by main.py scheduler on a configurable interval.
        
        Processes cards sequentially (not async) because this is CPU-bound
        (math), not I/O-bound (network). Async doesn't help for CPU work.
        For very large card databases (100k+ cards), you'd use multiprocessing.
        """
        cards = self.repo.get_all_cards()  # synchronous psycopg2 query
        results = []

        for card in cards:
            try:
                result = self._analyze_card(card)
                if result is None:
                    continue  # not enough data yet

                # Write computed trends back to PostgreSQL via repo (not directly)
                self.repo.update_trend(
                    card["card_id"],
                    result.trend_label,
                    result.trending_score,
                    result.pct_change_7d,
                )
                results.append(result)

            except Exception as e:
                # Log but NEVER let one bad card crash the whole analysis
                # This is critical for production reliability
                logger.warning(f"Trend analysis failed for {card['name']}: {e}")

        logger.info(f"Trend analysis complete: {len(results)} cards updated")
        return results

    def _analyze_card(self, card: dict) -> Optional[TrendResult]:
        """
        Private method (underscore prefix convention): compute trend for ONE card.
        
        Called by run() for each card.
        Returns None if there's not enough price history (need at least 2 data points).
        
        NUMPY LINEAR REGRESSION:
        np.polyfit(x, y, 1) fits a line (y = mx + b) to the data.
        Returns [m (slope), b (intercept)].
        slope > 0 = prices trending up
        slope < 0 = prices trending down
        
        LINEAR REGRESSION at FAANG:
        Used in recommendation systems, anomaly detection, trend analysis.
        numpy is standard — no need for TensorFlow for simple math.
        """
        history = self.repo.get_price_history(card["card_id"], days=7)

        # Need at least 2 data points to define a trend line
        if len(history) < 2:
            return None

        # LIST COMPREHENSION: [expression for item in iterable if condition]
        # Extract avg_price from each history row, skip rows with no price (NULL)
        prices = [float(h["avg_price"]) for h in history if h["avg_price"]]
        if not prices:
            return None

        # NUMPY OPERATIONS:
        # np.arange(n) = [0, 1, 2, ..., n-1] — the x-axis (day numbers)
        # np.polyfit(x, y, 1) = linear regression, returns [slope, intercept]
        # float() converts numpy float64 to Python float (for JSON serialization)
        x     = np.arange(len(prices))
        slope = float(np.polyfit(x, prices, 1)[0])  # [0] = slope only
        avg   = float(np.mean(prices))               # np.mean = average

        # Percentage change: (final - initial) / initial * 100
        # Guard against division by zero (prices[0] could be 0 for free cards)
        pct_change = ((prices[-1] - prices[0]) / prices[0]) * 100 if prices[0] else 0.0
        # prices[-1] = last element (Python: negative index = from end)

        # Normalize slope to -100..100 range (our trending_score field)
        # slope/avg removes the price scale effect (same slope on $10 vs $1000 card = same score)
        # * 1000 amplifies for scoring scale
        score = min(100, max(-100, int(slope / avg * 1000))) if avg else 0

        # Business rule: >5% change = trending, <5% = stable
        if pct_change > 5:
            label = "RISING"
        elif pct_change < -5:
            label = "FALLING"
        else:
            label = "STABLE"

        return TrendResult(
            card_id=card["card_id"],
            card_name=card["name"],
            trend_label=label,
            trending_score=score,
            pct_change_7d=round(pct_change, 2),  # round to 2 decimal places
            avg_price=round(avg, 2),
        )

# ============================================================
# TODO #1 (Practice): Improve the trend algorithm with volume weighting
# Currently we treat all days equally. But a day with 50 sales should
# carry more weight than a day with 1 sale.
# Modify _analyze_card() to use sale_count as weights in np.polyfit:
#   weights = [float(h["sale_count"] or 1) for h in history]
#   np.polyfit(x, prices, 1, w=weights)  # w= = weights parameter
# HINT: history rows have "sale_count" from price_history table

# TODO #2 (Practice): Add anomaly detection
# Sometimes a card's price spikes or drops dramatically in one day
# (e.g., a famous YouTuber opens a pack on stream — "the YouTube effect").
# These are outliers that distort the trend line.
# Add a method _remove_outliers(prices: List[float]) -> List[float]:
#   Use IQR (Interquartile Range) method to detect outliers:
#   Q1, Q3 = np.percentile(prices, [25, 75])
#   IQR = Q3 - Q1
#   Return prices where Q1 - 1.5*IQR <= price <= Q3 + 1.5*IQR
# Call this before running polyfit for more accurate trend lines.
# ============================================================
