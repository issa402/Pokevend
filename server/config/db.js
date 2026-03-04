// ============================================================
// PokémonTool — Database Configuration (PostgreSQL only)
// Removed MongoDB entirely — everything lives in PostgreSQL.
//
// Why PostgreSQL over MongoDB for this project?
//   - We already have a relational schema (users, watchlists, alerts)
//   - Card data has a predictable structure — no need for document flexibility
//   - Joins between users ↔ watchlists ↔ cards are cleaner in SQL
//   - One database = simpler ops, simpler Docker setup, lower memory use
// ============================================================

const { Pool } = require('pg');

// Connection pool — efficiently reuses connections across all requests.
// In production, pool size should match your Postgres max_connections setting.
const pool = new Pool({
    host:     process.env.POSTGRES_HOST     || 'localhost',
    port:     parseInt(process.env.POSTGRES_PORT) || 5432,
    database: process.env.POSTGRES_DB       || 'pokemontool',
    user:     process.env.POSTGRES_USER     || 'pokemontool_user',
    password: process.env.POSTGRES_PASSWORD || 'pokemontool_pass',
    max:      20,     // Max simultaneous connections in the pool
    idleTimeoutMillis:       30000,
    connectionTimeoutMillis: 3000,
});

// Log any unexpected pool errors (e.g. database restart)
pool.on('error', (err) => {
    console.error('Unexpected PostgreSQL pool error:', err.message);
});

/**
 * Verifies the PostgreSQL connection at startup.
 * Throws on failure so the server refuses to start without a DB.
 */
async function connectPostgres() {
    const client = await pool.connect();
    const { rows } = await client.query('SELECT current_database(), version()');
    console.log(`  ✓ PostgreSQL connected (db: ${rows[0].current_database})`);
    client.release();
    return pool;
}

// Export pool for use across all controllers
module.exports = { pool, connectPostgres };
