// ============================================================
// PokémonTool — Shows, API Keys, & Deals Routes
// ============================================================

// --- Shows ---
const express    = require('express');
const showRouter = express.Router();
const showCtrl   = require('../controllers/showsController');

// GET /api/shows/upcoming?zip=10001&radius=100
showRouter.get('/upcoming', showCtrl.getUpcomingShows);
module.exports = showRouter;
