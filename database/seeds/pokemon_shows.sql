-- ============================================================
-- FILE: database/seeds/pokemon_shows.sql
-- TYPE: Database Seed Data
--
-- WHAT IS SEED DATA?
-- Seed data = pre-loaded rows added to a fresh database so
-- the application immediately has useful data when first run.
-- Without seeds, your "upcoming shows" page would be empty for
-- new users — bad first impression.
--
-- SEEDS vs MIGRATIONS:
--   Migration (001_init.sql) = SCHEMA changes (CREATE TABLE, ALTER)
--   Seed (pokemon_shows.sql) = DATA loading (INSERT INTO)
-- Both are versioned SQL files, but serve different purposes.
--
-- FAANG PRACTICE: Separate schema from data.
-- Migrations run in ALL environments (dev, staging, production).
-- Seeds typically run in dev/staging only (not production).
-- Production data comes from real user actions, not seeds.
--
-- HOW THIS RUNS:
-- setup-dev.sh executes:
--   docker exec -i pokemontool_postgres psql -U pokemontool_user -d pokemontool < seeds/pokemon_shows.sql
--
-- ON CONFLICT DO NOTHING = safe to run multiple times.
-- If a show already exists (same eventbrite_id), skip it.
-- Without this, re-running seeds would cause duplicate key errors.

-- ============================================================
-- SAMPLE UPCOMING POKEMON TCG SHOWS
-- These are placeholder shows for development testing.
-- In production: Python analytics-engine populates this via Eventbrite API.
-- ============================================================
INSERT INTO shows (name, venue_name, city, state, zip_code, start_date, end_date, description)
VALUES
  (
    'Regional Pokémon TCG Championship',
    'Dallas Convention Center',
    'Dallas', 'TX', '75201',
    NOW() + INTERVAL '14 days',   -- NOW() = current timestamp
    NOW() + INTERVAL '15 days',   -- INTERVAL arithmetic: add 14 days to today
    'Official Regional Championship with VGC and TCG events. 3000+ attendees.'
  ),
  (
    'Local Card Show & Tournament',
    'Marriott Hotel Conference Center',
    'Austin', 'TX', '78701',
    NOW() + INTERVAL '7 days',
    NOW() + INTERVAL '7 days',
    'Monthly buy/sell/trade show with Pokémon TCG singles market.'
  ),
  (
    'Houston Pokemon Card Expo',
    'George R. Brown Convention Center',
    'Houston', 'TX', '77010',
    NOW() + INTERVAL '30 days',
    NOW() + INTERVAL '31 days',
    'Largest card show in Texas. Grading services, vendors, and tournaments.'
  )
-- ON CONFLICT DO NOTHING: if eventbrite_id matches an existing row, skip.
-- Since we're inserting without eventbrite_id, PostgreSQL checks the UUID PK.
-- Each run generates new UUIDs so there's no conflict — this is fine for seeds.
ON CONFLICT DO NOTHING;

-- Verify the seeds loaded (uncomment to run manually and check):
-- SELECT name, city, start_date FROM shows ORDER BY start_date;

-- ============================================================
-- TODO #1 (Practice): Add a seed for the cards table
-- Right now "cards" is empty until Python analytics-engine runs.
-- Add 5-10 famous Pokémon cards as seed data so search works immediately:
--   INSERT INTO cards (card_id, name, set_name, trend_label, price_tcgplayer)
--   VALUES ('base1-4', 'Charizard', 'Base Set', 'RISING', 450.00),
--          ('base1-2', 'Blastoise', 'Base Set', 'STABLE', 180.00), ...
--   ON CONFLICT (card_id) DO NOTHING;
-- This lets frontend developers test the search UI without running Python.

-- TODO #2 (Practice): Add a seed verification query
-- After inserts, add a verification:
--   DO $$
--   DECLARE show_count INT;
--   BEGIN
--     SELECT COUNT(*) INTO show_count FROM shows;
--     IF show_count = 0 THEN
--       RAISE EXCEPTION 'Seeds failed: shows table is empty!';
--     END IF;
--     RAISE NOTICE 'Seeds OK: % shows loaded', show_count;
--   END;
--   $$;
-- This PL/pgSQL block verifies data was inserted and fails with a clear error if not.
-- Research: PL/pgSQL DO blocks — PostgreSQL's anonymous procedure blocks
-- ============================================================
