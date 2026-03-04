// ============================================================
// PokémonTool — Rate Limiter Middleware
// Protects the API from abuse and excessive crawling
// ============================================================

const rateLimit = require('express-rate-limit');

/**
 * Standard rate limiter: 100 requests per 15-minute window per IP.
 * Applied to all /api/* routes in index.js.
 */
const rateLimiter = rateLimit({
    windowMs:   15 * 60 * 1000, // 15 minutes in milliseconds
    max:        100,              // Max requests per window per IP
    standardHeaders: true,        // Return RateLimit-* headers in response
    legacyHeaders:   false,
    message: {
        error: 'Too many requests from this IP. Please slow down and try again after 15 minutes.',
    },
    skip: (req) => {
        // Don't rate-limit the SSE stream endpoint (it's a long-lived connection)
        return req.path === '/api/stream';
    },
});

module.exports = rateLimiter;
