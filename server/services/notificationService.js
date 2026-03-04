// ============================================================
// PokémonTool — Notification Consumer Service
// Consumes RabbitMQ queues and dispatches real-time SSE events.
//
// Flow:
//   Python/Go publishes listing → RabbitMQ queue
//   → This service receives it
//   → Checks if any user's watchlist matches
//   → Inserts alert into PostgreSQL
//   → Pushes live notification via SSE to connected browser
// ============================================================

const { getChannel, QUEUES } = require('../config/rabbitmq');
const { pool }   = require('../config/db');
const sseManager = require('./sseManager');

/**
 * Start consuming all RabbitMQ queues relevant to alerting.
 * Called once during server bootstrap.
 */
async function startConsuming() {
    const channel = getChannel();

    // Consume new card listings from both the Python API consumer and Go scraper
    await channel.consume(QUEUES.LISTINGS,         handleNewListing,   { noAck: false });
    await channel.consume(QUEUES.SCRAPED_LISTINGS, handleNewListing,   { noAck: false });
    await channel.consume(QUEUES.ANALYTICS,        handleAnalyticsEvent, { noAck: false });

    console.log('[Notification] Consuming queues:', Object.values(QUEUES).join(', '));
}

// ------------------------------------------------------------
// Handle a new listing event published by Python or Go worker
// ------------------------------------------------------------
async function handleNewListing(msg) {
    if (!msg) return;
    const channel = getChannel();

    try {
        const listing = JSON.parse(msg.content.toString());
        /*
         * Expected listing payload (from Python/Go):
         * {
         *   cardName:    "Charizard Base Set",
         *   price:       45.00,
         *   marketplace: "ebay",
         *   listingUrl:  "https://ebay.com/itm/...",
         *   seller:      "seller_username",
         *   condition:   "NM",
         * }
         */

        // Find all users watching this card and check if price thresholds are met
        const watchlist = await pool.query(
            `SELECT w.id, w.user_id, w.card_name, w.target_buy_price, w.target_sell_price
             FROM watchlists w
             WHERE LOWER(w.card_name) = LOWER($1)`,
            [listing.cardName]
        );

        // For each interested user, check if the price triggers an alert
        for (const watch of watchlist.rows) {
            let alertType = null;
            let message   = '';

            // Price dropped below their buy target — time to buy!
            if (watch.target_buy_price && listing.price <= watch.target_buy_price) {
                alertType = 'PRICE_DROP';
                message   = `🔥 ${listing.cardName} is listed at $${listing.price} on ${listing.marketplace} — below your $${watch.target_buy_price} buy target!`;
            }
            // Price spiked above their sell target — time to list!
            else if (watch.target_sell_price && listing.price >= watch.target_sell_price) {
                alertType = 'PRICE_SPIKE';
                message   = `📈 ${listing.cardName} hit $${listing.price} on ${listing.marketplace} — above your $${watch.target_sell_price} sell target!`;
            }

            if (alertType) {
                // Save alert to database so user sees it even if offline
                await pool.query(
                    `INSERT INTO alerts (user_id, card_name, alert_type, message, marketplace, price, listing_url)
                     VALUES ($1,$2,$3,$4,$5,$6,$7)`,
                    [watch.user_id, listing.cardName, alertType, message, listing.marketplace, listing.price, listing.listingUrl]
                );

                // Push real-time SSE notification if user is currently connected
                sseManager.sendToUser(watch.user_id, alertType, {
                    cardName:   listing.cardName,
                    price:      listing.price,
                    marketplace: listing.marketplace,
                    listingUrl: listing.listingUrl,
                    message,
                });
            }
        }

        // Acknowledge the message — tells RabbitMQ we processed it successfully
        channel.ack(msg);
    } catch (err) {
        console.error('[Notification] Error processing listing:', err.message);
        // Negative-ack with requeue=false — don't loop forever on bad messages
        channel.nack(msg, false, false);
    }
}

// ------------------------------------------------------------
// Handle analytics events (trending updates, deal-of-day found)
// ------------------------------------------------------------
async function handleAnalyticsEvent(msg) {
    if (!msg) return;
    const channel = getChannel();

    try {
        const event = JSON.parse(msg.content.toString());

        if (event.type === 'TREND_CHANGE') {
            // Notify users watching a card whose trend just changed
            const watchers = await pool.query(
                'SELECT user_id FROM watchlists WHERE LOWER(card_name) = LOWER($1)',
                [event.cardName]
            );

            const message = event.trendLabel === 'RISING'
                ? `📊 ${event.cardName} is trending UP +${event.changePct}% in the last 7 days!`
                : `📉 ${event.cardName} is trending DOWN -${Math.abs(event.changePct)}% in the last 7 days.`;

            for (const { user_id } of watchers.rows) {
                await pool.query(
                    'INSERT INTO alerts (user_id, card_name, alert_type, message) VALUES ($1,$2,$3,$4)',
                    [user_id, event.cardName, 'TREND_CHANGE', message]
                );
                sseManager.sendToUser(user_id, 'TREND_UPDATE', { cardName: event.cardName, message, trendLabel: event.trendLabel });
            }
        }

        channel.ack(msg);
    } catch (err) {
        console.error('[Notification] Error processing analytics event:', err.message);
        channel.nack(msg, false, false);
    }
}

module.exports = { startConsuming };
