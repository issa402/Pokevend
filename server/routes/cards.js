// ============================================================
// PokémonTool — Cards Routes
// GET /api/cards/search?q=charizard   - Search for a card
// GET /api/cards/trending             - Get trending/downtrending cards
// GET /api/cards/:id/history          - Get price history for a card
// ============================================================

const express = require('express');
const router  = express.Router();
const ctrl    = require('../controllers/cardsController');

router.get('/search',      ctrl.searchCards);   // Live search across all markets
router.get('/trending',    ctrl.getTrending);   // Analytics engine output
router.get('/:id/history', ctrl.getPriceHistory); // Chart data for a specific card

module.exports = router;
