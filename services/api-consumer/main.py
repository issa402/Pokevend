# ============================================================
# FILE: services/api-consumer/main.py
# TYPE: FastAPI Application Entry Point
#
# WHAT IS THIS?
# The entry point for the api-consumer microservice.
# This service's job: poll eBay and TCGplayer APIs for card listings,
# normalize the data, and publish it to RabbitMQ for Go to process.
#
# FAANG MICROSERVICE PATTERN:
# Each service does ONE thing — api-consumer only collects listings.
# It does NOT analyze trends (that's analytics-engine).
# It does NOT send alerts (that's Go's notification_worker).
# It does NOT serve HTTP endpoints to the frontend (that's Go API).
#
# FASTAPI ADVANTAGES OVER FLASK:
#   - Fully async (handles thousands of concurrent API calls)
#   - Auto-generates /docs (Swagger UI) — no additional tools needed
#   - Pydantic validation built-in (type-safe request/response)
#   - 2-3x faster than Flask for I/O-bound workloads
#
# KEY PYTHON CONCEPTS DEMONSTRATED:
#   async/await, asynccontextmanager (lifespan), asyncio.create_task,
#   dependency injection (manual), logging, os.getenv defaults
# ============================================================
import asyncio
import logging
import os
import httpx
from contextlib import asynccontextmanager


from fastapi import FastAPI, Query, Response, status
from dotenv import load_dotenv

# Local package imports — each is a separate directory in this service
from api.webhook import router as webhook_router
from publisher.rabbitmq_publisher import RabbitMQPublisher
from repositories.ebay_repo import EbayRepo
from repositories.tcg_repo import TCGRepo
from services.ebay_service import EbayService
from services.tcg_service import TCGService

# Load .env file into os.environ (development only)
# In production on EC2, environment variables are set via docker-compose.yml
load_dotenv()

# Configure structured logging
# %(asctime)s = timestamp, %(name)s = logger name (module), %(levelname)s = INFO/WARNING/ERROR
# Use logging.INFO in production — DEBUG produces too much output
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(name)s] %(levelname)s: %(message)s"
)
logger = logging.getLogger(__name__)
# __name__ = "main" when this file runs — helps identify log sources

# Global service instances — created in lifespan, used in scanner_loop
# These are module-level globals (acceptable pattern for singletons in Python)
publisher: RabbitMQPublisher = None
ebay_svc: EbayService = None
tcg_svc: TCGService = None

MAJOR_SLAB_TIERS = [
    "PSA_10", "PSA_9", "PSA_8", "PSA_7",
    "CGC_10", "CGC_9_5", "CGC_9",
    "BGS_10", "BGS_9_5", "BGS_9",
]


@asynccontextmanager
async def lifespan(app: FastAPI):
    """
    LIFESPAN = startup + shutdown logic for FastAPI.
    
    PYTHON CONCEPT: @asynccontextmanager
    This decorator wraps an async generator function as a context manager.
    The code before 'yield' runs on startup.
    The code after 'yield' runs on shutdown (even if an exception occurs).
    
    FAANG PATTERN: "Lifespan Pattern" for resource management
    - Open connections on startup, close them on shutdown
    - Don't initialize global resources in module scope (non-testable)
    - This pattern is dependency injection for the startup process
    """
    global publisher, ebay_svc, tcg_svc  # modify the module-level globals

    # ── STARTUP (runs before accepting any HTTP requests) ─────────
    rabbitmq_url = os.getenv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672")
    publisher = RabbitMQPublisher(rabbitmq_url)
    
    try:
        await publisher.connect()
        logger.info("✓ RabbitMQ publisher connected")
    except Exception as e:
        # Don't crash on startup if RabbitMQ is unavailable
        # The scanner loop will just fail silently and retry
        logger.warning(f"⚠️  RabbitMQ unavailable: {e}")

    # DEPENDENCY INJECTION (Python style):
    # We create repos first, then inject them into services
    # This is the same DI pattern as Go's main.go — just without a DI framework
    ebay_svc = EbayService(repo=EbayRepo(), publisher=publisher)
    tcg_svc = TCGService(repo=TCGRepo(), publisher=publisher)

    # asyncio.create_task() = run scanner_loop as a background coroutine
    # It runs concurrently with the HTTP server (like Go's "go" keyword)
    # The task continues running after lifespan's yield
    asyncio.create_task(scanner_loop())
    logger.info("✓ Background scanner started")

    yield  # ← APP IS RUNNING HERE (serving HTTP requests) ─────────

    # ── SHUTDOWN (runs after last HTTP request is done) ───────────
    if publisher:
        await publisher.close()
        logger.info("RabbitMQ publisher closed")
    logger.info("api-consumer shut down cleanly")


# Create FastAPI app with metadata
# title/description appear in the auto-generated /docs UI
app = FastAPI(
    title="PokémonTool — API Consumer",
    description=(
        "Scans eBay and TCGplayer for Pokémon card listings. "
        "Publishes normalized listings to RabbitMQ for Go to process into alerts."
    ),
    version="2.0.0",
    lifespan=lifespan,  # use our startup/shutdown logic
)

# Include the webhook router — registers POST /webhook route
# prefix="" means the route is /webhook (not /api/webhook)
app.include_router(webhook_router, prefix="", tags=["eBay Webhook"])


