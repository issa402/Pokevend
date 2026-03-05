# ============================================================
# api-consumer/main.py — FastAPI application entry point
# Replaces the old Flask app. Async, typed, auto-docs at /docs.
# ============================================================
import asyncio
import logging
import os
from contextlib import asynccontextmanager

from fastapi import FastAPI
from dotenv import load_dotenv

from api.webhook import router as webhook_router
from publisher.rabbitmq_publisher import RabbitMQPublisher
from repositories.ebay_repo import EbayRepo
from repositories.tcg_repo import TCGRepo
from services.ebay_service import EbayService
from services.tcg_service import TCGService

load_dotenv()
logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(name)s] %(levelname)s: %(message)s")
logger = logging.getLogger(__name__)

# Global service instances (dependency-injected into routes)
publisher: RabbitMQPublisher = None
ebay_svc:  EbayService       = None
tcg_svc:   TCGService        = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Startup and shutdown lifecycle."""
    global publisher, ebay_svc, tcg_svc

    # ── Startup ────────────────────────────────────────────────
    rabbitmq_url = os.getenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672")
    publisher = RabbitMQPublisher(rabbitmq_url)
    try:
        await publisher.connect()
        logger.info("✓ RabbitMQ publisher connected")
    except Exception as e:
        logger.warning(f"⚠️  RabbitMQ unavailable: {e}")

    # Wire: repo → service (dependency injection)
    ebay_svc = EbayService(repo=EbayRepo(), publisher=publisher)
    tcg_svc  = TCGService(repo=TCGRepo(), publisher=publisher)

    # Start background scanner loop
    asyncio.create_task(scanner_loop())

    yield  # App runs here

    # ── Shutdown ───────────────────────────────────────────────
    if publisher:
        await publisher.close()
    logger.info("api-consumer shut down cleanly")


app = FastAPI(
    title="PokémonTool — API Consumer",
    description="Fetches Pokémon card listings from eBay and TCGplayer, publishes to RabbitMQ.",
    version="2.0.0",
    lifespan=lifespan,
)

# Register routes
app.include_router(webhook_router, prefix="", tags=["eBay Webhook"])


async def scanner_loop():
    """
    Background task: continuously scan popular cards on eBay and TCGplayer.
    Interval is configured via SCRAPING_INTERVAL_MINUTES env var.
    """
    interval = int(os.getenv("SCRAPING_INTERVAL_MINUTES", "30")) * 60
    # eBay popular Pokémon cards to monitor
    WATCH_LIST = [
        "Charizard Base Set", "Pikachu Illustrator", "Blastoise Base Set",
        "Mewtwo Base Set", "Umbreon Gold Star", "Rayquaza Gold Star",
        "Lugia Neo Genesis", "Charizard VMAX", "Pikachu VMAX Rainbow",
    ]
    while True:
        for card in WATCH_LIST:
            try:
                await ebay_svc.scan_card(card)
                await tcg_svc.scan_card(card)
            except Exception as e:
                logger.error(f"Scanner error for '{card}': {e}")
            await asyncio.sleep(1)  # Polite delay between cards
        logger.info(f"Scan complete. Sleeping {interval}s...")
        await asyncio.sleep(interval)


if __name__ == "__main__":
    import uvicorn
    uvicorn.run("main:app", host="0.0.0.0", port=8001, reload=False)
