// ============================================================
// PokémonTool — Global Error Handler Middleware
// Must be the LAST middleware registered in Express
// ============================================================

/**
 * Centralized error handler. Catches any error passed via next(err)
 * and returns a consistent JSON error response.
 * 
 * In production, hides internal error details from the client.
 */
function errorHandler(err, req, res, next) {
    // Log the full error for server-side debugging
    console.error(`[ERROR] ${req.method} ${req.path}:`, err.message);
    if (process.env.NODE_ENV !== 'production') {
        console.error(err.stack);
    }

    // Determine appropriate HTTP status code
    const status = err.statusCode || err.status || 500;

    // Send clean JSON response — never leak stack traces to clients in production
    res.status(status).json({
        error:   err.message || 'An unexpected error occurred',
        status,
        path:    req.path,
        ...(process.env.NODE_ENV === 'development' && { stack: err.stack }),
    });
}

/**
 * Helper: Creates an error object with a specific HTTP status code.
 * Usage: throw createError(404, 'Card not found')
 */
function createError(statusCode, message) {
    const err = new Error(message);
    err.statusCode = statusCode;
    return err;
}

module.exports = { errorHandler, createError };
