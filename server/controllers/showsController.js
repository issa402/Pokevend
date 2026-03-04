// ============================================================
// PokémonTool — Shows Controller
// Fetches upcoming Pokemon TCG events from:
//   1. Local PostgreSQL cache (refreshed daily)
//   2. Eventbrite API (when cache is stale)
// ============================================================

const axios   = require('axios');
const { pool }= require('../config/db');
const { getCache, setCache } = require('../config/redis');
const { createError } = require('../middleware/errorHandler');

// @route GET /api/shows/upcoming?zip=10001&radius=100
exports.getUpcomingShows = async (req, res, next) => {
    try {
        const { zip = '', radius = 150 } = req.query;

        // Try Redis cache first for this query
        const cacheKey = `shows:${zip}:${radius}`;
        const cached   = await getCache(cacheKey);
        if (cached) return res.json({ shows: cached, source: 'cache' });

        // Pull from the database (seeded daily by analytics engine)
        const result = await pool.query(
            `SELECT id, name, venue_name, address, city, state, zip_code,
                    latitude, longitude, start_date, end_date, event_url, description
             FROM shows
             WHERE start_date >= NOW()
             ORDER BY start_date ASC
             LIMIT 50`
        );

        let shows = result.rows;

        // If we have an Eventbrite token, try to fetch live results too
        const eventbriteToken = process.env.EVENTBRITE_TOKEN;
        if (eventbriteToken && shows.length < 5) {
            try {
                // Search Eventbrite for Pokemon shows near the user's zip
                const ebResponse = await axios.get('https://www.eventbriteapi.com/v3/events/search/', {
                    headers: { Authorization: `Bearer ${eventbriteToken}` },
                    params: {
                        q:              'pokemon cards',
                        'location.address': zip || 'United States',
                        'location.within':  `${radius}mi`,
                        sort_by:        'date',
                        'start_date.range_start': new Date().toISOString(),
                        expand:         'venue',
                    },
                    timeout: 5000, // Don't block the response for more than 5 seconds
                });

                // Map Eventbrite format to our internal format
                const liveShows = (ebResponse.data.events || []).map(ev => ({
                    name:       ev.name?.text,
                    venue_name: ev.venue?.name,
                    city:       ev.venue?.address?.city,
                    state:      ev.venue?.address?.region,
                    start_date: ev.start?.utc,
                    end_date:   ev.end?.utc,
                    event_url:  ev.url,
                    description: ev.description?.text?.substring(0, 300),
                }));

                shows = [...shows, ...liveShows];
            } catch (ebErr) {
                // Eventbrite is optional — log and continue with cached data
                console.warn('Eventbrite API fetch failed:', ebErr.message);
            }
        }

        // Cache for 30 minutes
        await setCache(cacheKey, shows, 1800);
        res.json({ shows, source: 'database' });
    } catch (err) { next(err); }
};
