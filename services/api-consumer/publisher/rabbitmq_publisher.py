# ============================================================
# FILE: services/api-consumer/publisher/rabbitmq_publisher.py
# TYPE: Infrastructure — RabbitMQ Message Publisher
#
# WHAT IS THIS?
# Encapsulates all RabbitMQ publishing logic in one place.
# No other file should directly use aio_pika — they go through this class.
#
# FAANG MESSAGING PATTERN: Message Broker
# Instead of Python calling Go's API directly (tight coupling),
# Python publishes messages to RabbitMQ, and Go consumes them.
#
#   TIGHT COUPLING (bad):                 LOOSE COUPLING (our approach):
#   Python → HTTP POST → Go API          Python → RabbitMQ → Go consumer
#   If Go is down, Python fails          If Go is down, messages queue up
#   Cannot scale independently           Scale each service independently
#
# RABBITMQ CONCEPTS:
#   Connection: TCP connection to RabbitMQ broker
#   Channel: lightweight logical connection (many per connection)
#   Exchange: receives messages and routes to queues
#   Queue: stores messages until consumers read them
#   Routing key: which queue the exchange sends the message to
#
# aio_pika: async Python RabbitMQ library (for FastAPI's async world)
# amqp091-go: Go RabbitMQ library (same AMQP protocol, different language)
#
# PYTHON CONCEPTS:
#   async def, await, Optional[...], try/except, logging
# ============================================================
import json
import logging
from typing import Any, Dict, Optional

import aio_pika

logger = logging.getLogger(__name__)


class RabbitMQPublisher:
    """
    Async RabbitMQ publisher for the api-consumer service.
    
    FAANG PATTERN: Infrastructure wrapped in a class
    RabbitMQ details (connection, channel, exchange) are hidden inside.
    Service code just calls: await publisher.publish("listings", data)
    If we switched from RabbitMQ to AWS SQS, only THIS class changes.
    The services calling it don't change at all.
    """

    def __init__(self, url: str):
        self.url = url
        # Optional[aio_pika.Connection] = starts as None, set by connect()
        self._connection: Optional[aio_pika.Connection] = None
        self._channel: Optional[aio_pika.Channel] = None

    async def connect(self) -> None:
        """
        Establish connection to RabbitMQ.
        Called once during FastAPI startup (in main.py lifespan).
        
        connect_robust() vs connect():
        - connect(): fails immediately if RabbitMQ is unreachable
        - connect_robust(): retries with exponential backoff, reconnects on drops
        Use connect_robust() in production — temporary network blips shouldn't crash the service.
        """
        self._connection = await aio_pika.connect_robust(self.url)
        # A channel is a logical multiplexed connection inside the physical TCP connection
        # Creating a channel does NOT create a new TCP connection
        self._channel = await self._connection.channel()
        logger.info(f"RabbitMQ publisher connected to {self.url}")

    async def publish(self, queue_name: str, data: Dict[str, Any]) -> None:
        """
        Publish a message to a RabbitMQ queue.
        
        data (dict) -> JSON bytes -> aio_pika.Message -> RabbitMQ queue
        
        QUEUE DECLARATION:
        declare_queue is idempotent — creates queue if not exists, returns existing if it does.
        durable=True: queue survives RabbitMQ restarts (stored on disk).
        Without durable: queue disappears on broker restart — messages lost!
        
        DELIVERY MODE PERSISTENT:
        Messages are also stored on disk (not just RAM).
        Without this: messages in the queue disappear if RabbitMQ crashes.
        With this: messages survive crashes — critical for alert data.
        
        MESSAGE FORMAT:
        json.dumps() converts dict to JSON string.
        .encode() converts string to bytes (RabbitMQ requires bytes).
        """
        if not self._channel:
            logger.warning("Publisher not connected — skipping publish")
            return

        try:
            # Declare the queue — creates if not exists, returns existing if it does
            queue = await self._channel.declare_queue(queue_name, durable=True)

            # Create the Message object with JSON payload
            message = aio_pika.Message(
                body=json.dumps(data).encode("utf-8"),  # dict → JSON → bytes
                delivery_mode=aio_pika.DeliveryMode.PERSISTENT,  # survive broker restart
            )

            # Publish to the DEFAULT exchange with routing_key = queue_name
            # Default exchange: routes directly to queue with matching name
            await self._channel.default_exchange.publish(
                message,
                routing_key=queue_name,  # which queue to send to
            )
        except Exception as e:
            logger.error(f"Failed to publish to '{queue_name}': {e}")
            # Don't re-raise — a publish failure shouldn't crash the scanner loop

    async def close(self) -> None:
        """
        Close the RabbitMQ connection cleanly.
        Called on FastAPI shutdown (after lifespan's yield).
        
        Graceful close is important: it flushes any buffered messages
        and tells RabbitMQ this consumer is done (prevents message loss).
        """
        if self._connection and not self._connection.is_closed:
            await self._connection.close()
            logger.info("RabbitMQ publisher closed")

# ============================================================
# TODO #1 (Practice): Add a batch publish method
# Publishing one message at a time is fine for low volume.
# For high-volume scenarios (scraping 1000 listings at once),
# publishing one by one causes 1000 network round-trips.
# Add: async def publish_batch(self, queue_name, messages: List[Dict]) -> None
# Use a RabbitMQ transaction or publisher confirms to batch-send efficiently.
# Research: aio_pika Transaction, AMQP publisher_confirms

# TODO #2 (Practice): Add dead-letter queue handling
# When Go's consumer rejects a message (Nack without requeue),
# RabbitMQ can route it to a "dead-letter queue" for inspection.
# Modify declare_queue() to add arguments:
#   {"x-dead-letter-exchange": "", "x-dead-letter-routing-key": "listings.dead"}
# Then declare a "listings.dead" queue for monitoring.
# At FAANG, teams monitor dead-letter queues for data quality issues.
# ============================================================
