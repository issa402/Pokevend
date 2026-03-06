# Python Backend Engineering Guide
## Everything You Need to Know to Be an Insane Python Backend Engineer

---

## 1. Python Fundamentals for Backend

### Variables & Types
```python
# Python is dynamically typed — no type declarations required
# But we ADD type hints everywhere for clarity and tooling support

x: int = 42
name: str = "Charizard"
price: float = 89.99
found: bool = True

# None = the Python equivalent of null/nil
user: User | None = None
# (also written as: Optional[User] — same thing, older syntax)

# CONSTANTS: Python doesn't enforce constants, convention is UPPER_SNAKE_CASE
MAX_RETRIES = 3
BASE_URL = "https://api.ebay.com"
```

### Type Hints — Mandatory at FAANG
```python
from typing import Optional, List, Dict, Any, Tuple, Union
from datetime import datetime

# Function signatures WITH type hints — always do this
def get_card(card_id: str, limit: int = 10) -> Optional[Card]:
    ...

# Optional[str] = str | None (can be string or nothing)
def get_user(email: str) -> Optional[User]: ...

# List[str] = a list of strings
def get_card_names() -> List[str]: ...

# Dict[str, Any] = dict with string keys and any value type
def parse_ebay_response(raw: Dict[str, Any]) -> List[CardListing]: ...

# Tuple[int, str] = a fixed-length tuple
def get_id_and_name() -> Tuple[int, str]: ...

# Python 3.10+ shorthand (modern Python):
def old_style(x: Optional[str]) -> None: ...   # old
def new_style(x: str | None) -> None: ...      # new (same thing)
```

### Functions
```python
# Basic function
def add(a: int, b: int) -> int:
    return a + b

# Default arguments
def search(query: str, limit: int = 20) -> List[Card]:
    ...

# *args (variable positional) and **kwargs (variable keyword)
def log(*messages: str, level: str = "INFO") -> None:
    for msg in messages:
        print(f"[{level}] {msg}")

# Lambda (anonymous function — for simple cases)
prices = [10.5, 20.0, 5.99]
highest = max(prices, key=lambda p: p)  # key= takes a function

# Comprehensions (most Pythonic way to build lists/dicts/sets)
prices = [float(item["price"]) for item in results]              # list comprehension
valid  = [p for p in prices if p > 0]                           # with filter
lookup = {card["id"]: card["name"] for card in cards}           # dict comprehension
unique = {card["set_name"] for card in cards}                   # set comprehension
```

### Classes & OOP
```python
class CardService:
    """Docstring: explains what the class does."""

    # Class constant
    MAX_RESULTS = 100

    def __init__(self, repo: EbayRepo, publisher: RabbitMQPublisher):
        """Constructor — Python's version of NewCardService()"""
        self.repo = repo            # instance attributes (like Go struct fields)
        self.publisher = publisher
        self._cache: dict = {}      # private (convention: _underscore prefix)

    def scan_card(self, card_name: str) -> List[CardListing]:
        """Public method"""
        ...

    def _normalize(self, raw: dict) -> CardListing:
        """Private method (convention)"""
        ...

    @classmethod
    def from_config(cls, config: Config) -> "CardService":
        """Alternative constructor — creates instance from config"""
        return cls(repo=EbayRepo(config), publisher=RabbitMQPublisher(config))

    @staticmethod
    def validate_price(price: float) -> bool:
        """Static method — doesn't need self or cls"""
        return 0 < price < 10_000

# Inheritance (less used in our codebase — prefer composition)
class FancyCardService(CardService):
    def scan_card(self, card_name: str) -> List[CardListing]:
        results = super().scan_card(card_name)  # call parent
        return self._filter_fancy(results)
```

---

## 2. Async / Await — The Heart of FastAPI

