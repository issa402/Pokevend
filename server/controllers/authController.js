// ============================================================
// PokémonTool — Auth Controller
// Handles user registration, login, and logout logic
// ============================================================

const bcrypt = require('bcryptjs');
const jwt    = require('jsonwebtoken');
const { pool } = require('../config/db');
const { createError } = require('../middleware/errorHandler');

/**
 * Generate a signed JWT for the given user.
 * Expires in 7 days by default.
 */
function generateToken(user) {
    return jwt.sign(
        { id: user.id, email: user.email },
        process.env.JWT_SECRET,
        { expiresIn: '7d' }
    );
}

// ------------------------------------------------------------
// @route   POST /api/auth/register
// @desc    Create a new vendor account
// @body    { email, password, displayName }
// ------------------------------------------------------------
exports.register = async (req, res, next) => {
    try {
        const { email, password, displayName } = req.body;

        // Validate required fields
        if (!email || !password) {
            throw createError(400, 'Email and password are required.');
        }
        if (password.length < 8) {
            throw createError(400, 'Password must be at least 8 characters.');
        }

        // Check if email is already taken
        const existing = await pool.query('SELECT id FROM users WHERE email = $1', [email]);
        if (existing.rows.length > 0) {
            throw createError(409, 'An account with this email already exists.');
        }

        // Hash the password before storing — NEVER store plaintext
        const saltRounds   = 12;
        const passwordHash = await bcrypt.hash(password, saltRounds);

        // Insert the new user
        const result = await pool.query(
            'INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3) RETURNING id, email, display_name',
            [email, passwordHash, displayName || null]
        );
        const user = result.rows[0];

        // Issue a token immediately so they don't have to log in separately
        const token = generateToken(user);

        res.status(201).json({
            message: 'Account created successfully. Welcome to PokémonTool!',
            token,
            user: { id: user.id, email: user.email, displayName: user.display_name },
        });
    } catch (err) {
        next(err);
    }
};

// ------------------------------------------------------------
// @route   POST /api/auth/login
// @body    { email, password }
// ------------------------------------------------------------
exports.login = async (req, res, next) => {
    try {
        const { email, password } = req.body;
        if (!email || !password) throw createError(400, 'Email and password are required.');

        // Fetch user by email
        const result = await pool.query(
            'SELECT id, email, password_hash, display_name FROM users WHERE email = $1',
            [email]
        );
        const user = result.rows[0];
        if (!user) throw createError(401, 'Invalid email or password.');

        // Compare provided password against stored hash
        const isMatch = await bcrypt.compare(password, user.password_hash);
        if (!isMatch) throw createError(401, 'Invalid email or password.');

        const token = generateToken(user);

        res.json({
            message: 'Login successful.',
            token,
            user: { id: user.id, email: user.email, displayName: user.display_name },
        });
    } catch (err) {
        next(err);
    }
};

// ------------------------------------------------------------
// @route   POST /api/auth/logout
// @desc    Stateless JWT — client discards token. Log event server-side.
// ------------------------------------------------------------
exports.logout = async (req, res) => {
    // With stateless JWTs, logout is handled client-side by discarding the token.
    // For enhanced security, you could add a token blacklist in Redis here.
    console.log(`User logged out at ${new Date().toISOString()}`);
    res.json({ message: 'Logged out successfully.' });
};
