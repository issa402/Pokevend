// ============================================================
// PokémonTool — SSE (Server-Sent Events) Manager
// Manages live connections from React clients.
// When a price alert fires, it's pushed HERE to reach the browser.
//
// Each user can have ONE active SSE connection (one dashboard tab).
// If a second tab opens, it replaces the first.
// ============================================================

// Map of userId → Express response object (the SSE stream)
const clients = new Map();

/**
 * Register a new client (called when a user connects to /api/stream).
 * @param {string}   userId - UUID of the authenticated user
 * @param {Response} res    - Express response object (kept open for SSE)
 */
function addClient(userId, res) {
    // If user already has an active connection, close the old one first
    if (clients.has(userId)) {
        try {
            clients.get(userId).end();
        } catch (_) {}
    }
    clients.set(userId, res);
    console.log(`[SSE] Client connected: ${userId} (total: ${clients.size})`);
}

/**
 * Deregister a client (called when the browser tab closes or disconnects).
 */
function removeClient(userId) {
    clients.delete(userId);
    console.log(`[SSE] Client disconnected: ${userId} (total: ${clients.size})`);
}

/**
 * Send a real-time event to a specific user.
 * @param {string} userId  - Target user's UUID
 * @param {string} type    - Event type: 'PRICE_ALERT', 'NEW_LISTING', 'TREND_UPDATE', 'DEAL_FOUND'
 * @param {object} payload - Event data to send to the client
 */
function sendToUser(userId, type, payload) {
    const res = clients.get(userId);
    if (!res) return; // User not connected — alert will be in their DB for next login

    const event = JSON.stringify({ type, ...payload, timestamp: new Date().toISOString() });
    try {
        res.write(`data: ${event}\n\n`);
        // res.flush() needed for some reverse proxies (nginx gzip buffering)
        if (typeof res.flush === 'function') res.flush();
    } catch (err) {
        console.error(`[SSE] Failed to send to ${userId}:`, err.message);
        removeClient(userId); // Clean up broken connection
    }
}

/**
 * Broadcast a message to ALL connected clients.
 * Used for system-wide events (e.g., "New Pokemon set revealed — prices shifting!").
 */
function broadcast(type, payload) {
    const event = JSON.stringify({ type, ...payload, timestamp: new Date().toISOString() });
    for (const [userId, res] of clients.entries()) {
        try {
            res.write(`data: ${event}\n\n`);
            if (typeof res.flush === 'function') res.flush();
        } catch (err) {
            removeClient(userId);
        }
    }
}

/**
 * Returns how many users are currently connected.
 */
function getConnectedCount() {
    return clients.size;
}

module.exports = { addClient, removeClient, sendToUser, broadcast, getConnectedCount };
