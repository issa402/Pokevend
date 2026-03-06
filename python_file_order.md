# Python Backend: The Exact Order You Build Files
## And WHY That Order Is the FAANG Standard

---

## The Mental Model: Dependency-Driven Development

```
Domain Types (models/schemas.py — pure Pydantic, zero app imports)
       ↓
External I/O (repositories — raw HTTP/DB calls, return raw dicts)
       ↓
Infrastructure Clients (publisher — RabbitMQ wrapper)
       ↓
Business Logic (services — normalize, decide, publish)
       ↓
HTTP Surface (api/ routes — thin FastAPI decorators + 1 service call)
       ↓
Entry Point (main.py — DI wiring + lifespan ALWAYS LAST)
```

---

## Step 1: `requirements.txt` — Before ANY Python files

```txt
fastapi==0.110.0
uvicorn[standard]==0.28.0
pydantic==2.6.4
httpx==0.27.0
aio-pika==9.4.1
psycopg2-binary==2.9.9
python-dotenv==1.0.1
schedule==1.2.1
numpy==1.26.4
```

**Why first?** Every `import` in your code must be installed.
Pin exact versions — `==` not `>=` — for reproducible builds.
Think of it as `go.mod` for Python.

---

## Step 2: `models/schemas.py` — ALWAYS the First .py File

```python
from pydantic import BaseModel, field_validator
from typing import Optional
from datetime import datetime

class CardListing(BaseModel):
    """Message contract between Python (publisher) and Go (consumer)."""
    card_name: str
    price: float
    marketplace: str
    listing_url: str
    image_url: Optional[str] = None
    scraped_at: datetime = datetime.utcnow()

    @field_validator("price")
    @classmethod
    def price_must_be_positive(cls, v: float) -> float:
        if v <= 0:
            raise ValueError("price must be positive")
        return v
```

**Why first?** Schemas have ZERO dependencies on your app code.
Every other file (repos, services, routes) imports from here.
Without defined types, you don't know what shape your data is in.

**What belongs in schemas:**
- All Pydantic models the service uses
- Field validators (data rules that belong to the TYPE not the business logic)
- Nothing else — no HTTP, no SQL, no business rules

---

## Step 3: `repositories/*.py` — Raw External I/O

Write one file per external system. These are the ONLY files that touch external APIs or DBs directly.

```
repositories/
  ebay_repo.py        ← raw eBay API HTTP calls
  tcg_repo.py         ← raw TCGplayer API HTTP calls
  card_repo.py        ← PostgreSQL queries (analytics-engine)
  listing_repo.py     ← listing DB queries
```

**Inside each repo:**
1. Class + `__init__` (reads credentials from env vars directly)
2. Auth/token method
3. One method per API endpoint or SQL operation

```python
# repositories/ebay_repo.py
import os, httpx
from typing import Any, Dict, Optional

class EbayRepo:
    BASE_URL = "https://api.ebay.com"

    def __init__(self):
        self.client_id     = os.getenv("EBAY_CLIENT_ID", "")
        self.client_secret = os.getenv("EBAY_CLIENT_SECRET", "")
        self._token: Optional[str] = None

    async def get_token(self) -> str:
        if self._token:
            return self._token
        async with httpx.AsyncClient() as client:
            resp = await client.post(
                f"{self.BASE_URL}/identity/v1/oauth2/token",
                auth=(self.client_id, self.client_secret),
                data={"grant_type": "client_credentials", "scope": "..."},
            )
            resp.raise_for_status()
            self._token = resp.json()["access_token"]
        return self._token

    async def search_listings(self, card_name: str) -> Dict[str, Any]:
        token = await self.get_token()
        async with httpx.AsyncClient() as client:
            resp = await client.get(
                f"{self.BASE_URL}/buy/browse/v1/item_summary/search",
                headers={"Authorization": f"Bearer {token}"},
                params={"q": f"Pokemon card {card_name}", "limit": 50},
            )
            resp.raise_for_status()
            return resp.json()   # raw dict — service normalizes it
```

