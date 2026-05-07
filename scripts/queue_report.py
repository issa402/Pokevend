import asyncio 
import os 
import sys
import logging 
from typing import List, Optional, Dict, Any
import aio_pika #its an aystnchronous library so it doesnt block code while waiting for server to respond
import httpx # used to talk to rabbitmq web based ,amagment API

logging.basicConfig(
    level = logging.INFO, 
    format = "%(asctime)s [%(levelname)s] %(message)s",
    datefmt= "%H:%M:%S"
)

logger = logging.getLogger("queue_report")

RABBITMQ_AMQP_URL = os.getenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
RABBITMQ_MGMT_URL = os.getenv("RABBITMQ_MGMT_URL", "http://guest:guest@localhost:15672/api")
EXPECTED_QUEUE = ["listings"]
class QueueReporter:
    def __init__(self, amqp_url: str, mgmt_url: str):
        self.amqp_url = amqp_url 
        self.mgmt_url = mgmt_url

    async def check_connection(self) -> bool:
        try:
            connection = await aio_pika.connect_robust(self.amqp_url) #Establoshes a TCP conncetion to the rabbitmq server 
            await connection.close()
            logger.info("Heartbeat: RabbitMQ AMQP is reachable")
            return True
        except Exception as e:
            logger.error("Heartbeat: RabbitMq AMQP is not reachable. Debug Now")
            return False 

    async def check_queues_amqp(self, queue : List[str]):
        try: 
            connection = await aio_pika.connect_robust(self.amqp_url)
            async with connection:
                channel = await connection.channel()#channels are virtual connections inside the bo TCP connection. more effecinet to open and have multiple lanes

                for q_name in queue:
                    try:
                        queue = await channel.declare_queue(q_name, passive=True) #find the queue and passive means dont create a new queue 
                        msg = queue.declaration_result.message_count # is number of messages waiting to in queeu to be processed
                        workers = queue.declaration_result.consumer_count #number of worker scripts currently connected adn listening to that queue
                        logger.info(f"Queue '{q_name}': {msg} pending msgs | {workers} workers active")
                    except aio_pika.exceptions.ChannelClosed as e:
                        logger.warning(f"Queue '{q_name}' does not exists in the broker yet")
                        channel = await connection.channel()
        except Exception as e:
            logger.error(f"Failed to inspect queues: {e}")


    async def check_mgmt_stats(self):
        async with httpx.AsyncClient() as client: # allows use of internet
            try:
                response = await client.get(f"{self.mgmt_url}/overview", auth=("guest", "guest")) # write url to the internet
                if response.status_code == 200:
                    data = response.json()
                    version = data.get("rabbitmq_version")
                    total_msgs = data.get("queue_totals", {}).get("messages", 0)

                    logger.info(f"🌐 Management API: Server v{version} is healthy")
                    logger.info(f"📈 Total Server Load: {total_msgs} total messages across all queues")
                else:
                    logger.warning(f"⚠️  Management API returned status {response.status_code}")
            except Exception as e:
                logger.error(f"❌ Could not reach Management API: {e}")

async def main():
    print("\n" + "="*40)
    print(" 🐇 RABBITMQ OPERATIONAL REPORT ")
    print("="*40)


    reporter = QueueReporter(RABBITMQ_AMQP_URL, RABBITMQ_MGMT_URL)

    if await reporter.check_connection():
        await reporter.check_queues_amqp(EXPECTED_QUEUE)
        await reporter.check_mgmt_stats()

    print("="*40 + "\n")

if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        sys.exit(0)



