// ============================================================
// PokémonTool — Watchlist Routes
// GET    /api/watchlist         - Get all watchlist items for user
// POST   /api/watchlist         - Add a card to watchlist
// DELETE /api/watchlist/:id     - Remove a card from watchlist
// ============================================================

const express = require('express');
const router  = express.Router();
const ctrl    = require('../controllers/watchlistController');

router.get('/',     ctrl.getWatchlist);   // Fetch user's watchlist
router.post('/',    ctrl.addToWatchlist); // Add card with price targets
router.delete('/:id', ctrl.removeFromWatchlist); // Remove by watchlist row ID

module.exports = router;
