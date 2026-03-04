// ============================================================
// PokémonTool — Redis Configuration
// Used for caching card prices and storing session data
// ============================================================

const Redis = require('ioredis');

let redisClient;

/**
 * Creates and returns a singleton Redis client.
 * Redis caches recently fetched card prices to reduce API calls.
 */
async function connectRedis() {
    redisClient = new Redis(process.env.REDIS_URL || 'redis://localhost:6379', {
        lazyConnect: false,
        retryStrategy: (times) => Math.min(times * 50, 2000), // Exponential backoff
    });

    redisClient.on('connect', ()  => console.log('  ✓ Redis connected'));
    redisClient.on('error',   (e) => console.error('Redis error:', e.message));

    await redisClient.ping(); // Confirm connection
    return redisClient;
}

/**
 * Cache a card price with a TTL (time-to-live).
 * @param {string} key    - e.g. "price:charizard:ebay"
 * @param {any}    value  - The price data to cache
 * @param {number} ttlSec - Seconds before cache expires (default: 5 min)
 */
async function setCache(key, value, ttlSec = 300) {
    if (!redisClient) return;
    await redisClient.setex(key, ttlSec, JSON.stringify(value));
}

/**
 * Retrieve a cached value.
 * Returns null if the key is expired or doesn't exist.
 */
async function getCache(key) {
    if (!redisClient) return null;
    const cached = await redisClient.get(key);
    return cached ? JSON.parse(cached) : null;
}

/**
 * Delete a cached key (call when a price update comes in).
 */
async function deleteCache(key) {
    if (!redisClient) return;
    await redisClient.del(key);
}

module.exports = { connectRedis, getCache, setCache, deleteCache, getClient: () => redisClient };
