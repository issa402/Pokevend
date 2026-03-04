"""
============================================================
PokémonTool — eBay API Client
============================================================
Handles:
  - OAuth2 client credentials token management
  - eBay Browse API card searches
  - eBay Notification API subscription management
  - Webhook challenge verification
  - Parsing eBay responses into our standard CardListing model
============================================================
"""

import os
import hashlib
import logging
import time
from typing import Optional, List

import requests

from models import CardListing

log = logging.getLogger(__name__)

# eBay API endpoints — sandbox vs production
EBAY_SANDBOX_AUTH_URL  = "https://api.sandbox.ebay.com/identity/v1/oauth2/token"
EBAY_PROD_AUTH_URL     = "https://api.ebay.com/identity/v1/oauth2/token"
EBAY_SANDBOX_BROWSE    = "https://api.sandbox.ebay.com/buy/browse/v1"
EBAY_PROD_BROWSE       = "https://api.ebay.com/buy/browse/v1"
EBAY_PROD_NOTIFICATION = "https://api.ebay.com/commerce/notification/v1"

# Pokemon Trading Card Game category ID on eBay
POKEMON_CATEGORY_ID = "183454"


class EbayClient:
    """
    Client for interacting with the eBay REST APIs.
    Automatically refreshes OAuth2 tokens before they expire.
    """

    def __init__(self):
        self.client_id     = os.getenv("EBAY_CLIENT_ID")
        self.client_secret = os.getenv("EBAY_CLIENT_SECRET")
        self.sandbox_mode  = os.getenv("EBAY_SANDBOX_MODE", "true").lower() == "true"

        # Select correct base URLs based on environment
        self.auth_url  = EBAY_SANDBOX_AUTH_URL if self.sandbox_mode else EBAY_PROD_AUTH_URL
        self.browse_url= EBAY_SANDBOX_BROWSE   if self.sandbox_mode else EBAY_PROD_BROWSE
        self.notif_url = EBAY_PROD_NOTIFICATION  # Notifications are production-only

        self._access_token  = None
        self._token_expires = 0   # Unix timestamp when the token expires

        if not self.client_id or not self.client_secret:
            log.warning("eBay credentials not configured — eBay polling disabled.")

    def _get_token(self) -> Optional[str]:
        """
        Fetches or refreshes the eBay OAuth2 client credentials access token.
        Tokens are cached in memory until they expire.
        """
        if not self.client_id or not self.client_secret:
            return None

        # Return cached token if still valid (with 60s buffer)
        if self._access_token and time.time() < (self._token_expires - 60):
            return self._access_token

        log.info("Refreshing eBay OAuth2 token...")
        resp = requests.post(
            self.auth_url,
            data={
                "grant_type": "client_credentials",
                "scope":      "https://api.ebay.com/oauth/api_scope",
            },
            auth=(self.client_id, self.client_secret),
            timeout=10,
        )
        resp.raise_for_status()
        data = resp.json()

        self._access_token  = data["access_token"]
        self._token_expires = time.time() + data["expires_in"]
        log.info(f"eBay token refreshed. Expires in {data['expires_in']}s")
        return self._access_token

    def _headers(self) -> dict:
        """Returns authenticated headers for eBay API requests."""
        return {
            "Authorization":               f"Bearer {self._get_token()}",
            "X-EBAY-C-MARKETPLACE-ID":     "EBAY_US",
            "X-EBAY-C-ENDUSERCTX":        "affiliateCampaignId=<ePNCampaignId>",
            "Content-Type":               "application/json",
        }

    def search_pokemon_cards(self, query: str, limit: int = 50) -> List[CardListing]:
        """
        Searches eBay for Pokémon card listings using the Browse API.
        Returns a list of CardListing objects normalized to our data model.

        :param query: Search term e.g. "charizard psa 10"
        :param limit: Max results (eBay max per call is 200)
        """
        token = self._get_token()
        if not token:
            return []

        try:
            resp = requests.get(
                f"{self.browse_url}/item_summary/search",
                headers=self._headers(),
                params={
                    "q":           query,
                    "category_ids": POKEMON_CATEGORY_ID,  # Narrow to Pokemon TCG category
                    "limit":       min(limit, 200),
                    "filter":      "buyingOptions:{FIXED_PRICE}",  # Skip auctions
                    "sort":        "newlyListed",            # Most recent first
                    "fieldgroups": "ASPECT_REFINEMENTS",
                },
                timeout=10,
            )
            resp.raise_for_status()
        except requests.RequestException as e:
            log.error(f"eBay Browse API error: {e}")
            return []

        items      = resp.json().get("itemSummaries", [])
        listings   = []
        for item in items:
            try:
                price_val = float(item.get("price", {}).get("value", 0))
                listing   = CardListing(
                    card_name   = item.get("title", "Unknown Card"),
                    price       = price_val,
                    marketplace = "ebay",
                    listing_url = item.get("itemWebUrl", ""),
                    image_url   = item.get("image", {}).get("imageUrl", ""),
                    seller      = item.get("seller", {}).get("username", "unknown"),
                    condition   = item.get("condition", "Unknown"),
                    listing_id  = item.get("itemId", ""),
                )
                listings.append(listing)
            except Exception as parse_err:
                log.warning(f"Failed to parse eBay item: {parse_err}")

        log.info(f"eBay search '{query}' → {len(listings)} listings")
        return listings

    def setup_notification_subscription(self):
        """
        Sets up an eBay Notification API subscription so eBay pushes
        real-time events (ItemListed, ItemRevised, etc.) to our webhook.
        Requires production credentials and a publicly accessible webhook URL.
        """
        token = self._get_token()
        if not token or self.sandbox_mode:
            log.info("Skipping notification subscription (sandbox mode or no token)")
            return

        webhook_url = os.getenv("EBAY_WEBHOOK_URL")  # Set this to your public endpoint
        if not webhook_url:
            log.warning("EBAY_WEBHOOK_URL not set — skipping notification subscription")
            return

        # Subscribe to price change and new listing events for Pokemon category
        payload = {
            "notificationEndpoint": {
                "endpointType": "HTTPS",
                "webhookUrl":   webhook_url,
            },
            "topics": [
                {"topicId": "MARKETPLACE_ACCOUNT_DELETION"},  # Required subscription
            ]
        }
        resp = requests.post(
            f"{self.notif_url}/subscription",
            headers=self._headers(),
            json=payload,
            timeout=10,
        )
        if resp.ok:
            log.info(f"eBay notification subscription created: {resp.json()}")
        else:
            log.warning(f"eBay subscription failed: {resp.status_code} {resp.text}")

    def compute_challenge_response(self, challenge_code: str, verification_token: str, endpoint_url: str) -> str:
        """
        eBay sends a GET request with a challenge_code when you register a webhook.
        You must respond with a specific hash so eBay can verify you own the endpoint.
        Formula: SHA-256(challengeCode + verificationToken + endpointUrl)
        """
        hash_input = challenge_code + verification_token + endpoint_url
        return hashlib.sha256(hash_input.encode()).hexdigest()

    def parse_notification(self, event_data: dict) -> Optional[CardListing]:
        """
        Converts a raw eBay Notification API webhook payload into our CardListing model.
        Returns None if the event is not a listing/price event.
        """
        try:
            topic   = event_data.get("metadata", {}).get("topic", "")
            payload = event_data.get("notification", {}).get("data", {})

            # Only process listing-related events
            if "ITEM" not in topic and "LISTING" not in topic:
                return None

            return CardListing(
                card_name   = payload.get("title", "Unknown"),
                price       = float(payload.get("currentPrice", {}).get("value", 0)),
                marketplace = "ebay",
                listing_url = payload.get("itemWebUrl", ""),
                listing_id  = payload.get("itemId", ""),
                condition   = payload.get("conditionDescription", "Unknown"),
            )
        except Exception as e:
            log.warning(f"Could not parse eBay notification: {e}")
            return None
