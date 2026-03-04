"""
============================================================
PokémonTool — eBay API Consumer Entry Point
============================================================
This worker runs continuously and does two things:
  1. Polls eBay Browse API every N minutes for new Pokémon listings
  2. Runs a tiny Flask webhook server to receive eBay Notification API pushes

All discovered listings are published to RabbitMQ for the
Node.js notification service to process and dispatch as alerts.
============================================================
"""

import os
import sys
import logging
import schedule
import time
import threading

from dotenv import load_dotenv
from ebay_client import EbayClient
from tcgplayer_client import TCGPlayerClient
from publisher import RabbitMQPublisher

# Load environment variables from .env file
load_dotenv()

# --- Configure logging ---
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [API-CONSUMER] %(levelname)s — %(message)s",
    handlers=[logging.StreamHandler(sys.stdout)],
)
log = logging.getLogger(__name__)


def poll_ebay(ebay_client: EbayClient, publisher: RabbitMQPublisher):
    """
    Polls eBay Browse API for new Pokemon card listings.
    Publishes each listing to the RabbitMQ 'listings' queue.
    """
    log.info("Polling eBay Browse API for Pokémon card listings...")
    try:
        # Search terms that cover the most traded Pokémon card categories
        search_queries = [
            "pokemon card psa 10",
            "charizard pokemon card",
            "pokemon booster box",
            "pokemon graded card",
            "pokemon vintage card base set",
        ]

        for query in search_queries:
            listings = ebay_client.search_pokemon_cards(query, limit=50)
            for listing in listings:
                publisher.publish_listing(listing)

        log.info(f"eBay poll complete — queued listings from {len(search_queries)} searches")
    except Exception as e:
        log.error(f"eBay poll failed: {e}", exc_info=True)


def poll_tcgplayer(tcg_client: TCGPlayerClient, publisher: RabbitMQPublisher):
    """
    Polls TCGPlayer API for market price data on trending cards.
    """
    log.info("Fetching market prices from TCGPlayer...")
    try:
        # Fetch prices for a curated list of high-value cards
        trending_cards = [
            "Charizard Base Set",
            "Pikachu Illustrator",
            "Umbreon VMAX",
            "Rayquaza VMAX",
            "Mew VMAX",
        ]
        for card_name in trending_cards:
            price_data = tcg_client.get_market_price(card_name)
            if price_data:
                publisher.publish_listing(price_data)
        log.info("TCGPlayer price poll complete")
    except Exception as e:
        log.error(f"TCGPlayer poll failed: {e}", exc_info=True)


def start_webhook_server(ebay_client: EbayClient, publisher: RabbitMQPublisher):
    """
    Starts a Flask webhook endpoint to receive real-time push notifications
    from the eBay Notification API. Runs in a background thread.
    """
    from flask import Flask, request, jsonify

    app = Flask(__name__)

    @app.route("/ebay/webhook", methods=["GET", "POST"])
    def ebay_webhook():
        # GET: eBay sends a challenge token to verify ownership of the endpoint
        if request.method == "GET":
            challenge = request.args.get("challenge_code", "")
            response_hash = ebay_client.compute_challenge_response(
                challenge,
                os.getenv("EBAY_NOTIFICATION_VERIFICATION_TOKEN"),
                request.url,
            )
            return jsonify({"challengeResponse": response_hash})

        # POST: eBay sends the actual notification event
        event_data = request.json
        if event_data:
            log.info(f"eBay webhook received: {event_data.get('metadata', {}).get('topic')}")
            # Convert the raw eBay notification to our standard listing shape
            listing = ebay_client.parse_notification(event_data)
            if listing:
                publisher.publish_listing(listing)
        return jsonify({"status": "ok"}), 200

    # Run on port 8080 (mapped in docker-compose for eBay to reach)
    log.info("Starting eBay webhook server on port 8080")
    app.run(host="0.0.0.0", port=8080, debug=False)


def main():
    log.info("🎮 PokémonTool API Consumer starting...")

    # Initialize clients
    publisher   = RabbitMQPublisher()
    ebay_client = EbayClient()
    tcg_client  = TCGPlayerClient()

    # Subscribe to eBay Notification API for real-time pushes
    try:
        ebay_client.setup_notification_subscription()
    except Exception as e:
        log.warning(f"eBay notification subscription failed (will still poll): {e}")

    # Start the webhook endpoint in a daemon thread
    webhook_thread = threading.Thread(
        target=start_webhook_server,
        args=(ebay_client, publisher),
        daemon=True,  # Dies when the main thread dies
    )
    webhook_thread.start()

    # --- Scheduled polling: fill the gaps between push notifications ---
    poll_interval = int(os.getenv("SCRAPING_INTERVAL_MINUTES", 30))

    # Run immediately on start, then on schedule
    poll_ebay(ebay_client, publisher)
    poll_tcgplayer(tcg_client, publisher)

    # Schedule recurring polls
    schedule.every(poll_interval).minutes.do(poll_ebay,      ebay_client, publisher)
    schedule.every(poll_interval).minutes.do(poll_tcgplayer, tcg_client,  publisher)

    log.info(f"✅ Polling scheduled every {poll_interval} minutes. Running...")

    # Keep the process alive — schedule runs tasks on the main thread
    while True:
        schedule.run_pending()
        time.sleep(10)  # Check schedule every 10 seconds


if __name__ == "__main__":
    main()
