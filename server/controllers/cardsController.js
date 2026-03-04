// ============================================================
// PokémonTool — Cards Controller (PostgreSQL edition)
// All card data now stored in PostgreSQL — no MongoDB needed.
//
// PostgreSQL tables used:
//   cards          — card metadata + trend scores
//   card_listings  — individual listings from all marketplaces
//   price_history  — daily price snapshots per card (for charts)
// ============================================================

const { pool }   = require('../config/db');
const { getCache, setCache } = require('../config/redis');
const { createError } = require('../middleware/errorHandler');

// @route GET /api/cards/search?q=charizard&limit=20
// Full-text search against the cards table using PostgreSQL ILIKE
exports.searchCards = async (req, res, next) => {
    try {
        const { q, limit = 20 } = req.query;
        if (!q || q.trim().length < 2) throw createError(400, 'Search query must be at least 2 characters.');

        // Check Redis cache first — avoid DB round-trip on repeated searches
        const cacheKey = `search:${q.toLowerCase().trim()}:${limit}`;
        const cached   = await getCache(cacheKey);
        if (cached) return res.json({ cards: cached, source: 'cache' });

        // Use pg_trgm-powered similarity or simple ILIKE for case-insensitive search
        const result = await pool.query(
            `SELECT card_id, name, set_name, set_code, image_url,
                    trending_score, trend_label,
                    price_ebay, price_tcgplayer, price_facebook, price_mercari,
                    last_updated
             FROM cards
             WHERE name ILIKE $1 OR set_name ILIKE $1
             ORDER BY trending_score DESC
             LIMIT $2`,
            [`%${q.trim()}%`, parseInt(limit)]
        );

        // Normalize column names to camelCase for the React frontend
        const cards = result.rows.map(normalizeCard);

        // Cache for 5 minutes
        await setCache(cacheKey, cards, 300);
        res.json({ cards, source: 'database' });
    } catch (err) { next(err); }
};

// @route GET /api/cards/trending
// Returns top 10 rising and top 10 falling cards (updated hourly by analytics engine)
exports.getTrending = async (req, res, next) => {
    try {
        const cacheKey = 'trending:all';
        const cached   = await getCache(cacheKey);
        if (cached) return res.json(cached);

        const [risingRes, fallingRes] = await Promise.all([
            pool.query(
                `SELECT card_id, name, set_name, image_url,
                        trending_score, trend_label, pct_change_7d,
                        price_ebay, price_tcgplayer
                 FROM cards WHERE trend_label = 'RISING'
                 ORDER BY trending_score DESC LIMIT 10`
            ),
            pool.query(
                `SELECT card_id, name, set_name, image_url,
                        trending_score, trend_label, pct_change_7d,
                        price_ebay, price_tcgplayer
                 FROM cards WHERE trend_label = 'FALLING'
                 ORDER BY trending_score ASC LIMIT 10`
            ),
        ]);

        const response = {
            rising:    risingRes.rows.map(normalizeCard),
            falling:   fallingRes.rows.map(normalizeCard),
            updatedAt: new Date().toISOString(),
        };

        // Cache for 30 minutes — analytics engine recalculates hourly
        await setCache(cacheKey, response, 1800);
        res.json(response);
    } catch (err) { next(err); }
};

// @route GET /api/cards/:id/history
// Returns 30-day price history for a specific card (used for Chart.js line chart)
exports.getPriceHistory = async (req, res, next) => {
    try {
        const { id } = req.params;

        // Fetch card metadata
        const cardRes = await pool.query(
            'SELECT card_id, name, set_name, image_url FROM cards WHERE card_id = $1',
            [id]
        );
        if (cardRes.rowCount === 0) throw createError(404, `Card "${id}" not found.`);

        // Fetch 30-day price history ordered oldest-first (for chart labels)
        const historyRes = await pool.query(
            `SELECT date, avg_price, price_ebay, price_tcgplayer
             FROM price_history
             WHERE card_id = $1
             ORDER BY date ASC
             LIMIT 30`,
            [id]
        );

        res.json({
            card: {
                ...normalizeCard(cardRes.rows[0]),
                priceHistory: historyRes.rows.map(h => ({
                    date:         h.date,
                    avgPrice:     parseFloat(h.avg_price),
                    marketplaces: {
                        ebay:      h.price_ebay ? parseFloat(h.price_ebay) : null,
                        tcgplayer: h.price_tcgplayer ? parseFloat(h.price_tcgplayer) : null,
                    },
                })),
            },
        });
    } catch (err) { next(err); }
};

// ---- Helper ----
// Normalizes PostgreSQL snake_case column names to camelCase for the frontend
function normalizeCard(row) {
    return {
        cardId:        row.card_id,
        name:          row.name,
        setName:       row.set_name,
        setCode:       row.set_code,
        imageUrl:      row.image_url,
        trendingScore: row.trending_score,
        trendLabel:    row.trend_label || 'STABLE',
        pctChange7d:   row.pct_change_7d ? parseFloat(row.pct_change_7d) : null,
        lastUpdated:   row.last_updated,
        prices: {
            ebay:      row.price_ebay      ? parseFloat(row.price_ebay)      : null,
            tcgplayer: row.price_tcgplayer ? parseFloat(row.price_tcgplayer) : null,
            facebook:  row.price_facebook  ? parseFloat(row.price_facebook)  : null,
            mercari:   row.price_mercari   ? parseFloat(row.price_mercari)   : null,
        },
    };
}
