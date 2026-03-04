// ============================================================
// PokémonTool — Inventory Controller
// Manages a vendor's personal card collection
// Supports CSV import/export for portability
// ============================================================

const { pool }   = require('../config/db');
const { parse }  = require('csv-parse/sync');
const { createError } = require('../middleware/errorHandler');

// @route GET /api/inventory
exports.getInventory = async (req, res, next) => {
    try {
        const { id: userId } = req.user;
        const result = await pool.query(
            `SELECT id, card_name, set_name, card_number, condition, quantity,
                    purchase_price, current_value, notes, acquired_at
             FROM inventory WHERE user_id = $1 ORDER BY card_name ASC`,
            [userId]
        );
        res.json({ inventory: result.rows, total: result.rowCount });
    } catch (err) { next(err); }
};

// @route POST /api/inventory
// @body  { cardName, setName, cardNumber, condition, quantity, purchasePrice, notes }
exports.addCard = async (req, res, next) => {
    try {
        const { id: userId } = req.user;
        const { cardName, setName, cardNumber, condition, quantity, purchasePrice, notes } = req.body;
        if (!cardName) throw createError(400, 'Card name is required.');

        const result = await pool.query(
            `INSERT INTO inventory (user_id, card_name, set_name, card_number, condition, quantity, purchase_price, notes)
             VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING *`,
            [userId, cardName, setName||null, cardNumber||null, condition||'NM', quantity||1, purchasePrice||null, notes||null]
        );
        res.status(201).json({ card: result.rows[0] });
    } catch (err) { next(err); }
};

// @route PUT /api/inventory/:id
exports.updateCard = async (req, res, next) => {
    try {
        const { id: userId } = req.user;
        const { id }         = req.params;
        const { quantity, condition, notes, currentValue } = req.body;

        const result = await pool.query(
            `UPDATE inventory
             SET quantity = COALESCE($1, quantity),
                 condition = COALESCE($2, condition),
                 notes = COALESCE($3, notes),
                 current_value = COALESCE($4, current_value),
                 updated_at = NOW()
             WHERE id = $5 AND user_id = $6 RETURNING *`,
            [quantity, condition, notes, currentValue, id, userId]
        );
        if (result.rowCount === 0) throw createError(404, 'Inventory item not found.');
        res.json({ card: result.rows[0] });
    } catch (err) { next(err); }
};

// @route DELETE /api/inventory/:id
exports.deleteCard = async (req, res, next) => {
    try {
        const { id: userId } = req.user;
        const { id }         = req.params;
        const result = await pool.query(
            'DELETE FROM inventory WHERE id = $1 AND user_id = $2 RETURNING id',
            [id, userId]
        );
        if (result.rowCount === 0) throw createError(404, 'Inventory item not found.');
        res.json({ message: 'Card removed from inventory.' });
    } catch (err) { next(err); }
};

// @route POST /api/inventory/import
// Accepts a CSV file with columns: card_name, set_name, card_number, condition, quantity, purchase_price
exports.importCSV = async (req, res, next) => {
    try {
        const { id: userId } = req.user;
        if (!req.file) throw createError(400, 'No CSV file uploaded.');

        // Parse the uploaded CSV buffer
        const records = parse(req.file.buffer.toString(), {
            columns:           true,   // Use first row as headers
            skip_empty_lines:  true,
            trim:              true,
        });

        if (records.length === 0) throw createError(400, 'CSV file is empty.');
        if (records.length > 5000) throw createError(400, 'CSV limit is 5,000 rows per import.');

        // Bulk insert using a single transaction for performance
        const client = await pool.connect();
        let importedCount = 0;
        try {
            await client.query('BEGIN');
            for (const row of records) {
                await client.query(
                    `INSERT INTO inventory (user_id, card_name, set_name, card_number, condition, quantity, purchase_price)
                     VALUES ($1,$2,$3,$4,$5,$6,$7)
                     ON CONFLICT DO NOTHING`, // Skip duplicate rows
                    [
                        userId,
                        row.card_name  || row['Card Name']  || '',
                        row.set_name   || row['Set Name']   || null,
                        row.card_number|| row['Card Number']|| null,
                        row.condition  || row['Condition']  || 'NM',
                        parseInt(row.quantity || row['Quantity'] || 1),
                        parseFloat(row.purchase_price || row['Purchase Price'] || 0) || null,
                    ]
                );
                importedCount++;
            }
            await client.query('COMMIT');
        } catch (err) {
            await client.query('ROLLBACK');
            throw err;
        } finally {
            client.release();
        }

        res.json({ message: `Successfully imported ${importedCount} cards.`, importedCount });
    } catch (err) { next(err); }
};

// @route GET /api/inventory/export
// Returns the user's inventory as a downloadable CSV file
exports.exportCSV = async (req, res, next) => {
    try {
        const { id: userId } = req.user;
        const result = await pool.query(
            'SELECT card_name, set_name, card_number, condition, quantity, purchase_price, notes FROM inventory WHERE user_id = $1 ORDER BY card_name',
            [userId]
        );

        // Build CSV string manually
        const header = 'card_name,set_name,card_number,condition,quantity,purchase_price,notes\n';
        const rows   = result.rows.map(r =>
            `"${r.card_name}","${r.set_name||''}","${r.card_number||''}","${r.condition}",${r.quantity},${r.purchase_price||''},"${r.notes||''}"`
        ).join('\n');

        res.setHeader('Content-Type', 'text/csv');
        res.setHeader('Content-Disposition', 'attachment; filename="pokemontool_inventory.csv"');
        res.send(header + rows);
    } catch (err) { next(err); }
};