```python
# sync vs async:
# SYNC: blocks the entire thread until done
def get_data() -> str:
    time.sleep(2)   # BLOCKS everything for 2 full seconds
    return data

# ASYNC: suspends and lets other code run during the wait
async def get_data() -> str:
    await asyncio.sleep(2)   # suspend HERE, let other requests run
    return data

# ── Rules ──────────────────────────────────────────────────
# 1. async def = this function is a coroutine
# 2. await = suspend here until the awaited thing is done
# 3. await can ONLY be used inside async def
# 4. Calling an async function WITHOUT await returns a coroutine object (not the result!)

# ── Common async patterns ──────────────────────────────────
async def fetch_from_apis() -> dict:
    # Sequential (slower — awaits one at a time)
    ebay_result  = await ebay_repo.search("Charizard")
    tcg_result   = await tcg_repo.get_price("Charizard")

    # Concurrent (faster — runs both at same time)
    ebay_task = asyncio.create_task(ebay_repo.search("Charizard"))
    tcg_task  = asyncio.create_task(tcg_repo.get_price("Charizard"))
    ebay_result, tcg_result = await asyncio.gather(ebay_task, tcg_task)
    # asyncio.gather() = run multiple coroutines concurrently, collect results

# ── asyncio.create_task() ─────────────────────────────────
# Schedules a coroutine to run in the background
# Like Go's "go" keyword:
#   Go:     go worker.Run()
#   Python: asyncio.create_task(worker.run())

# ── in FastAPI ────────────────────────────────────────────
@app.get("/cards")
async def list_cards():
    cards = await card_service.get_all()   # await the service
    return cards

# RULE: FastAPI can handle sync (def) routes too, but use async when possible
# async def: FastAPI puts it on the async event loop (good for I/O bound work)
# def:        FastAPI runs it in a thread pool (good for CPU bound work)
```

---

## 3. Pydantic — Type-Safe Data Models

```python
from pydantic import BaseModel, field_validator
from typing import Optional
from datetime import datetime

class CardListing(BaseModel):
    # Required fields (no default)
    card_name: str
    price: float
    marketplace: str

    # Optional fields (None by default)
    image_url: Optional[str] = None
    condition: Optional[str] = None

    # Default value using a factory function
    scraped_at: datetime = datetime.utcnow()

    # Field validator (validates on instantiation)
    @field_validator("price")
    @classmethod
    def price_must_be_positive(cls, v: float) -> float:
        if v <= 0:
            raise ValueError("price must be positive")
        return v

    @field_validator("card_name")
    @classmethod
    def normalize_name(cls, v: str) -> str:
        return v.strip().title()  # "  charizard  " → "Charizard"

# Instantiation (automatic validation!)
listing = CardListing(card_name="charizard", price=89.99, marketplace="ebay")
# listing.card_name == "Charizard"  (normalized by validator)

# This FAILS → ValidationError:
bad = CardListing(card_name="Test", price=-5, marketplace="ebay")

# Serialization
listing.model_dump()               # → Python dict
listing.model_dump(mode="json")    # → dict with JSON-safe types (datetime → string)
listing.model_dump_json()          # → JSON string directly

# Parsing from dict (e.g., from RabbitMQ message)
listing = CardListing.model_validate(raw_dict)
```

---

## 4. FastAPI — Modern Python Web Framework

