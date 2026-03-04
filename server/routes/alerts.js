// ============================================================
// PokémonTool — Alerts Routes
// GET /api/alerts              - Get all alerts for user
// PUT /api/alerts/:id/read     - Mark alert as read
// DELETE /api/alerts/:id       - Delete an alert
// DELETE /api/alerts/read-all  - Mark all as read
// ============================================================

const express = require('express');
const router  = express.Router();
const ctrl    = require('../controllers/alertsController');

router.get('/',              ctrl.getAlerts);
router.put('/read-all',      ctrl.markAllRead);   // Must be BEFORE /:id to avoid conflict
router.put('/:id/read',      ctrl.markRead);
router.delete('/:id',        ctrl.deleteAlert);

module.exports = router;
