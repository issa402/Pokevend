// ============================================================
// PokémonTool — JWT Authentication Middleware
// Validates Bearer tokens on all protected routes
// ============================================================

const jwt = require('jsonwebtoken');

/**
 * authMiddleware — Protects routes by validating the JWT token.
 * Attaches the decoded user payload (id, email) to req.user.
 * 
 * Usage: router.get('/protected', authMiddleware, handler)
 */
function authMiddleware(req, res, next) {
    // Extract token from Authorization: Bearer <token> header
    const authHeader = req.headers.authorization;
    const token = authHeader && authHeader.startsWith('Bearer ')
        ? authHeader.split(' ')[1]
        : null;

    if (!token) {
        return res.status(401).json({ error: 'No token provided. Please log in.' });
    }

    try {
        // Verify the token signature and expiration using the app's secret
        const decoded = jwt.verify(token, process.env.JWT_SECRET);
        req.user = decoded; // { id, email, iat, exp }
        next();
    } catch (err) {
        if (err.name === 'TokenExpiredError') {
            return res.status(401).json({ error: 'Session expired. Please log in again.' });
        }
        return res.status(403).json({ error: 'Invalid token.' });
    }
}

module.exports = { authMiddleware };
