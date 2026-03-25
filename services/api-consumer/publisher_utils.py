"""
============================================================
PokémonTool — RabbitMQ Publisher
============================================================
Publishes CardListing objects to the RabbitMQ message queue
so the Node.js notification service can consume and process them.
Uses persistent message delivery so listings survive broker restarts.
============================================================
"""

import os
import json
import logging
import time

import pika

from models import CardListing

log = logging.getLogger(__name__)

QUEUE_LISTINGS  = "listings"          # Consumed by Node.js notification service
QUEUE_ANALYTICS = "analytics_results" # For analytics engine output


class RabbitMQPublisher:
    """
    Publishes messages to RabbitMQ with automatic reconnect logic.
    """

    def __init__(self):
        self.url     = os.getenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672")
        self._conn   = None
        self._channel= None
        self._connect()

    def _connect(self):
        """Establish (or re-establish) connection to RabbitMQ."""
        retries = 0
        while retries < 10:
            try:
                params        = pika.URLParameters(self.url)
                self._conn    = pika.BlockingConnection(params)
                self._channel = self._conn.channel()

                # Ensure queues exist (idempotent — safe to call many times)
                for queue in [QUEUE_LISTINGS, QUEUE_ANALYTICS]:
                    self._channel.queue_declare(queue=queue, durable=True)

                log.info("✓ RabbitMQ publisher connected")
                return
            except Exception as e:
                retries += 1
                wait = min(2 ** retries, 30)  # Exponential backoff, cap at 30s
                log.warning(f"RabbitMQ connect failed ({retries}/10): {e}. Retrying in {wait}s...")
                time.sleep(wait)

        raise RuntimeError("Could not connect to RabbitMQ after 10 attempts")

    def _ensure_connected(self):
        """Reconnect if the connection was dropped."""
        if not self._conn or self._conn.is_closed:
            log.info("RabbitMQ connection lost — reconnecting...")
            self._connect()

    def publish_listing(self, listing: CardListing):
        """
        Publishes a single CardListing to the 'listings' queue.
        Delivery mode 2 = persistent (message survives broker restart).
        """
        self._ensure_connected()
        try:
            body = json.dumps(listing.to_queue_dict()).encode()
            self._channel.basic_publish(
                exchange   = "",              # Default exchange
                routing_key= QUEUE_LISTINGS,
                body       = body,
                properties = pika.BasicProperties(delivery_mode=2),  # Persistent
            )
            log.debug(f"Published listing: {listing.card_name} @ ${listing.price} ({listing.marketplace})")
        except Exception as e:
            log.error(f"Failed to publish listing: {e}")
            self._connect()  # Try to reconnect and let next poll retry

    def publish_analytics(self, event: dict):
        """
        Publishes an analytics event (trend change, deal found) to the analytics queue.
        The Node.js notification service also consumes this for user alerts.
        """
        self._ensure_connected()
        try:
            body = json.dumps(event).encode()
            self._channel.basic_publish(
                exchange   = "",
                routing_key= QUEUE_ANALYTICS,
                body       = body,
                properties = pika.BasicProperties(delivery_mode=2),
            )
        except Exception as e:
            log.error(f"Failed to publish analytics event: {e}")
