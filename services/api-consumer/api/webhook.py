# ============================================================
# api-consumer/api/webhook.py — FastAPI route: eBay webhook
# POST /webhook — receives eBay item change notifications
# This is the ONLY HTTP route in api-consumer.
# ============================================================
import hmac
import hashlib
import logging
import os
from fastapi import APIRouter, Request, HTTPException, Depends

logger = logging.getLogger(__name__)
router = APIRouter()

VERIFICATION_TOKEN = os.getenv("EBAY_NOTIFICATION_VERIFICATION_TOKEN", "")


@router.post("/webhook")
async def ebay_webhook(request: Request):
    """
    Receive eBay Notification API payloads.
    eBay sends item sold/price change events here.
    We verify the signature, then the background scanner picks up changes.
    """
    # Verify eBay challenge token (required for webhook validation)
    body = await request.json()
    if "challenge_code" in body:
        challenge = body["challenge_code"]
        # eBay requires: SHA256(challenge + verification_token + endpoint_url)
        endpoint_url = str(request.url)
        hash_val = hashlib.sha256(
            f"{challenge}{VERIFICATION_TOKEN}{endpoint_url}".encode()
        ).hexdigest()
        return {"challengeResponse": hash_val}

    # Real notification — log and let the scanner handle it
    logger.info(f"eBay webhook received: {body.get('metadata', {}).get('topic', 'unknown')}")
    return {"status": "received"}