```python
from fastapi import FastAPI, APIRouter, Depends, HTTPException, Request, status

# App creation
app = FastAPI(title="PokémonTool", version="2.0.0", lifespan=lifespan)

# ── Route decorators ───────────────────────────────────────
@app.get("/items")         # GET /items
@app.post("/items")        # POST /items
@app.put("/items/{id}")    # PUT /items/{id}
@app.delete("/items/{id}") # DELETE /items/{id}

# ── Path parameters ────────────────────────────────────────
@app.get("/cards/{card_id}")
async def get_card(card_id: str):  # {card_id} from URL
    ...

# ── Query parameters ───────────────────────────────────────
@app.get("/cards/search")
async def search_cards(q: str, limit: int = 20):  # ?q=...&limit=...
    # q is REQUIRED (no default) → 422 if missing
    # limit is OPTIONAL (has default)
    ...

# ── Request body (Pydantic model) ─────────────────────────
class CreateCardRequest(BaseModel):
    name: str
    price: float

@app.post("/cards")
async def create_card(body: CreateCardRequest):
    # FastAPI auto-parses JSON → CreateCardRequest
    # Auto-validates (returns 422 if invalid)
    return {"id": "new-uuid", "name": body.name}

# ── HTTP exceptions ────────────────────────────────────────
raise HTTPException(status_code=404, detail="Card not found")
raise HTTPException(status_code=401, detail="Not authenticated")
raise HTTPException(status_code=409, detail="Email already registered")

# ── APIRouter (grouping routes) ────────────────────────────
router = APIRouter(prefix="/api/cards", tags=["Cards"])

@router.get("/trending")
async def trending(): ...

app.include_router(router)  # registers all router routes on app

# ── Lifespan (startup + shutdown) ─────────────────────────
from contextlib import asynccontextmanager

@asynccontextmanager
async def lifespan(app: FastAPI):
    # STARTUP: runs before accepting requests
    db = await connect_database()
    rabbitmq = await connect_rabbitmq()
    asyncio.create_task(background_worker())
    
    yield  # ← app is running here, accepting requests
    
    # SHUTDOWN: runs after last request
    await db.close()
    await rabbitmq.close()

app = FastAPI(lifespan=lifespan)

# ── Dependency Injection ───────────────────────────────────
def get_db():
    """FastAPI dependency — called for each request"""
    db = SessionLocal()
    try:
        yield db   # provide db to handler
    finally:
        db.close() # cleanup after handler returns

@app.get("/cards")
async def list_cards(db = Depends(get_db)):  # FastAPI injects db
    ...
```

---

## 5. psycopg2 — PostgreSQL Driver

```python
import psycopg2
import psycopg2.extras  # for RealDictCursor

# ── Connect ────────────────────────────────────────────────
conn = psycopg2.connect(
    host="localhost", port=5432,
    dbname="pokemontool", user="user", password="pass"
)

# ── Query with context manager ─────────────────────────────
with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
    # RealDictCursor: rows as dicts {"id": 1, "name": "Charizard"}
    # Without: rows as tuples (1, "Charizard") — harder to use

    # SELECT (multiple rows)
    cur.execute("SELECT id, name FROM cards WHERE name ILIKE %s", ("%char%",))
    rows = cur.fetchall()     # list of dicts
    row  = cur.fetchone()     # just the first row

    # INSERT
    cur.execute(
        "INSERT INTO cards (name, price) VALUES (%s, %s) RETURNING id",
        ("Charizard", 89.99)
    )
    new_id = cur.fetchone()["id"]  # get the generated UUID
    conn.commit()          # REQUIRED for writes! else rolled back

    # UPDATE
    cur.execute("UPDATE cards SET price=%s WHERE id=%s", (99.99, card_id))
    conn.commit()

    # NEVER use f-strings for SQL — ALWAYS use %s:
    # SAFE:   cur.execute("SELECT ... WHERE id=%s", (card_id,))
    # UNSAFE: cur.execute(f"SELECT ... WHERE id={card_id}")  # SQL INJECTION!

# ── Connection pool ────────────────────────────────────────
from psycopg2 import pool

connection_pool = pool.SimpleConnectionPool(
    minconn=1,
    maxconn=10,
    host="localhost", dbname="db", user="user", password="pass"
)

conn = connection_pool.getconn()    # get connection from pool
# use conn ...
connection_pool.putconn(conn)       # always return to pool!
```

---

## 6. httpx — Async HTTP Client

```python
import httpx
from typing import Any, Dict

# ── Async request (for FastAPI/async code) ─────────────────
async def fetch_data(url: str) -> Dict[str, Any]:
    async with httpx.AsyncClient() as client:
        # GET
        resp = await client.get(url, params={"q": "charizard"})
        resp.raise_for_status()   # raises HTTPStatusError if 4xx/5xx
        return resp.json()

        # POST with JSON body
        resp = await client.post(url, json={"name": "test"})

        # POST with form data (OAuth2 token endpoints)
        resp = await client.post(url, data={"grant_type": "client_credentials"})

        # With auth
        resp = await client.get(url, auth=("user", "pass"))          # Basic Auth
        resp = await client.get(url, headers={"Authorization": f"Bearer {token}"})

# ── Sync request (for analytics-engine which uses schedule + sync) ────
import httpx  # sync version

with httpx.Client() as client:
    resp = client.get(url)
    resp.raise_for_status()
    data = resp.json()

# ── Response attributes ────────────────────────────────────
resp.status_code        # 200, 404, etc
resp.json()             # parse JSON body → dict
resp.text               # body as string
resp.content            # body as bytes
resp.headers            # dict of response headers
resp.raise_for_status() # raise httpx.HTTPStatusError if error status
```

