// ============================================================
// PokémonTool — Node.js API Gateway Entry Point
// Handles: REST API routes, SSE streaming, and RabbitMQ consumer
// ============================================================

require('dotenv').config();
const express = require('express');
const cors    = require('cors');
const helmet  = require('helmet');
const morgan  = require('morgan');
const http    = require('http');
const path    = require('path');

// --- Internal modules ---
const { connectPostgres } = require('./config/db');
const { connectRedis }    = require('./config/redis');
const { connectRabbitMQ } = require('./config/rabbitmq');
const sseManager          = require('./services/sseManager');
const notificationService = require('./services/notificationService');

// --- Routes ---
const authRoutes       = require('./routes/auth');
const watchlistRoutes  = require('./routes/watchlist');
const alertRoutes      = require('./routes/alerts');
const cardRoutes       = require('./routes/cards');
const inventoryRoutes  = require('./routes/inventory');
const showRoutes       = require('./routes/shows');
const apiKeyRoutes     = require('./routes/apikeys');
const dealRoutes       = require('./routes/deals');

// --- Middleware ---
const { authMiddleware } = require('./middleware/authMiddleware');
const { errorHandler }   = require('./middleware/errorHandler');
const rateLimiter        = require('./middleware/rateLimiter');

const app    = express();
const server = http.createServer(app);
const PORT   = process.env.PORT || 3001;

// ---- Global Middleware ----
app.use(helmet());                             // Sets secure HTTP headers
app.use(cors({ origin: process.env.CLIENT_URL || 'http://localhost:5173' }));
app.use(morgan('dev'));                         // Request logging
app.use(express.json({ limit: '10mb' }));      // Parse JSON bodies (10mb for CSV)
app.use(express.urlencoded({ extended: true }));
app.use('/api', rateLimiter);                  // Rate limit all /api routes

// ---- Health Check (unauthenticated) ----
app.get('/health', (req, res) => {
    res.json({ status: 'ok', timestamp: new Date().toISOString(), service: 'pokemontool-server' });
});

// ---- SSE Stream Endpoint ----
// Clients connect here once and receive real-time price alerts + notifications
app.get('/api/stream', authMiddleware, (req, res) => {
    const userId = req.user.id;

    // Set SSE headers — keep connection alive and disable buffering
    res.setHeader('Content-Type', 'text/event-stream');
    res.setHeader('Cache-Control', 'no-cache');
    res.setHeader('Connection', 'keep-alive');
    res.setHeader('X-Accel-Buffering', 'no');  // Needed for nginx proxying
    res.flushHeaders();

    // Register this client connection with SSE manager
    sseManager.addClient(userId, res);

    // Send a welcome ping so the client knows the stream is live
    res.write(`data: ${JSON.stringify({ type: 'CONNECTED', message: 'Stream connected' })}\n\n`);

    // Clean up when the client disconnects
    req.on('close', () => {
        sseManager.removeClient(userId);
    });
});

// ---- API Routes (all require JWT auth except /auth) ----
app.use('/api/auth',      authRoutes);
app.use('/api/watchlist', authMiddleware, watchlistRoutes);
app.use('/api/alerts',    authMiddleware, alertRoutes);
app.use('/api/cards',     authMiddleware, cardRoutes);
app.use('/api/inventory', authMiddleware, inventoryRoutes);
app.use('/api/shows',     authMiddleware, showRoutes);
app.use('/api/apikeys',   authMiddleware, apiKeyRoutes);
app.use('/api/deals',     authMiddleware, dealRoutes);

// ---- Global Error Handler (must be last middleware) ----
app.use(errorHandler);

// ============================================================
// Bootstrap — Connect to all dependencies, then start server
// ============================================================
async function bootstrap() {
    try {
        console.log('🔌 Connecting to databases...');
        await connectPostgres();
        await connectRedis();
        console.log('✅ Databases connected');

        // RabbitMQ is optional — server works without it (alerts won't fire)
        try {
            await connectRabbitMQ();
            await notificationService.startConsuming();
            console.log('✅ RabbitMQ consumer started');
        } catch (rmqErr) {
            console.warn('⚠️  RabbitMQ not available — real-time alerts disabled:', rmqErr.message);
        }

        server.listen(PORT, () => {
            console.log(`\n🚀 PokémonTool API Gateway running on port ${PORT}`);
            console.log(`   Dashboard: http://localhost:${PORT}/health`);
            console.log(`   SSE Stream: http://localhost:${PORT}/api/stream\n`);
        });
    } catch (err) {
        console.error('❌ Bootstrap failed:', err);
        process.exit(1);
    }
}

bootstrap();

module.exports = { app, server }; // Export for testing
