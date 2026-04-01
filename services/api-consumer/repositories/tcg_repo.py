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

import logging 
from typing import Optional, Dict, Any
from tcgdexsdk import TCGdex, Query 
import re
logger = logging.getLogger(__name__)



class TCGRepo:
    def __init__(self):
        self.client = TCGdex("en")

    async def get_market_price(self, card_name: str) -> Dict[str, Any]:
        try:
            # 1. UNIVERSAL CLEANER: Strip numbers and extra tags so TCGdex can find the base card
            # 'Charizard ex 199' -> 'Charizard'
            clean_name = re.sub(r'[^a-zA-Z\s]', '', card_name).split()[0]

            # 2. Search using the cleaned name
            search_results = await self.client.card.list(Query().equal("name", clean_name))
            
            if not search_results or len(search_results) == 0:
                logger.warning(f"TCGdex: No cards found for '{clean_name}'")
                return {"results": []}

            # 3. PICK THE FIRST ITEM [0] FROM THE LIST
            card_id = search_results[0].id
            
            # 4. Fetch the FULL card details
            full_card = await self.client.card.get(card_id)
            
            market_price = 0.0
        
            # 5. Extract price: pricing -> tcgplayer -> variant -> marketPrice
            if hasattr(full_card, 'pricing') and full_card.pricing:
                tp = full_card.pricing.tcgplayer
                if tp:
                    # We loop through possible variant names to find a price
                    for variant in ['normal', 'holofoil', 'reverse', 'firstEdition']:
                        v_data = getattr(tp, variant, None)
                        if v_data:
                            # marketPrice is the specific field from the TCGdex schema
                            market_price = getattr(v_data, 'marketPrice', 0.0)
                            if market_price > 0:
                                break

            if market_price > 0:
                return {
                    "results": [{
                        "marketPrice": float(market_price),
                        "productId": full_card.localId
                    }]
                }
            
            return {"results": []}
        except Exception as e:
            logger.error(f"TCGdex Repo Error for {card_name}: {e}")
            return {"results": []}

































# ============================================================
# ARCHIVED REPO: Legacy TCGplayer (Commented Out)
# ============================================================
# class LegacyTCGRepo:
#     BASE_URL = "https://api.tcgplayer.com"
#
#     def __init__(self):
#         self.public_key  = os.getenv("TCGPLAYER_PUBLIC_KEY", "")
#         self.private_key = os.getenv("TCGPLAYER_PRIVATE_KEY", "")
#         self._token: Optional[str] = None
#
#     async def get_token(self) -> str:
#         if self._token:
#             return self._token
#         async with httpx.AsyncClient() as client:
#             resp = await client.post(
#                 f"{self.BASE_URL}/token",
#                 data={
#                     "grant_type": "client_credentials",
#                     "client_id": self.public_key,
#                     "client_secret": self.private_key,
#                 },
#             )
#             resp.raise_for_status()
#             self._token = resp.json()["access_token"]
#         return self._token
#
#     async def get_market_price(self, card_name: str) -> Dict[str, Any]:
#         token = await self.get_token()
#         headers = {"Authorization": f"Bearer {token}"}
#         async with httpx.AsyncClient() as client:
#             search_resp = await client.get(
#                 f"{self.BASE_URL}/v1.37.0/catalog/products",
#                 headers=headers,
#                 params={"productName": card_name, "categoryId": 3, "limit": 1},
#             )
#             search_resp.raise_for_status()
#             products = search_resp.json().get("results", [])
#             if not products: return {"results": []}
#             product_id = products[0]["productId"]
#             price_resp = await client.get(
#                 f"{self.BASE_URL}/v1.37.0/pricing/product/{product_id}",
#                 headers=headers,
#             )
#             price_resp.raise_for_status()
#             return price_resp.json()

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
