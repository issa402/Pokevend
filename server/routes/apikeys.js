// ============================================================
// PokémonTool — API Keys Routes
// POST   /api/apikeys         - Save or update an API key
// GET    /api/apikeys          - List which platforms have keys saved
// DELETE /api/apikeys/:platform - Remove a key for a platform
// ============================================================

const express = require('express');
const router  = express.Router();
const ctrl    = require('../controllers/apiKeysController');

router.get('/',              ctrl.listKeys);    // Returns platform names only (NOT the values)
router.post('/',             ctrl.saveKey);     // Encrypts and stores the key
router.delete('/:platform',  ctrl.deleteKey);  // Remove a platform's key

module.exports = router;
