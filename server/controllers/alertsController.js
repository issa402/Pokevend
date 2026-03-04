// ============================================================
// PokémonTool — Alerts Controller
// Handles user notifications — price drops, new listings, trends
// ============================================================

const { pool } = require('../config/db');
const { createError } = require('../middleware/errorHandler');

// @route GET /api/alerts?unread=true&limit=50
exports.getAlerts = async (req, res, next) => {
    try {
        const { id: userId }     = req.user;
        const { unread, limit = 50 } = req.query;

        // Optionally filter to only unread alerts
        const whereClause = unread === 'true'
            ? 'WHERE user_id = $1 AND is_read = false'
            : 'WHERE user_id = $1';

        const result = await pool.query(
            `SELECT id, card_name, alert_type, message, marketplace, price, listing_url, is_read, created_at
             FROM alerts ${whereClause}
             ORDER BY created_at DESC LIMIT $2`,
            [userId, parseInt(limit)]
        );

        // Also return count of unread alerts for the badge in the UI
        const unreadCount = await pool.query(
            'SELECT COUNT(*) FROM alerts WHERE user_id = $1 AND is_read = false',
            [userId]
        );

        res.json({
            alerts:      result.rows,
            unreadCount: parseInt(unreadCount.rows[0].count),
        });
    } catch (err) { next(err); }
};

// @route PUT /api/alerts/:id/read
exports.markRead = async (req, res, next) => {
    try {
        const { id: userId } = req.user;
        const { id }         = req.params;

        const result = await pool.query(
            'UPDATE alerts SET is_read = true WHERE id = $1 AND user_id = $2 RETURNING id',
            [id, userId]
        );
        if (result.rowCount === 0) throw createError(404, 'Alert not found.');
        res.json({ message: 'Alert marked as read.' });
    } catch (err) { next(err); }
};

// @route PUT /api/alerts/read-all
exports.markAllRead = async (req, res, next) => {
    try {
        const { id: userId } = req.user;
        await pool.query('UPDATE alerts SET is_read = true WHERE user_id = $1', [userId]);
        res.json({ message: 'All alerts marked as read.' });
    } catch (err) { next(err); }
};

// @route DELETE /api/alerts/:id
exports.deleteAlert = async (req, res, next) => {
    try {
        const { id: userId } = req.user;
        const { id }         = req.params;
        const result = await pool.query(
            'DELETE FROM alerts WHERE id = $1 AND user_id = $2 RETURNING id',
            [id, userId]
        );
        if (result.rowCount === 0) throw createError(404, 'Alert not found.');
        res.json({ message: 'Alert deleted.' });
    } catch (err) { next(err); }
};
