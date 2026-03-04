"""
============================================================
PokémonTool — TCGPlayer API Client
============================================================
Handles interaction with the TCGPlayer API for:
  - Product/card search
  - Market price retrieval
  - Sale velocity data

NOTE: TCGPlayer requires a commercial API agreement.
If no API key is provided, this client returns empty results.
TCGPlayer API docs: https://docs.tcgplayer.com/
============================================================
"""

import os
import logging
import time
from typing import Optional, List

import requests

from models import CardListing

log = logging.getLogger(__name__)

TCGPLAYER_AUTH_URL = "https://api.tcgplayer.com/token"
TCGPLAYER_BASE_URL = "https://api.tcgplayer.com/v1.39.0"

# Pokemon TCG group ID in TCGPlayer catalog
POKEMON_CATEGORY_ID = 3


class TCGPlayerClient:
    """
    Client for the TCGPlayer API.
    Uses API key auth (public key + private key).
    Auto-refreshes the bearer token.
    """

    def __init__(self):
        self.public_key  = os.getenv("TCGPLAYER_PUBLIC_KEY")
        self.private_key = os.getenv("TCGPLAYER_PRIVATE_KEY")
        self._token      = None
        self._expires    = 0

        if not self.public_key or not self.private_key:
            log.warning("TCGPlayer API keys not configured — TCGPlayer integration disabled.")

    def _get_token(self) -> Optional[str]:
        """Fetches or refreshes TCGPlayer bearer token."""
        if not self.public_key or not self.private_key:
            return None

        # Reuse cached token if still valid
        if self._token and time.time() < (self._expires - 60):
            return self._token

        log.info("Refreshing TCGPlayer access token...")
        resp = requests.post(
            TCGPLAYER_AUTH_URL,
            data={
                "grant_type":    "client_credentials",
                "client_id":     self.public_key,
                "client_secret": self.private_key,
            },
            timeout=10,
        )
        resp.raise_for_status()
        data = resp.json()

        self._token   = data.get("access_token")
        self._expires = time.time() + data.get("expires_in", 3600)
        return self._token

    def _headers(self) -> dict:
        """Returns headers for authenticated TCGPlayer API requests."""
        token = self._get_token()
        if not token:
            return {}
        return {"Authorization": f"Bearer {token}"}

    def get_market_price(self, card_name: str) -> Optional[CardListing]:
        """
        Searches TCGPlayer for a card by name and returns current market price data.
        Returns a CardListing with tcgplayer marketplace tag.
        """
        token = self._get_token()
        if not token:
            return None

        try:
            # Step 1: Search for the product by name
            search_resp = requests.get(
                f"{TCGPLAYER_BASE_URL}/catalog/products",
                headers=self._headers(),
                params={
                    "categoryId": POKEMON_CATEGORY_ID,
                    "productName": card_name,
                    "limit": 5,
                },
                timeout=10,
            )
            search_resp.raise_for_status()
            products = search_resp.json().get("results", [])

            if not products:
                log.debug(f"TCGPlayer: no results for '{card_name}'")
                return None

            # Use the first result (most relevant match)
            product_id = products[0].get("productId")

            # Step 2: Fetch current market prices for this product
            price_resp = requests.get(
                f"{TCGPLAYER_BASE_URL}/pricing/product/{product_id}",
                headers=self._headers(),
                timeout=10,
            )
            price_resp.raise_for_status()
            prices = price_resp.json().get("results", [])

            # Find the "Near Mint" condition price (most relevant for vendors)
            nm_price = next(
                (p["marketPrice"] for p in prices
                 if p.get("subTypeName") == "Normal" and p.get("marketPrice")),
                None,
            )

            if not nm_price:
                return None

            return CardListing(
                card_name   = products[0].get("name", card_name),
                price       = float(nm_price),
                marketplace = "tcgplayer",
                listing_url = f"https://www.tcgplayer.com/product/{product_id}",
                image_url   = products[0].get("imageUrl", ""),
                listing_id  = str(product_id),
            )
        except requests.RequestException as e:
            log.error(f"TCGPlayer API error for '{card_name}': {e}")
            return None

    def search_products(self, query: str, limit: int = 20) -> List[CardListing]:
        """
        Full-text search across all Pokemon TCG products on TCGPlayer.
        Returns a list of CardListing with market prices attached.
        """
        token = self._get_token()
        if not token:
            return []

        try:
            resp = requests.get(
                f"{TCGPLAYER_BASE_URL}/catalog/products",
                headers=self._headers(),
                params={
                    "categoryId":  POKEMON_CATEGORY_ID,
                    "productName": query,
                    "limit":       limit,
                    "getExtendedFields": True,
                },
                timeout=10,
            )
            resp.raise_for_status()
            products = resp.json().get("results", [])

            return [
                CardListing(
                    card_name   = p.get("name", ""),
                    price       = float(p.get("lowestPrice") or 0),
                    marketplace = "tcgplayer",
                    listing_url = f"https://www.tcgplayer.com/product/{p.get('productId')}",
                    image_url   = p.get("imageUrl", ""),
                    listing_id  = str(p.get("productId", "")),
                )
                for p in products
                if p.get("name")
            ]
        except Exception as e:
            log.error(f"TCGPlayer search error: {e}")
            return []
