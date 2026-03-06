# ============================================================
# FILE: services/api-consumer/repositories/tcg_repo.py
# TYPE: Repository Layer — Raw TCGplayer API Calls
#
# WHAT IS THIS?
# Makes raw HTTP calls to the TCGplayer API.
# Mirror of ebay_repo.py — same structure, different API.
#
# TCGplayer API Authentication:
#   Uses OAuth2 Client Credentials (same as eBay)
#   Base URL: https://api.tcgplayer.com
#   API version: v1.37.0 (check docs for latest)
#
# TCGPLAYER FLOW:
#   1. POST /token → get access_token (valid 60 days)
#   2. GET /catalog/products?productName=Charizard → get product IDs
#   3. GET /pricing/product/{productId} → get market price for that ID
#
# REPOSITORY LAYER RULE:
# ONLY raw API calls live here. No Pydantic models. No publishing.
# Returns raw dicts that the service layer interprets.
# ============================================================
import os
import httpx
from typing import Any, Dict, Optional


class TCGRepo:
    """
    Encapsulates all raw TCGplayer API HTTP calls.
    
    Note the TWO-STEP lookup:
      1. Search by name → get productId
      2. Fetch price by productId → get marketPrice
    This is because TCGplayer requires a product ID for pricing.
    EbayRepo does it in one step (eBay's Browse API accepts text search directly).
    """

    BASE_URL = "https://api.tcgplayer.com"

    def __init__(self):
        self.public_key  = os.getenv("TCGPLAYER_PUBLIC_KEY", "")
        self.private_key = os.getenv("TCGPLAYER_PRIVATE_KEY", "")
        # Both keys are required — TCGplayer's OAuth2 uses a two-key system
        self._token: Optional[str] = None

    async def get_token(self) -> str:
        """
        OAuth2 client credentials flow for TCGplayer.
        Same pattern as ebay_repo.get_token() — different endpoint and params.
        
        TOKEN CACHING: simple in-memory cache (same trade-off as eBay).
        TCGplayer tokens are valid for 60 days — much longer than eBay's 2 hours.
        In production: store token in Redis with proper expiry tracking.
        """
        if self._token:
            return self._token

        async with httpx.AsyncClient() as client:
            resp = await client.post(
                f"{self.BASE_URL}/token",
                # TCGplayer uses form-encoded body (not JSON) for token requests
                data={
                    "grant_type":    "client_credentials",
                    "client_id":     self.public_key,
                    "client_secret": self.private_key,
                },
            )
            resp.raise_for_status()
            self._token = resp.json()["access_token"]

        return self._token

    async def get_market_price(self, card_name: str) -> Dict[str, Any]:
        """
        Get the current market price for a card by name.
        Two-step process: search by name → get productId → fetch price.
        Returns raw TCGplayer pricing response dict.
        """
        token = await self.get_token()
        headers = {"Authorization": f"Bearer {token}"}

        async with httpx.AsyncClient() as client:
            # Step 1: Search for the product by name to get its ID
            search_resp = await client.get(
                f"{self.BASE_URL}/v1.37.0/catalog/products",
                headers=headers,
                params={
                    "productName": card_name,
                    "categoryId": 3,   # categoryId 3 = Pokémon on TCGplayer
                    "limit": 1,
                },
            )
            search_resp.raise_for_status()
            products = search_resp.json().get("results", [])
            if not products:
                return {"results": []}  # no product found — return empty

            # Step 2: Fetch pricing for the found product
            product_id = products[0]["productId"]
            price_resp = await client.get(
                f"{self.BASE_URL}/v1.37.0/pricing/product/{product_id}",
                headers=headers,
            )
            price_resp.raise_for_status()
            return price_resp.json()

# ============================================================
# TODO #1 (Practice): Add batch pricing
# TCGplayer supports fetching up to 100 product prices at once:
#   GET /v1.37.0/pricing/product/{id1},{id2},{id3}...
# This is much more efficient than N separate calls for N cards.
# Modify get_market_price to accept a list of card names,
# batch search for all of them, collect product IDs, then fetch
# pricing for all in one call. Return a dict of cardName → price.

# TODO #2 (Practice): Handle TCGplayer rate limits gracefully
# TCGplayer allows 300 requests/minute. During WATCH_LIST scanning,
# you may hit this. TCGplayer returns 429 with a Retry-After header.
# Read: retry_after = int(resp.headers.get("Retry-After", 1))
# sleep: await asyncio.sleep(retry_after) then retry the request.
# This is called "respecting rate limits" — a required practice for any API integration.
# ============================================================