---

## 7. aio_pika — Async RabbitMQ

```python
import aio_pika, json

# ── Connect ────────────────────────────────────────────────
conn = await aio_pika.connect_robust("amqp://guest:guest@localhost:5672")
ch   = await conn.channel()

# ── Declare queue (idempotent) ───────────────────────────
q = await ch.declare_queue("listings", durable=True)
# durable=True: queue survives RabbitMQ restart

# ── Publish ───────────────────────────────────────────────
await ch.default_exchange.publish(
    aio_pika.Message(
        body=json.dumps({"card_name": "Charizard", "price": 89.99}).encode(),
        delivery_mode=aio_pika.DeliveryMode.PERSISTENT,  # survive restart
    ),
    routing_key="listings",  # which queue
)

# ── Consume ───────────────────────────────────────────────
async with q.iterator() as msgs:
    async for msg in msgs:
        async with msg.process():   # auto-ack on exit, nack on exception
            data = json.loads(msg.body)
            await process(data)
```

---

## 8. The 4-Layer Python Architecture

```
HTTP Request (FastAPI route)
      │
      ▼
┌─────────────────────────────────────────────┐
│  api/ (or webhook.py)                       │  Layer 4
│  Route decorator + request parsing          │
│  Calls service, returns response             │
└─────────────────────────────────────────────┘
      │
      ▼
┌─────────────────────────────────────────────┐
│  services/                                  │  Layer 3
│  Business logic: filter, normalize, validate │
│  Calls repo(s), publisher                   │
└─────────────────────────────────────────────┘
      │
      ▼
┌─────────────────────────────────────────────┐
│  repositories/  (and publisher/)            │  Layer 2
│  Raw API calls / DB queries / RabbitMQ      │
│  Returns raw data (dicts)                   │
└─────────────────────────────────────────────┘
      │
      ▼
┌─────────────────────────────────────────────┐
│  models/schemas.py                          │  Layer 1
│  Pydantic models — data shape + validation  │
└─────────────────────────────────────────────┘
```

---

## 9. Python Essentials

