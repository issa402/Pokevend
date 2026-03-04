// ============================================================
// PokémonTool — RabbitMQ Configuration
// Connects the Node.js server to the message queue.
// The notification service consumes messages published by
// Python (api-consumer) and Go (scraping-service).
// ============================================================

const amqp = require('amqplib');

let connection;
let channel;

// Queue names — must match what the Python & Go services publish to
const QUEUES = {
    LISTINGS:         'listings',          // New card listings from all sources
    SCRAPED_LISTINGS: 'scraped_listings',  // Listings from Go scraper (FB/Mercari)
    PRICE_ALERTS:     'price_alerts',      // Price change events
    ANALYTICS:        'analytics_results', // Trend data from the analytics engine
};

/**
 * Establishes a RabbitMQ connection and creates a shared channel.
 * Asserts all queues to ensure they exist before consuming.
 */
async function connectRabbitMQ() {
    const url = process.env.RABBITMQ_URL || 'amqp://guest:guest@localhost:5672';
    connection = await amqp.connect(url);
    channel    = await connection.createChannel();

    // Assert all queues (creates them if they don't exist, no-op if they do)
    for (const queue of Object.values(QUEUES)) {
        await channel.assertQueue(queue, { durable: true }); // durable = survives restart
    }

    console.log('  ✓ RabbitMQ connected, queues asserted');

    // Reconnect automatically if AMQP connection drops
    connection.on('error', (err) => {
        console.error('RabbitMQ connection error:', err.message);
        setTimeout(connectRabbitMQ, 5000);
    });

    return { connection, channel };
}

/**
 * Returns the active AMQP channel for consuming/publishing.
 */
function getChannel() {
    if (!channel) throw new Error('RabbitMQ not connected yet. Call connectRabbitMQ() first.');
    return channel;
}

module.exports = { connectRabbitMQ, getChannel, QUEUES };