**Repository HARD rules:**
| ✅ Allowed | ❌ Not Allowed |
|---|---|
| HTTP requests (httpx) | Pydantic model construction |
| DB queries (psycopg2) | Business decisions (filtering) |
| Token fetching/caching | Publishing to RabbitMQ |
| Return raw `dict` | Logging business events |

**Return raw dicts, not Pydantic models.** The repo is shielded from your domain.
If eBay changes their response format, only the repo changes.

---

## Step 4: `publisher/rabbitmq_publisher.py` — Infrastructure Client

Write alongside repositories — same dependency level.

```python
import aio_pika, json, logging
from typing import Any, Dict, Optional

logger = logging.getLogger(__name__)

class RabbitMQPublisher:
    def __init__(self, url: str):
        self.url = url
        self._connection: Optional[aio_pika.Connection] = None
        self._channel: Optional[aio_pika.Channel] = None

    async def connect(self) -> None:
        self._connection = await aio_pika.connect_robust(self.url)
        self._channel    = await self._connection.channel()

    async def publish(self, queue_name: str, data: Dict[str, Any]) -> None:
        if not self._channel:
            return
        await self._channel.declare_queue(queue_name, durable=True)
        await self._channel.default_exchange.publish(
            aio_pika.Message(
                body=json.dumps(data).encode(),
                delivery_mode=aio_pika.DeliveryMode.PERSISTENT,
            ),
            routing_key=queue_name,
        )

    async def close(self) -> None:
        if self._connection and not self._connection.is_closed:
            await self._connection.close()
```

---

## Step 5: `services/*.py` — Business Logic

Write after ALL repos and publisher exist. Deps are injected via `__init__`.

```python
# services/ebay_service.py
import logging
from typing import List
from datetime import datetime

from models.schemas import CardListing
from publisher.rabbitmq_publisher import RabbitMQPublisher
from repositories.ebay_repo import EbayRepo

logger = logging.getLogger(__name__)

class EbayService:
    def __init__(self, repo: EbayRepo, publisher: RabbitMQPublisher):
        self.repo = repo          # INJECTED — never do EbayRepo() inside
        self.publisher = publisher

    async def scan_card(self, card_name: str) -> List[CardListing]:
        try:
            raw = await self.repo.search_listings(card_name)
        except Exception as e:
            logger.error(f"eBay search failed: {e}")
            return []

        listings: List[CardListing] = []
        for item in raw.get("itemSummaries", []):
            price = float(item.get("price", {}).get("value", 0))
            if price <= 0:      # business rule: skip zero-price listings
                continue
            listing = CardListing(
                card_name=card_name,
                price=price,
                marketplace="ebay",
                listing_url=item.get("itemWebUrl", ""),
                scraped_at=datetime.utcnow(),
            )
            listings.append(listing)
            await self.publisher.publish("listings", listing.model_dump(mode="json"))

        logger.info(f"Published {len(listings)} eBay listings for '{card_name}'")
        return listings
```

**Service HARD rules:**
| ✅ Allowed | ❌ Not Allowed |
|---|---|
| Business validation | HTTP request/response handling |
| Calling repositories | SQL queries |
| Creating Pydantic models | `os.getenv()` |
| Publishing to MQ | Creating repo instances internally |

---

## Step 6: `analyzers/*.py` — Compute Logic (analytics-engine variant)

Same as services but called by a scheduler, not HTTP.