```python
# ── Exception handling ────────────────────────────────────
try:
    result = risky_operation()
except ConnectionError as e:
    logger.error(f"Connection failed: {e}")
    return None  # graceful degradation
except (ValueError, TypeError) as e:
    raise HTTPException(status_code=400, detail=str(e))
except Exception as e:
    logger.error(f"Unexpected error: {e}")
    raise  # re-raise unknown errors (don't silently swallow)
finally:
    cleanup()  # always runs (like Go's defer)

# Custom exceptions:
class EbayAuthError(Exception): pass
class RateLimitError(Exception): pass
raise EbayAuthError("Token expired")

# ── Logging ───────────────────────────────────────────────
import logging
logger = logging.getLogger(__name__)   # __name__ = module path
logger.debug("Detailed info")          # not shown in production
logger.info("Normal operations")
logger.warning("Something unexpected")
logger.error("Something failed")
logger.critical("Service is down")

# Structured logging (JSON — logs parseable by Datadog, CloudWatch):
import json
logger.info(json.dumps({"event": "listing_published", "card": "Charizard", "price": 89.99}))

# ── Environment variables ─────────────────────────────────
import os
from dotenv import load_dotenv

load_dotenv()   # loads .env file into os.environ

db_host = os.getenv("POSTGRES_HOST", "localhost")  # default if not set
secret  = os.getenv("JWT_SECRET")                  # None if not set
if not secret:
    raise ValueError("JWT_SECRET must be set!")

# ── f-strings (string formatting) ────────────────────────
name = "Charizard"
price = 89.99
msg = f"Card: {name}, Price: ${price:.2f}"  # {variable:.2f} = 2 decimal places
url = f"https://ebay.com/search?q={name.lower().replace(' ', '+')}"

# ── List/dict operations ──────────────────────────────────
prices = [10.5, 20.0, 5.99, 15.0]
max(prices)                    # 20.0
min(prices)                    # 5.99
sum(prices)                    # 51.49
sorted(prices)                 # [5.99, 10.5, 15.0, 20.0]
sorted(prices, reverse=True)   # [20.0, 15.0, 10.5, 5.99]

# Dict operations
d = {"a": 1, "b": 2}
d.get("c", 0)      # 0 (safe — returns default, no KeyError)
d["c"] = 3         # add/update
del d["a"]         # delete
list(d.keys())     # ["b", "c"]
list(d.values())   # [2, 3]
list(d.items())    # [("b", 2), ("c", 3)]

# ── Decorators ────────────────────────────────────────────
# Decorators wrap functions — add behavior without modifying them
@app.get("/cards")           # FastAPI route decorator
@field_validator("price")    # Pydantic validation decorator
@asynccontextmanager         # makes a generator a context manager
@classmethod                 # makes a class method (first arg = cls, not self)
@staticmethod                # no self or cls
```

---

## 10. Python Gotchas for Backend Devs

```python
# 1. Mutable default argument bug (very common!)
# WRONG: list is shared across all calls
def append_item(item, lst=[]):
    lst.append(item)
    return lst

# RIGHT: use None as default, create new list inside
def append_item(item, lst=None):
    if lst is None:
        lst = []
    lst.append(item)
    return lst

# 2. time.sleep() vs asyncio.sleep()
# In async code, ALWAYS use asyncio.sleep() — time.sleep() blocks the event loop!
async def wait():
    await asyncio.sleep(1)   # CORRECT — suspends coroutine, lets other run
    time.sleep(1)            # WRONG — blocks ALL async code for 1 second

# 3. Dict.get() vs dict["key"]
d = {"a": 1}
d["b"]       # KeyError: 'b' (crash if key missing)
d.get("b")   # None (safe return if key missing)
d.get("b", 0) # 0 (default value if key missing)

# 4. None checks
x = None
if x is None: ...    # CORRECT — use 'is' for None comparison
if x == None: ...    # works but not idiomatic Python

# 5. Truthiness
if x:   # False for: None, 0, "", [], {}, set()
        # True for: anything else (non-zero, non-empty)
```

---

## 11. Production Readiness Checklist (Python)

```
Architecture:
  ✅ 4-layer: models → repositories → services → api
  ✅ Dependency injection (repos injected into services via __init__)
  ✅ No business logic in repositories (only raw API/DB calls)
  ✅ Pydantic models for all data crossing service boundaries

FastAPI:
  ✅ Lifespan pattern for startup/shutdown
  ✅ Background tasks via asyncio.create_task()
  ✅ Input validation via Pydantic (automatic 422 on bad input)
  ✅ /health endpoint for load balancer checks

Async/IO:
  ✅ asyncio.sleep() (never time.sleep()) in async code
  ✅ async with / async for for async context managers and iterators
  ✅ httpx.AsyncClient() for async HTTP calls
  ✅ aio_pika for async RabbitMQ

Security:
  ✅ No hardcoded credentials (always os.getenv())
  ✅ Parameterized SQL queries (always %s, never f-strings in SQL)
  ✅ Secrets loaded from .env / environment
  ✅ HTTPS in production (handled at nginx/load balancer level)

Still To Do (your practice TODOs):
  [ ] Token expiry tracking (EbayRepo, TCGRepo)
  [ ] Redis caching for API responses
  [ ] Retry logic with exponential backoff
  [ ] Proper pytest unit tests for all services
  [ ] Dead-letter queue for failed RabbitMQ messages
  [ ] Webhook signature verification (api/webhook.py)
```
