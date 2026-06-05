# ============================================================
# FILE: services/api-consumer/repositories/ebay_repo.py
# TYPE: Repository Layer — Raw eBay API Calls
#
# WHAT IS THIS?
# The eBay repository layer. Contains ONLY raw HTTP calls to the eBay API.
# No business logic. No data transformation beyond what's needed for the call.
#
# REPOSITORY LAYER RULES:
#   ✅ Make HTTP requests to external APIs
#   ✅ Handle authentication (get/refresh tokens)
#   ✅ Return raw API responses (json dicts)
#   ❌ NO business logic (filtering, normalizing) — that's EbayService
#   ❌ NO Pydantic models — return raw dicts
#   ❌ NO RabbitMQ publishing — that's EbayService
#
# WHY SEPARATE THE API CALL FROM THE LOGIC?
# If eBay changes their API response format, you only change EbayRepo.
# The service (EbayService) stays the same because it calls the same interface.
# This is the same reason we separate Go's store from services.
#
# FAANG PATTERN: External API clients behind a repository interface
# At Stripe, every third-party API (Twilio, SendGrid, etc.) has its own
# client class. The service layer never calls httpx directly.
#
# PYTHON CONCEPTS:
#   httpx.AsyncClient (async HTTP), Optional type hints,
#   _token (private attribute by convention), basic auth
# ============================================================

import os
from pathlib import Path

import httpx
from dotenv import load_dotenv
from typing import Any, Dict, Optional


def _env_file_candidates(repo_file: Path) -> list[Path]:
    parents = repo_file.resolve().parents
    candidates = [parents[1] / ".env"]
    if len(parents) > 3:
        candidates.append(parents[3] / ".env")
    return candidates


def _load_env_files() -> None:
    for env_file in _env_file_candidates(Path(__file__)):
        load_dotenv(env_file, override=False)


_load_env_files()


class EbayRepo:
    """
    #Encapsulates all raw eBay API HTTP calls.
    
    #PYTHON CONVENTION: Leading underscore (_token) = private (not enforced, just convention)
    #In Go, lowercase = private (enforced by compiler)
    #In Python, underscore prefix = "please don't use this from outside the class"
    
    #Manages OAuth token caching: fetches once, reuses until it expires.
    #(Production improvement: check token expiry and refresh proactively)
    """

    # Class-level constants: UPPER_SNAKE_CASE = constants in Python
    # Accessible as EbayRepo.SANDBOX_URL or self.SANDBOX_URL
    SANDBOX_URL = "https://api.sandbox.ebay.com"
    PROD_URL    = "https://api.ebay.com"

    
    def __init__(self):
        # Read credentials from environment — never hardcode API keys in code!
        self.client_id     = os.getenv("EBAY_CLIENT_ID", "")
        self.client_secret = os.getenv("EBAY_CLIENT_SECRET", "")
        # Sandbox mode for testing (uses eBay's test environment, not real marketplace)
        self.sandbox       = os.getenv("EBAY_SANDBOX_MODE", "true").lower() == "true"
        self.base_url      = self.SANDBOX_URL if self.sandbox else self.PROD_URL
        # _token with underscore = private attribute convention
        # Optional[str] = can be None initially, becomes str after first get_token() call
        self._token: Optional[str] = None

    async def get_token(self) -> str:
        """
        #Fetch an OAuth2 access token from eBay using client credentials flow.
        
        #OAUTH2 CLIENT CREDENTIALS FLOW:
        #Used for server-to-server authentication (no user involved).
        #Send client_id + client_secret → get access_token (valid for ~2 hours).
        
        #Token caching: we reuse self._token if already fetched.
        #In production: check token expiry (eBay returns "expires_in" seconds).
        """
        if self._token:  # simple cache — reuse existing token
            return self._token

        # httpx.AsyncClient = async HTTP client (like requests but async)
        # "async with" = context manager that automatically closes the client
        # This pattern ensures connections are properly cleaned up
        async with httpx.AsyncClient() as client:
            resp = await client.post(
                f"{self.base_url}/identity/v1/oauth2/token",
                # auth=(id, secret) = HTTP Basic Auth header (built-in httpx feature)
                # Sends "Authorization: Basic base64(client_id:client_secret)"
                auth=(self.client_id, self.client_secret),
                # data= sends as application/x-www-form-urlencoded (form data, not JSON)
                # OAuth2 token endpoints require form data, not JSON body
                data={
                    "grant_type": "client_credentials",
                    "scope": "https://api.ebay.com/oauth/api_scope"
                },
            )
            resp.raise_for_status()  # raise exception if HTTP status >= 400
            self._token = resp.json()["access_token"]

        return self._token

    async def search_listings(self, card_name: str, limit: int = 200, max_pages: int = 1) -> Dict[str, Any]:
        """
        #Search eBay's Browse API for Pokémon card listings by name.
        #Returns raw eBay API response (dict with "itemSummaries" array).
        
        #categoryIds:{183454} = Pokémon category on eBay
        #(filters out non-card items with similar names)
        
        #PYTHON TYPING: Dict[str, Any]
        #Dict = dictionary, str = string keys, Any = any value type
        #eBay's response has nested structure we can't fully type without Pydantic
        """
        token = await self.get_token()

        page_limit = min(max(limit, 1), 200)
        page_count = min(max(max_pages, 1), 5)
        combined: Dict[str, Any] = {"itemSummaries": [], "total": 0}

        async with httpx.AsyncClient() as client:
            for page in range(page_count):
                resp = await client.get(
                    f"{self.base_url}/buy/browse/v1/item_summary/search",
                    # Authorization: Bearer <token> = standard OAuth2 bearer token header
                    headers={"Authorization": f"Bearer {token}"},
                    # params= = URL query parameters (?q=...&limit=...&filter=...)
                    params={
                        "q": _marketplace_query(card_name),
                        "limit": page_limit,
                        "offset": page * page_limit,
                        "filter": "categoryIds:{183454},buyingOptions:{FIXED_PRICE}",
                        "sort": "price",
                        "auto_correct": "KEYWORD",
                    },
                )
                resp.raise_for_status()
                payload = resp.json()
                combined["total"] = payload.get("total", combined["total"])
                combined["itemSummaries"].extend(payload.get("itemSummaries", []))
                if not payload.get("next"):
                    break
            return combined  # returns the full eBay response as a Python dict

# ============================================================
# TODO #1 (Practice): Add token expiry handling
# eBay tokens are valid for approximately 7200 seconds (2 hours).
# After get_token() sets self._token, also store the expiry time:
#   import time
#   self._token_expires_at = time.time() + resp.json()["expires_in"] - 60
#   (subtract 60 seconds as a buffer)
# In get_token(), before returning cached token, check:
#   if time.time() >= self._token_expires_at: self._token = None
# This prevents using an expired token.

# TODO #2 (Practice): Add retry logic for failed API calls
# eBay's API occasionally returns 429 (rate limited) or 503 (unavailable).
# The httpx library supports retry with: httpx.HTTPTransport(retries=3)
# Or manually: wrap the request in a for loop (max 3 attempts)
# with exponential backoff: await asyncio.sleep(2 ** attempt)
# This makes the service resilient to temporary eBay API issues.
# Research: "exponential backoff with jitter" — FAANG standard for retries
# ============================================================


def _marketplace_query(query: str) -> str:
    value = " ".join((query or "").split())
    lowered = value.lower()
    if any(term in lowered for term in ("psa", "cgc", "bgs", "graded", "/", "#")):
        return value
    if "pokemon" in lowered or "pokémon" in lowered:
        return value
    return f"{value} pokemon card"
