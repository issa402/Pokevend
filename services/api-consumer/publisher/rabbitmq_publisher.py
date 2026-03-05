# ============================================================
# api-consumer/publisher/rabbitmq_publisher.py
# Publishes normalized listing data to RabbitMQ.
# Used by services — not repositories, not handlers.
# ============================================================
import json
import logging
import aio_pika
from typing import Any, Dict

logger = logging.getLogger(__name__)


class RabbitMQPublisher:
    """Async RabbitMQ publisher. Shared instance across the FastAPI app."""

    def __init__(self, url: str):
        self.url = url
        self._connection = None
        self._channel    = None

    async def connect(self):
        self._connection = await aio_pika.connect_robust(self.url)
        self._channel    = await self._connection.channel()
        logger.info("RabbitMQ publisher connected")

    async def publish(self, queue: str, message: Dict[str, Any]):
        """Publish a JSON message to a named queue."""
        if not self._channel:
            raise RuntimeError("Publisher not connected — call connect() first")
        await self._channel.default_exchange.publish(
            aio_pika.Message(
                body=json.dumps(message).encode(),
                delivery_mode=aio_pika.DeliveryMode.PERSISTENT,
            ),
            routing_key=queue,
        )

    async def close(self):
        if self._connection:
            await self._connection.close()
