-- ============================================================
-- PokémonTool — Seed Data
-- Pokémon TCG sets for autocomplete in the UI
-- ============================================================

INSERT INTO shows (name, venue_name, city, state, start_date, event_url)
VALUES
    ('Pokemon Regional Championship - Dallas', 'Kay Bailey Hutchison Convention Center', 'Dallas', 'TX', NOW() + INTERVAL '30 days', 'https://www.pokemon.com/us/play-pokemon/'),
    ('Pokemon Local League Show - Chicago', 'Chicago Card Expo Center', 'Chicago', 'IL', NOW() + INTERVAL '14 days', 'https://www.pokemon.com/us/play-pokemon/'),
    ('TCG Collectors Bazaar', 'Miami Beach Convention Center', 'Miami', 'FL', NOW() + INTERVAL '45 days', 'https://eventbrite.com')
ON CONFLICT DO NOTHING;