```python
# analyzers/trend_analyzer.py
import numpy as np
from typing import List, Optional
from models.schemas import TrendResult
from repositories.card_repo import CardRepo

class TrendAnalyzer:
    def __init__(self, repo: CardRepo):
        self.repo = repo          # injected

    def run(self) -> List[TrendResult]:
        cards = self.repo.get_all_cards()
        results = []
        for card in cards:
            result = self._analyze_card(card)
            if result:
                self.repo.update_trend(card["card_id"], result)
                results.append(result)
        return results

    def _analyze_card(self, card: dict) -> Optional[TrendResult]:
        history = self.repo.get_price_history(card["card_id"], days=7)
        if len(history) < 2:
            return None
        prices = [float(h["avg_price"]) for h in history if h["avg_price"]]
        slope  = float(np.polyfit(range(len(prices)), prices, 1)[0])
        label  = "RISING" if slope > 1 else "FALLING" if slope < -1 else "STABLE"
        score  = max(-100, min(100, int(slope * 10)))
        return TrendResult(card_id=card["card_id"], trend_label=label, trending_score=score, pct_change_7d=0)
```

---

## Step 7: `api/*.py` — FastAPI Routes (thinnest layer)

```python
# api/webhook.py
from fastapi import APIRouter, HTTPException, Request

router = APIRouter(tags=["Webhook"])

@router.post("/webhook")
async def ebay_webhook(request: Request):
    try:
        payload = await request.json()
    except Exception:
        raise HTTPException(status_code=400, detail="Invalid JSON")
    return {"status": "accepted", "event": payload.get("notificationType")}
```

Route rules — each route should:
1. Parse the request (one line)
2. Call ONE service method (one line)
3. Return the result (one line)

That's it. 3-5 lines per route. Business logic in services, SQL in repos.

---

## Step 8: `main.py` — Entry Point (ALWAYS LAST)

```python
import asyncio, logging, os
from contextlib import asynccontextmanager
from fastapi import FastAPI
from dotenv import load_dotenv

from api.webhook import router as webhook_router
from publisher.rabbitmq_publisher import RabbitMQPublisher
from repositories.ebay_repo import EbayRepo
from services.ebay_service import EbayService

load_dotenv()
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

@asynccontextmanager
async def lifespan(app: FastAPI):
    publisher = RabbitMQPublisher(os.getenv("RABBITMQ_URL"))
    await publisher.connect()

    # DI: build from bottom up (repo → service)
    ebay_repo = EbayRepo()
    ebay_svc  = EbayService(repo=ebay_repo, publisher=publisher)

    asyncio.create_task(scanner_loop(ebay_svc))
    logger.info("✓ Ready")

    yield  # app runs here

    await publisher.close()

app = FastAPI(title="api-consumer", lifespan=lifespan)
app.include_router(webhook_router)
```

**main.py ONLY does:**
1. `load_dotenv()` — env vars
2. Logging config
3. Create infrastructure (RabbitMQ, DB pool)
4. Create repos (no deps on app code)
5. Create services (inject repos)
6. Start background tasks
7. Register routers

Zero business logic. Zero SQL. Zero HTTP calls.

---

## Complete File Creation Order

```
Phase 1: Declarations
  1.  requirements.txt
  2.  .env.example
  3.  Dockerfile

Phase 2: Domain Types
  4.  models/schemas.py              ← ALWAYS FIRST .py file

Phase 3: External I/O (same dependency level — write in any order)
  5.  repositories/ebay_repo.py
  6.  repositories/tcg_repo.py
  7.  repositories/card_repo.py
  8.  publisher/rabbitmq_publisher.py

Phase 4: Business Logic
  9.  services/ebay_service.py
  10. services/tcg_service.py
  11. analyzers/trend_analyzer.py    ← analytics-engine
  12. analyzers/deal_finder.py       ← analytics-engine

Phase 5: HTTP Surface
  13. api/webhook.py                 ← FastAPI routes only

Phase 6: Wiring
  14. main.py                        ← ALWAYS LAST
```

---

## The Rule Behind All of This

```
main.py      → imports everything
services/    → imports repos, publisher, schemas
repositories → imports schemas (type hints), httpx, psycopg2
publisher/   → imports aio_pika only
schemas.py   → imports pydantic only (no app imports at all)
```

If adding an import creates a cycle (A → B → A), your design is wrong.
The bottom-up creation order prevents circular imports by construction.
