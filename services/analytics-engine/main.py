"""
============================================================
PokémonTool — Analytics Engine Entry Point
============================================================
Runs scheduled batch analytics jobs:
  - Hourly:   Trend analysis (rising/falling card detection)
  - Every 30m: News scanning for price-moving events
  - Daily 6AM: "Deal of the Day" computation

Writes results back to MongoDB so the Node.js server can serve them.
Also publishes trend change events to RabbitMQ for real-time SSE alerts.
============================================================
"""

import os
import sys
import logging
import schedule
import time

from dotenv import load_dotenv
from pymongo import MongoClient

from trend_analyzer import TrendAnalyzer
from market_scanner import MarketScanner
from news_scanner   import NewsScanner
from show_finder    import ShowFinder

load_dotenv()

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [ANALYTICS] %(levelname)s — %(message)s",
    handlers=[logging.StreamHandler(sys.stdout)],
)
log = logging.getLogger(__name__)


def main():
    log.info("📊 PokémonTool Analytics Engine starting...")

    # Connect to MongoDB (shared with Node.js server)
    mongo = MongoClient(os.getenv("MONGO_URI", "mongodb://localhost:27017/pokemontool"))
    db    = mongo["pokemontool"]

    # Initialize all analyzers
    trend_analyzer  = TrendAnalyzer(db)
    market_scanner  = MarketScanner(db)
    news_scanner    = NewsScanner(db)
    show_finder     = ShowFinder(db)

    # --- Run everything immediately on startup ---
    log.info("Running initial analytics pass...")
    trend_analyzer.run()
    news_scanner.run()
    market_scanner.run()
    show_finder.run()

    # --- Schedule recurring jobs ---
    trend_interval = int(os.getenv("TREND_ANALYSIS_INTERVAL_HOURS", 1))
    deal_hour      = int(os.getenv("DEAL_OF_DAY_HOUR", 6))
    news_interval  = int(os.getenv("NEWS_SCAN_INTERVAL_MINUTES", 30))

    # Trend analysis runs every N hours
    schedule.every(trend_interval).hours.do(trend_analyzer.run)

    # News scanner runs every N minutes
    schedule.every(news_interval).minutes.do(news_scanner.run)

    # Deal of the day runs once daily at configured hour (UTC)
    schedule.every().day.at(f"{deal_hour:02d}:00").do(market_scanner.run)

    # Show finder refreshes daily
    schedule.every().day.at("00:00").do(show_finder.run)

    log.info(
        f"✅ Scheduled: trends every {trend_interval}h | "
        f"news every {news_interval}m | "
        f"deals at {deal_hour:02d}:00 UTC"
    )

    # Keep running forever
    while True:
        schedule.run_pending()
        time.sleep(30)


if __name__ == "__main__":
    main()
