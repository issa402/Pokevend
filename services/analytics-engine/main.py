# ============================================================
# FILE: services/analytics-engine/main.py
# TYPE: Service Entry Point — Scheduler
#
# WHAT IS THIS?
# The entry point for the analytics-engine service.
# Uses the "schedule" library to run analysis jobs on a timed schedule.
# This is a background CRON-style service — it never handles HTTP requests.
#
# ARCHITECTURE PATTERN: Batch Processing Service
# analytics-engine is designed for periodic batch computation.
# It doesn't need to be fast (no user waiting for it).
# It needs to be RELIABLE (must not crash and miss a scheduled run).
#
# FAANG BATCH PROCESSING:
# Most FAANG companies run batch jobs for:
#   - Nightly analytics (engagement metrics, recommendations)
#   - Daily report generation
#   - Scheduled ML model retraining
# Tools used: Apache Airflow, AWS Glue, Kubernetes CronJobs, or simple schedule library
# Our approach (schedule library) is simple and sufficient for this scale.
#
# HOW THIS CONNECTS TO THE SYSTEM:
#   1. trend_analyzer runs every 1 hour → updates cards.trend_label/trending_score
#   2. deal_finder runs once at 6 AM → writes to deals table for Go to read
#   3. Go's handlers/deal_handler.go reads deals table via store/deal_store.go
#   4. React frontend calls GET /api/deals/today → shows daily deals
#
# PYTHON CONCEPTS:
#   schedule library, while True loop, time.sleep,
#   dependency injection (manual), logging configuration
# ============================================================
import logging
import os
import schedule  # third-party: pip install schedule
import time
from dotenv import load_dotenv

# Load .env file for local development
# In Docker/EC2: environment variables are already set
load_dotenv()

# Configure logging before importing our modules
# This ensures all loggers (including imports) use our config
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(name)s] %(levelname)s: %(message)s"
)
logger = logging.getLogger(__name__)

# Import our layers — note the order: lowest level first
# models → repositories → analyzers (each depends on the ones above)
from analyzers.trend_analyzer import TrendAnalyzer
from analyzers.deal_finder import DealFinder
from repositories.card_repo import CardRepo
from repositories.listing_repo import ListingRepo

# ── Dependency Injection (Python style) ─────────────────────────
# Create low-level dependencies first (repos), then inject into analyzers.
# This is the SAME pattern as Go's main.go — build from the bottom up.
# store (repo) → service (analyzer)
card_repo    = CardRepo()
listing_repo = ListingRepo()

# Inject repos into analyzers — analyzers never create repos internally
trend_analyzer = TrendAnalyzer(repo=card_repo)
deal_finder    = DealFinder(card_repo=card_repo, listing_repo=listing_repo)


def run_trend_analysis():
    """
    Wrapper function for scheduled trend analysis.
    Wrapped in try/except so a crash doesn't stop the scheduler.
    
    WHY A WRAPPER?
    schedule.every().hour.do(trend_analyzer.run) would work,
    but then an unhandled exception would crash the while loop.
    By wrapping + catching, one failure just means that run is skipped.
    The next scheduled run still happens.
    """
    logger.info("Running trend analysis...")
    try:
        results = trend_analyzer.run()
        logger.info(f"✓ Trends computed for {len(results)} cards")
    except Exception as e:
        logger.error(f"Trend analysis failed: {e}")
        # DON'T re-raise — swallowed exceptions keep the scheduler alive


def run_deal_finder():
    """Wrapper for deal finder with error handling."""
    logger.info("Running deal finder...")
    try:
        deals = deal_finder.run()
        logger.info(f"✓ {len(deals)} deals found today")
    except Exception as e:
        logger.error(f"Deal finder failed: {e}")


if __name__ == "__main__":
    # Read configuration from environment variables
    # The schedule is configurable without code changes — DevOps can tune it
    trend_interval = int(os.getenv("TREND_ANALYSIS_INTERVAL_HOURS", "1"))
    deal_hour      = int(os.getenv("DEAL_OF_DAY_HOUR", "6"))

    # Run immediately on startup — don't wait for first scheduled time
    # This ensures the dashboard has data immediately after deployment
    run_trend_analysis()
    run_deal_finder()

    # schedule library API:
    # schedule.every(N).hours.do(function) = run function every N hours
    # schedule.every().day.at("06:00").do(function) = run daily at 6 AM
    schedule.every(trend_interval).hours.do(run_trend_analysis)
    # f"{deal_hour:02d}:00" = zero-padded 2-digit hour: 6 → "06:00"
    schedule.every().day.at(f"{deal_hour:02d}:00").do(run_deal_finder)

    logger.info(
        f"✓ Analytics engine running | "
        f"Trends: every {trend_interval}h | "
        f"Deals: daily at {deal_hour:02d}:00"
    )

    # THE SCHEDULER LOOP:
    # schedule.run_pending() checks if any scheduled jobs are due and runs them.
    # time.sleep(60) waits 60 seconds before checking again.
    # Resolution: jobs run within 60 seconds of their scheduled time.
    # For higher resolution: use sleep(1), but that uses more CPU.
    while True:              # infinite loop — runs forever until killed
        schedule.run_pending()
        time.sleep(60)       # check schedule every 60 seconds

# ============================================================
# TODO #1 (Practice): Add a news_scanner analyzer
# The original analytics engine had a news_scanner.py that scraped
# Pokémon-related news for price signals (e.g., "new Charizard promo announced").
# Create analyzers/news_scanner.py with a NewsScanner class.
# It should fetch RSS feeds from Pokebeach, PokeNews, TCGplayer blog.
# Identify mentions of specific card names and log the signal.
# Schedule it: schedule.every(6).hours.do(run_news_scanner)

# TODO #2 (Practice): Add a metrics report every 24 hours
# Add a function run_daily_report() that logs a summary:
#   - Total cards analyzed
#   - How many are RISING vs FALLING vs STABLE
#   - Today's top 5 trending cards (by trending_score DESC)
# Queries: SELECT trend_label, COUNT(*) FROM cards GROUP BY trend_label
#          SELECT name, trending_score FROM cards ORDER BY trending_score DESC LIMIT 5
# Schedule: schedule.every().day.at("23:00").do(run_daily_report)
# Future: send this as an email digest (use smtplib or SendGrid API)
# ============================================================
