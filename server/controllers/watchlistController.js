// ============================================================
// PokémonTool — Watchlist Controller
// Vendors add cards here to monitor for price changes
// ============================================================

const { pool } = require('../config/db');
const { createError } = require('../middleware/errorHandler');

// @route GET /api/watchlist
// Returns all watchlist entries for the logged-in user
exports.getWatchlist = async (req, res, next) => {
    try {
        const { id: userId } = req.user;
        const result = await pool.query(
            `SELECT id, card_name, set_name, target_buy_price, target_sell_price, notes, added_at
             FROM watchlists WHERE user_id = $1 ORDER BY added_at DESC`,
            [userId]
        );
        res.json({ watchlist: result.rows });
    } catch (err) { next(err); }
};

// @route POST /api/watchlist
// @body  { cardName, setName, targetBuyPrice, targetSellPrice, notes }
exports.addToWatchlist = async (req, res, next) => {
    try {
        const { id: userId }                                              = req.user;
        const { cardName, setName, targetBuyPrice, targetSellPrice, notes } = req.body;

        if (!cardName) throw createError(400, 'Card name is required.');

        const result = await pool.query(
            `INSERT INTO watchlists (user_id, card_name, set_name, target_buy_price, target_sell_price, notes)
             VALUES ($1, $2, $3, $4, $5, $6)
             RETURNING id, card_name, set_name, target_buy_price, target_sell_price, notes, added_at`,
            [userId, cardName, setName || null, targetBuyPrice || null, targetSellPrice || null, notes || null]
        );

        res.status(201).json({
            message: `${cardName} added to watchlist.`,
            item: result.rows[0],
        });
    } catch (err) { next(err); }
};

// @route DELETE /api/watchlist/:id
exports.removeFromWatchlist = async (req, res, next) => {
    try {
        const { id: userId } = req.user;
        const { id }         = req.params;

        // Ensure the row belongs to this user before deleting
        const result = await pool.query(
            'DELETE FROM watchlists WHERE id = $1 AND user_id = $2 RETURNING id',
            [id, userId]
        );

        if (result.rowCount === 0) throw createError(404, 'Watchlist item not found.');
        res.json({ message: 'Card removed from watchlist.' });
    } catch (err) { next(err); }
};
