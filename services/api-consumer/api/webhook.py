# ============================================================
# FILE: services/api-consumer/api/webhook.py
# TYPE: API Layer — HTTP Endpoint (FastAPI Router)
#
# WHAT IS THIS?
# A FastAPI router that handles the ONE HTTP endpoint this service exposes:
# POST /webhook — receives real-time sale notifications from eBay.
#
# WHAT IS AN eBay WEBHOOK?
# eBay can push notifications to YOUR server when events happen:
#   - A card you're watching gets a new sale
#   - A listing you're tracking closes
#   - Price changes on items in your eBay watchlist
# You register your URL in eBay's developer portal.
# When an event fires, eBay makes an HTTP POST to /webhook with JSON data.
# This is the "push" alternative to us constantly polling eBay's API.
#
# WHY FAANG COMPANIES PREFER WEBHOOKS:
#   Polling: your server tock API every 60s → 1440 calls/day → expensive
#   Webhook: eBay calls YOU when events happen → fewer calls, real-time data
#
# FASTAPI ROUTER PATTERN:
# Instead of defining routes directly on the app,
# we use APIRouter and include it in main.py with app.include_router().
# Benefit: groups related routes in one file, easier organization.
#
# FASTAPI CONCEPTS:
#   APIRouter, @router.post, Request, dependency injection (future)
# ============================================================
import logging
from typing import Any, Dict

from fastapi import APIRouter, HTTPException, Request

logger = logging.getLogger(__name__)

# APIRouter groups related endpoints
# Applied as: app.include_router(router) in main.py
# Tags appear in /docs UI to group endpoints visually
router = APIRouter(tags=["eBay Webhook"])


@router.post("/webhook")
async def ebay_webhook(request: Request) -> Dict[str, str]:
    """
    Receive eBay real-time notifications via webhook.
    
    REQUEST FLOW:
    eBay server → POST /webhook → this handler → extract data → process
    
    FASTAPI REQUEST OBJECT:
    request.json() = parse the body as JSON (returns a dict)
    request.headers = dict of HTTP headers
    request.body() = raw bytes (use if you need to verify signature)
    
    ASYNC: await request.json() suspends while reading the body
    (network I/O). Fast for request objects — body is already buffered.
    
    PRODUCTION SECURITY (TODO): eBay signs webhooks with a secret key.
    You should verify the signature before processing any webhook.
    Unverified webhooks can be spoofed by anyone who knows your URL.
    """
    try:
        # Parse the raw request body as JSON
        # eBay sends different event payloads depending on notification type
        payload: Dict[str, Any] = await request.json()
    except Exception:
        # 400 Bad Request if body isn't valid JSON
        raise HTTPException(status_code=400, detail="Invalid JSON body")

    # Extract event type from eBay's standard payload structure
    # eBay's notification format: {"notificationID": "...", "events": [...]}
    event_type = payload.get("notificationType", "UNKNOWN")
    logger.info(f"eBay webhook received: {event_type}")
    logger.debug(f"Webhook payload: {payload}")  # debug only — may contain sensitive data

    # FUTURE: process specific event types
    # For now, just acknowledge receipt (HTTP 200 with JSON body)
    # eBay retries webhooks if you return non-2xx — so always return 200 if received

    # TODO #1 (Practice): Verify eBay webhook signature
    # eBay sends an X-EBAY-SIGNATURE header with a base64-encoded HMAC-SHA256 signature.
    # Steps to verify:
    #   1. Get: signature = request.headers.get("X-EBAY-SIGNATURE")
    #   2. Get: body = await request.body()  (raw bytes)
    #   3. Compute: expected = hmac.new(secret.encode(), body, hashlib.sha256).b64encode()
    #   4. If signature != expected: raise HTTPException(401, "Invalid webhook signature")
    # Research: Python hmac module, base64 encoding, constant-time comparison (hmac.compare_digest)

    # TODO #2 (Practice): Route webhook events to appropriate service
    # eBay sends different notification types. Add an event router:
    #   if event_type == "MARKETPLACE_ACCOUNT_DELETION": handle account deletion (GDPR)
    #   if event_type == "ITEM_SOLD": extract card name + price, publish to RabbitMQ
    # For ITEM_SOLD, use the global publisher from main.py:
    #   from main import publisher  (or better: use FastAPI dependency injection)
    # Return meaningful success messages per event type.

    return {"status": "accepted", "event": event_type}
