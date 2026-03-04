// ============================================================
// PokémonTool — Deals Routes
// GET /api/deals/today  - Get today's best deals from analytics engine
// ============================================================

const express = require('express');
const router  = express.Router();
const ctrl    = require('../controllers/dealsController');

router.get('/today', ctrl.getDealsToday);

module.exports = router;
