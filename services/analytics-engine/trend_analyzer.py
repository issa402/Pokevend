"""
============================================================
PokémonTool — Trend Analyzer
============================================================
Calculates rising/falling trends for Pokemon cards by:
  1. Pulling 30-day price history from MongoDB
  2. Computing 7-day and 30-day moving averages
  3. Comparing recent average vs older average
  4. Labeling cards as RISING, FALLING, or STABLE
  5. Writing trend scores back to MongoDB
  6. Publishing TREND_CHANGE events to RabbitMQ for real-time alerts

Runs hourly via the scheduler in main.py.
============================================================
"""

import logging
import os
from datetime import datetime, timedelta
from typing import List, Tuple

import numpy as np
import pika
import json

log = logging.getLogger(__name__)

RISING_THRESHOLD  =  0.10   # +10% change = RISING
FALLING_THRESHOLD = -0.10   # -10% change = FALLING


class TrendAnalyzer:
    """Analyzes price history to detect trending cards."""

    def __init__(self, db):
        """
        :param db: A pymongo database instance (shared MongoDB connection)
        """
        self.db          = db
        self.cards_col   = db["cards"]        # Card metadata + trend scores
        self.history_col = db["price_history"] # Time-series price data

    def run(self):
        """Main entry point — run a full trend analysis pass."""
        log.info("Running trend analysis...")
        try:
            cards_analyzed = 0
            trend_changes  = []

            # Get all unique card IDs from price history
            card_ids = self.history_col.distinct("cardId")
            log.info(f"Analyzing {len(card_ids)} cards for trends...")

            for card_id in card_ids:
                changed = self._analyze_card(card_id)
                if changed:
                    trend_changes.append(changed)
                cards_analyzed += 1

            # Publish trend change events to RabbitMQ for SSE alerts
            if trend_changes:
                self._publish_trend_changes(trend_changes)

            log.info(f"Trend analysis complete: {cards_analyzed} cards, {len(trend_changes)} trend changes")
        except Exception as e:
            log.error(f"Trend analysis failed: {e}", exc_info=True)

    def _analyze_card(self, card_id: str) -> dict:
        """
        Analyzes a single card's price history.
        Returns a dict with trend info if the trend label changed, else None.
        """
        now      = datetime.utcnow()
        days_30  = now - timedelta(days=30)
        days_7   = now - timedelta(days=7)

        # Get 30-day price history
        history = list(self.history_col.find(
            {"cardId": card_id, "timestamp": {"$gte": days_30}},
            sort=[("timestamp", 1)]
        ))

        if len(history) < 5:
            return None  # Not enough data points to calculate a meaningful trend

        prices = [h["avgPrice"] for h in history if h.get("avgPrice")]
        if not prices:
            return None

        prices_np = np.array(prices)

        # Split into "older" (days 30-8) and "recent" (last 7 days) windows
        cutoff_idx   = max(1, len(prices) - 7)  # Approximate split at 7 days
        older_prices = prices_np[:cutoff_idx]
        recent_prices= prices_np[cutoff_idx:]

        if len(older_prices) < 2 or len(recent_prices) < 2:
            return None

        older_avg  = float(np.mean(older_prices))
        recent_avg = float(np.mean(recent_prices))

        if older_avg == 0:
            return None

        # Calculate percentage change between the two windows
        pct_change = (recent_avg - older_avg) / older_avg

        # Assign trend label
        if pct_change >= RISING_THRESHOLD:
            trend_label  = "RISING"
            trend_score  = min(100, int(pct_change * 200))   # Scale to 0-100
        elif pct_change <= FALLING_THRESHOLD:
            trend_label  = "FALLING"
            trend_score  = max(-100, int(pct_change * 200))  # Scale to -100 to 0
        else:
            trend_label   = "STABLE"
            trend_score   = 0

        # Fetch current card document
        card = self.cards_col.find_one({"cardId": card_id})
        old_label = card.get("trendLabel") if card else None

        # Update the card document with new trend data
        self.cards_col.update_one(
            {"cardId": card_id},
            {"$set": {
                "trendLabel":    trend_label,
                "trendingScore": trend_score,
                "pctChange7d":   round(pct_change * 100, 2),
                "avgPrice7d":    round(recent_avg, 2),
                "avgPrice30d":   round(older_avg, 2),
                "lastUpdated":   now,
            }},
            upsert=True,
        )

        # Only report if the trend label actually changed (avoid alert spam)
        if old_label and old_label != trend_label:
            return {
                "type":       "TREND_CHANGE",
                "cardId":     card_id,
                "cardName":   card.get("name", "Unknown") if card else card_id,
                "trendLabel": trend_label,
                "changePct":  round(abs(pct_change) * 100, 2),
                "oldLabel":   old_label,
            }

        return None

    def _publish_trend_changes(self, changes: List[dict]):
        """
        Publishes trend change events to the RabbitMQ analytics queue.
        The Node.js notification service will pick these up and alert users.
        """
        try:
            params  = pika.URLParameters(os.getenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672"))
            conn    = pika.BlockingConnection(params)
            channel = conn.channel()
            channel.queue_declare(queue="analytics_results", durable=True)

            for change in changes:
                channel.basic_publish(
                    exchange="",
                    routing_key="analytics_results",
                    body=json.dumps(change).encode(),
                    properties=pika.BasicProperties(delivery_mode=2),
                )
                log.info(f"Published trend change: {change['cardName']} → {change['trendLabel']}")

            conn.close()
        except Exception as e:
            log.warning(f"Could not publish trend changes to RabbitMQ: {e}")
