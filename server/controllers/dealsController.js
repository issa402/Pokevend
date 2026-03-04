// ============================================================
// PokémonTool — Deals Controller (PostgreSQL edition)
// Reads daily deals written by the Python analytics engine
// into the PostgreSQL 'deals' table. No MongoDB required.
// ============================================================

const { pool }   = require('../config/db');
const { getCache, setCache } = require('../config/redis');

// @route GET /api/deals/today
exports.getDealsToday = async (req, res, next) => {
    try {
        const today    = new Date().toISOString().split('T')[0]; // "YYYY-MM-DD"
        const cacheKey = `deals:${today}`;
        const cached   = await getCache(cacheKey);
        if (cached) return res.json({ deals: cached, date: today, source: 'cache' });

        // Fetch today's top deals from PostgreSQL
        // The analytics engine writes to this table daily at 6 AM
        const result = await pool.query(
            `SELECT card_name, set_name, image_url,
                    market_price, best_price, savings, savings_pct,
                    listing_url, marketplace, reason, created_at
             FROM deals
             WHERE deal_date = $1
             ORDER BY savings_pct DESC
             LIMIT 10`,
            [today]
        );

        if (result.rowCount === 0) {
            return res.json({
                deals:   [],
                date:    today,
                message: 'Deals are computed at 6 AM daily. Check back soon!',
            });
        }

        // Shape the results for the frontend
        const deals = result.rows.map(r => ({
            cardName:    r.card_name,
            setName:     r.set_name,
            imageUrl:    r.image_url,
            marketPrice: parseFloat(r.market_price),
            bestPrice:   parseFloat(r.best_price),
            savings:     parseFloat(r.savings),
            savingsPct:  parseFloat(r.savings_pct),
            listingUrl:  r.listing_url,
            marketplace: r.marketplace,
            reason:      r.reason,
        }));

        // Cache deals for 30 minutes
        await setCache(cacheKey, deals, 1800);
        res.json({ deals, date: today, source: 'database' });
    } catch (err) { next(err); }
};
