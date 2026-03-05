# ============================================================
# api-consumer/repositories/tcg_repo.py
# Raw TCGplayer API calls ONLY — no business logic here.
# ============================================================
import os
import httpx
from typing import Any, Dict, Optional


class TCGRepo:
    """Handles raw HTTP communication with the TCGplayer API."""

    BASE_URL = "https://api.tcgplayer.com"

    def __init__(self):
        self.public_key  = os.getenv("TCGPLAYER_PUBLIC_KEY", "")
        self.private_key = os.getenv("TCGPLAYER_PRIVATE_KEY", "")
        self._token: Optional[str] = None

    async def get_token(self) -> str:
        if self._token:
            return self._token
        async with httpx.AsyncClient() as client:
            resp = await client.post(
                f"{self.BASE_URL}/token",
                data={
                    "grant_type":    "client_credentials",
                    "client_id":     self.public_key,
                    "client_secret": self.private_key,
                },
            )
            resp.raise_for_status()
            self._token = resp.json()["access_token"]
        return self._token

    async def search_products(self, card_name: str) -> Dict[str, Any]:
        """Search TCGplayer catalog for a card."""
        token = await self.get_token()
        async with httpx.AsyncClient() as client:
            resp = await client.get(
                f"{self.BASE_URL}/v1.39.0/catalog/products",
                headers={"Authorization": f"Bearer {token}"},
                params={"productName": card_name, "categoryId": 3, "limit": 20},  # 3 = Pokémon
            )
            resp.raise_for_status()
            return resp.json()

    async def get_prices(self, product_id: str) -> Dict[str, Any]:
        """Get market and low prices for a product ID."""
        token = await self.get_token()
        async with httpx.AsyncClient() as client:
            resp = await client.get(
                f"{self.BASE_URL}/v1.39.0/pricing/product/{product_id}",
                headers={"Authorization": f"Bearer {token}"},
            )
            resp.raise_for_status()
            return resp.json()