async def scanner_loop():
    """
    Background coroutine: scan popular cards on eBay and TCGplayer.
    Runs forever with a configurable delay between full scans.
    
    PYTHON ASYNC CONCEPT:
    await asyncio.sleep(n) suspends THIS coroutine and lets OTHER code run.
    It's non-blocking — unlike time.sleep() which blocks the entire thread.
    FastAPI can serve HTTP requests while this loop is sleeping.
    """
    # Time between full scan cycles — configurable via environment variable
    interval = int(os.getenv("SCRAPING_INTERVAL_MINUTES", "30")) * 60  # convert to seconds

    # Docker mode: use the Compose service name "server".
    # Local host-run mode: change GO_INTERNAL_URL in .env to
    # http://localhost:3001/api/internal/watchlist-names
    GO_INTERNAL_URL = os.getenv(
        "GO_INTERNAL_URL",
        "http://server:3001/api/internal/watchlist-targets",
    )
    # The cards we actively monitor for price changes
    # In production: fetch these from PostgreSQL watchlists table instead

    while True: # Everything MUST be inside this loop
        try:
            async with httpx.AsyncClient() as client:
                resp = await client.get(GO_INTERNAL_URL)
                resp.raise_for_status()
                # Use .get('values', []) logic or 'or []' to prevent NoneType errors
                watch_list = resp.json() 
                if watch_list is None:
                    watch_list = []
                    
            logger.info(f"Watchlist successfully retrieved {len(watch_list)} cards to scan")

            # Move the scanning inside the TRY so it only runs if fetch succeeded
            for target in watch_list:
                if isinstance(target, dict):
                    card = target.get("cardName") or target.get("card_name")
                    external_card_id = target.get("externalCardId") or target.get("external_card_id")
                    set_name = target.get("setName") or target.get("set_name")
                    asset_type = target.get("assetType") or target.get("asset_type") or "RAW"
                    slab_tier = target.get("slabTier") or target.get("slab_tier")
                    language_preference = target.get("languagePreference") or target.get("language_preference") or "BOTH"
                else:
                    card = target
                    external_card_id = None
                    set_name = None
                    asset_type = "RAW"
                    slab_tier = None
                    language_preference = "BOTH"
                if not card:
                    continue
                try:
                    if asset_type == "ALL_SLABS":
                        for tier in MAJOR_SLAB_TIERS:
                            await ebay_svc.scan_card(
                                card,
                                external_card_id=external_card_id,
                                set_name=set_name,
                                asset_type="SLAB",
                                slab_tier=tier,
                                language_preference=language_preference,
                                max_pages=1,
                            )
                            await asyncio.sleep(1)
                    else:
                        await ebay_svc.scan_card(
                            card,
                            external_card_id=external_card_id,
                            set_name=set_name,
                            asset_type=asset_type,
                            slab_tier=slab_tier,
                            language_preference=language_preference,
                            max_pages=1,
                        ) # Fixed method name to scan_card
                    await tcg_svc.scan_card(card)
                except Exception as e:
                    logger.error(f"No apis for '{card}' : {e}")
                await asyncio.sleep(1)

        except Exception as e:
            logger.error(f"Failed to sync watchlist: {e}")
            await asyncio.sleep(10) # Retry quickly after startup/DNS races.
            continue

        # CRITICAL: This MUST be indented inside the 'while True' loop
        logger.info(f"✅ Scan cycle complete. Sleeping {interval}s...")
        await asyncio.sleep(interval)
    

# Entry point when running directly: python main.py
# In Docker: CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8001"]
if __name__ == "__main__":
    import uvicorn
    # uvicorn = the ASGI server that runs FastAPI
    # reload=False in production (True only in dev for auto-restart on code changes)
    uvicorn.run("main:app", host="0.0.0.0", port=8001, reload=False)

# ============================================================
# TODO #1 (Practice): Fetch watchlist from PostgreSQL instead of hardcoded list
# The current WATCH_LIST is hardcoded in the scanner loop.
# In production, you'd read from the watchlists table in PostgreSQL:
#   SELECT DISTINCT card_name FROM watchlists
# Add a PostgreSQL connection using psycopg2 (or asyncpg for async)
# and replace WATCH_LIST with a database query result.
# Update the scan every cycle so new watchlist additions are picked up.
# HINT: Create a WatchlistRepo in repositories/watchlist_repo.py

@app.get("/health")
async def health(response: Response):
    is_rabbitmq_ok = publisher is not None and publisher._channel is not None
    result = {
        "status" : "ok" if is_rabbitmq_ok else "unhealthy",
        "service" : "api-consumer",
        "rabbitmq" : "connected" if is_rabbitmq_ok else "not connected"
    }

    if not is_rabbitmq_ok:
        response.status_code = status.HTTP_503_SERVICE_UNAVAILABLE
        return result
    return result


@app.get("/ebay/search")
async def ebay_search(
    cardName: str = Query(..., min_length=1),
    externalCardId: str | None = None,
    setName: str | None = None,
    assetType: str = "RAW",
    slabTier: str | None = None,
    languagePreference: str = "BOTH",
    pages: int = 2,
    publish: bool = False,
):
    listings = await ebay_svc.scan_card(
        cardName,
        external_card_id=externalCardId,
        set_name=setName,
        asset_type=assetType,
        slab_tier=slabTier,
        publish=publish,
        language_preference=languagePreference,
        max_pages=pages,
    )
    return {
        "count": len(listings),
        "listings": [listing.model_dump(mode="json") for listing in listings],
    }



# TODO #2 (Practice): Add /health endpoint to FastAPI
# Add: @app.get("/health")
# async def health():
#     return {"status": "ok", "service": "api-consumer"}
# Make it also check if RabbitMQ is connected (publisher._channel is not None).
# Return 503 if RabbitMQ is down (unhealthy but running).
# This endpoint is called by Docker health checks and AWS load balancers.
# ============================================================
