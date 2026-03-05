# ============================================================
# analytics-engine/main.py — Scheduler entry point
# Wires all dependencies and runs the analysis loops.
# ============================================================
import logging
import os
import schedule
import time
from dotenv import load_dotenv

from analyzers.trend_analyzer import TrendAnalyzer
from analyzers.deal_finder import DealFinder
from repositories.card_repo import CardRepo
from repositories.listing_repo import ListingRepo

load_dotenv()
logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(name)s] %(levelname)s: %(message)s")
logger = logging.getLogger(__name__)

# ── Dependency Injection ────────────────────────────────────
card_repo    = CardRepo()
listing_repo = ListingRepo()

trend_analyzer = TrendAnalyzer(repo=card_repo)
deal_finder    = DealFinder(card_repo=card_repo, listing_repo=listing_repo)


def run_trend_analysis():
    logger.info("Running trend analysis...")
    try:
        results = trend_analyzer.run()
        logger.info(f"✓ Trends computed for {len(results)} cards")
    except Exception as e:
        logger.error(f"Trend analysis failed: {e}")


def run_deal_finder():
    logger.info("Running deal finder...")
    try:
        deals = deal_finder.run()
        logger.info(f"✓ {len(deals)} deals found today")
    except Exception as e:
        logger.error(f"Deal finder failed: {e}")


if __name__ == "__main__":
    trend_interval = int(os.getenv("TREND_ANALYSIS_INTERVAL_HOURS", "1"))
    deal_hour      = int(os.getenv("DEAL_OF_DAY_HOUR", "6"))

    # Run immediately on start
    run_trend_analysis()
    run_deal_finder()

    # Schedule recurring runs
    schedule.every(trend_interval).hours.do(run_trend_analysis)
    schedule.every().day.at(f"{deal_hour:02d}:00").do(run_deal_finder)

    logger.info(f"✓ Analytics engine running — trends every {trend_interval}h, deals at {deal_hour:02d}:00")
    while True:
        schedule.run_pending()
        time.sleep(60)
