// ============================================================
// PokémonTool — API Keys Controller
// Stores and retrieves platform API credentials using AES-256 encryption.
// Keys are encrypted BEFORE writing to the DB — so even if the DB
// is compromised, the raw keys cannot be recovered without ENCRYPTION_KEY.
// ============================================================

const crypto = require('crypto');
const { pool }= require('../config/db');
const { createError } = require('../middleware/errorHandler');

// AES-256-GCM encryption — GCM includes an authentication tag for integrity checking
const ALGORITHM = 'aes-256-gcm';

/**
 * Encrypts a plaintext string using AES-256-GCM.
 * Returns a hex string combining IV + AuthTag + Ciphertext.
 */
function encrypt(plaintext) {
    const key = Buffer.from(process.env.ENCRYPTION_KEY, 'hex'); // 32-byte key
    const iv  = crypto.randomBytes(16);  // Random IV per encryption call
    const cipher = crypto.createCipheriv(ALGORITHM, key, iv);

    let encrypted = cipher.update(plaintext, 'utf8', 'hex');
    encrypted += cipher.final('hex');
    const authTag = cipher.getAuthTag().toString('hex');

    // Store as "iv:authTag:ciphertext" so we can reverse it later
    return `${iv.toString('hex')}:${authTag}:${encrypted}`;
}

/**
 * Decrypts a string previously encrypted with encrypt().
 */
function decrypt(encryptedString) {
    const key = Buffer.from(process.env.ENCRYPTION_KEY, 'hex');
    const [iv, authTag, encrypted] = encryptedString.split(':');

    const decipher = crypto.createDecipheriv(ALGORITHM, key, Buffer.from(iv, 'hex'));
    decipher.setAuthTag(Buffer.from(authTag, 'hex'));

    let decrypted = decipher.update(encrypted, 'hex', 'utf8');
    decrypted += decipher.final('utf8');
    return decrypted;
}

// Allowed platforms — prevents arbitrary key names
const ALLOWED_PLATFORMS = ['ebay', 'tcgplayer', 'eventbrite'];

// @route GET /api/apikeys
// Returns which platforms have keys stored — NOT the actual key values
exports.listKeys = async (req, res, next) => {
    try {
        const { id: userId } = req.user;
        const result = await pool.query(
            'SELECT platform, key_label, created_at FROM api_keys WHERE user_id = $1 ORDER BY platform',
            [userId]
        );
        res.json({ keys: result.rows }); // Values are intentionally omitted
    } catch (err) { next(err); }
};

// @route POST /api/apikeys
// @body  { platform, keyValue, keyLabel }
exports.saveKey = async (req, res, next) => {
    try {
        const { id: userId }             = req.user;
        const { platform, keyValue, keyLabel } = req.body;

        if (!platform || !keyValue) throw createError(400, 'Platform and key value are required.');
        if (!ALLOWED_PLATFORMS.includes(platform.toLowerCase())) {
            throw createError(400, `Platform must be one of: ${ALLOWED_PLATFORMS.join(', ')}`);
        }

        // Encrypt the key before storage
        const encryptedKey = encrypt(keyValue.trim());

        // Upsert — replace existing key if already saved for this platform
        await pool.query(
            `INSERT INTO api_keys (user_id, platform, encrypted_key, key_label)
             VALUES ($1, $2, $3, $4)
             ON CONFLICT (user_id, platform)
             DO UPDATE SET encrypted_key = EXCLUDED.encrypted_key, key_label = EXCLUDED.key_label`,
            [userId, platform.toLowerCase(), encryptedKey, keyLabel || `${platform} API Key`]
        );

        res.json({ message: `${platform} API key saved and encrypted successfully.` });
    } catch (err) { next(err); }
};

// @route DELETE /api/apikeys/:platform
exports.deleteKey = async (req, res, next) => {
    try {
        const { id: userId } = req.user;
        const { platform }   = req.params;
        const result = await pool.query(
            'DELETE FROM api_keys WHERE user_id = $1 AND platform = $2 RETURNING platform',
            [userId, platform.toLowerCase()]
        );
        if (result.rowCount === 0) throw createError(404, 'No key found for this platform.');
        res.json({ message: `${platform} API key removed.` });
    } catch (err) { next(err); }
};

// Export decrypt so worker services can retrieve keys via internal service calls
module.exports.decrypt = decrypt;
