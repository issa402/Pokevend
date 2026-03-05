# ============================================================
# api-consumer/repositories/ebay_repo.py
# Raw eBay API calls ONLY — no business logic here.
# Returns raw data; service layer normalizes it.
# ============================================================
import os
import httpx
from typing import Any, Dict, Optional


class EbayRepo:
    """Handles raw HTTP communication with the eBay Browse API."""

    SANDBOX_URL  = "https://api.sandbox.ebay.com"
    PROD_URL     = "https://api.ebay.com"

    def __init__(self):
        self.client_id     = os.getenv("EBAY_CLIENT_ID", "")
        self.client_secret = os.getenv("EBAY_CLIENT_SECRET", "")
        self.sandbox       = os.getenv("EBAY_SANDBOX_MODE", "true").lower() == "true"
        self.base_url      = self.SANDBOX_URL if self.sandbox else self.PROD_URL
        self._token: Optional[str] = None

    async def get_token(self) -> str:
        """Fetch OAuth2 client-credentials token from eBay."""
        if self._token:
            return self._token
        async with httpx.AsyncClient() as client:
            resp = await client.post(
                f"{self.base_url}/identity/v1/oauth2/token",
                auth=(self.client_id, self.client_secret),
                data={"grant_type": "client_credentials", "scope": "https://api.ebay.com/oauth/api_scope"},
            )
            resp.raise_for_status()
            self._token = resp.json()["access_token"]
        return self._token

    async def search_listings(self, card_name: str, limit: int = 50) -> Dict[str, Any]:
        """Search eBay for Pokémon card listings by name."""
        token = await self.get_token()
        async with httpx.AsyncClient() as client:
            resp = await client.get(
                f"{self.base_url}/buy/browse/v1/item_summary/search",
                headers={"Authorization": f"Bearer {token}"},
                params={"q": f"Pokemon card {card_name}", "limit": limit,
                        "filter": "categoryIds:{183454}"},  # Pokémon category
            )
            resp.raise_for_status()
            return resp.json()
