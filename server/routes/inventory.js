// ============================================================
// PokémonTool — Inventory Routes
// GET    /api/inventory           - Get vendor's card inventory
// POST   /api/inventory           - Add a card to inventory
// PUT    /api/inventory/:id       - Update a card entry
// DELETE /api/inventory/:id       - Remove a card
// POST   /api/inventory/import    - Bulk import from CSV
// GET    /api/inventory/export    - Export inventory as CSV
// ============================================================

const express = require('express');
const router  = express.Router();
const multer  = require('multer');
const ctrl    = require('../controllers/inventoryController');

// Multer: store uploaded CSV files in memory (not on disk)
const upload = multer({
    storage: multer.memoryStorage(),
    limits:  { fileSize: 5 * 1024 * 1024 }, // 5MB max
    fileFilter: (req, file, cb) => {
        // Only accept CSV files
        if (file.mimetype === 'text/csv' || file.originalname.endsWith('.csv')) {
            cb(null, true);
        } else {
            cb(new Error('Only CSV files are accepted for inventory import.'));
        }
    },
});

router.get('/',              ctrl.getInventory);
router.post('/',             ctrl.addCard);
router.put('/:id',           ctrl.updateCard);
router.delete('/:id',        ctrl.deleteCard);
router.post('/import',       upload.single('file'), ctrl.importCSV);
router.get('/export',        ctrl.exportCSV);

module.exports = router;
