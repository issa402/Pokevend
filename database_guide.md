# Database Engineering Guide
## PostgreSQL + SQL — Everything You Need for FAANG-Level Database Design

---

## 1. PostgreSQL at FAANG

PostgreSQL is used at **Apple, Instagram, Spotify, Reddit, Twitch, GitLab**.
It's the most feature-rich open-source relational database.

Why relational databases? When data has **relationships** (user owns cards, card has price history), SQL enforces those relationships at the database level — not just in your app code.

---

## 2. SQL Fundamentals

### SELECT — Reading Data
```sql
-- Basic read
SELECT id, name, price_ebay FROM cards;

-- Filter with WHERE
SELECT * FROM cards WHERE trend_label = 'RISING';

-- Sort + Limit (always limit in production — never SELECT * without LIMIT on big tables)
SELECT * FROM cards ORDER BY trending_score DESC LIMIT 10;

-- Multiple conditions
SELECT * FROM alerts
WHERE user_id = 'abc-123'
  AND is_read = FALSE
  AND created_at > NOW() - INTERVAL '7 days';

-- Pattern matching (ILIKE = case-insensitive LIKE)
SELECT * FROM cards WHERE name ILIKE '%charizard%';
-- % = wildcard (matches any characters)
```

### INSERT
```sql
-- Basic insert
INSERT INTO cards (card_id, name, price_ebay) VALUES ('ch-1', 'Charizard', 450.00);

-- RETURNING — get the inserted row back (critical for getting auto-generated IDs)
INSERT INTO users (email, password_hash) VALUES ('user@test.com', 'hash')
RETURNING id, created_at;

-- ON CONFLICT — upsert (insert or update if exists)
INSERT INTO cards (card_id, name, price_ebay)
VALUES ('ch-1', 'Charizard', 451.00)
ON CONFLICT (card_id) DO UPDATE SET price_ebay = EXCLUDED.price_ebay;
-- EXCLUDED = the values that were attempted to be inserted
```

### UPDATE
```sql
UPDATE cards
SET trend_label = 'RISING', trending_score = 85, last_updated = NOW()
WHERE card_id = 'ch-1';

-- Update multiple rows
UPDATE alerts SET is_read = TRUE WHERE user_id = 'abc-123';
```

### DELETE
```sql
DELETE FROM alerts WHERE id = 'alert-uuid' AND user_id = 'abc-123';
-- Always include user_id check to prevent deleting other users' data!
```

### JOIN — Combining Tables
```sql
-- INNER JOIN: only rows that match in BOTH tables
SELECT cl.card_name, cl.price, c.price_tcgplayer
FROM card_listings cl
INNER JOIN cards c ON LOWER(cl.card_name) = LOWER(c.name);

-- LEFT JOIN: all rows from left table, matching rows from right (NULL if no match)
SELECT u.email, COUNT(a.id) as alert_count
FROM users u
LEFT JOIN alerts a ON a.user_id = u.id
GROUP BY u.id, u.email;
```

### GROUP BY and Aggregates
```sql
-- COUNT, SUM, AVG, MIN, MAX
SELECT marketplace, COUNT(*) as total, AVG(price) as avg_price
FROM card_listings
WHERE card_name ILIKE '%charizard%'
GROUP BY marketplace
ORDER BY avg_price DESC;
```

---

## 3. Data Types

| Type | Use for | Example |
|---|---|---|
| `UUID` | Primary keys | `gen_random_uuid()` |
| `VARCHAR(n)` | Short strings with max length | names, emails |
| `TEXT` | Unlimited length strings | URLs, descriptions |
| `DECIMAL(10,2)` | Money/prices — exact precision | `450.00` |
| `INTEGER` | Whole numbers | quantity, count |
| `BOOLEAN` | True/false | `is_read DEFAULT FALSE` |
| `TIMESTAMPTZ` | Timestamps WITH timezone | `NOW()` |
| `DATE` | Date only, no time | `deal_date` |

**Why DECIMAL for money, not FLOAT?**
FLOAT has rounding errors (`0.1 + 0.2 = 0.30000000000000004`). DECIMAL is exact. Always use DECIMAL for prices.

**Why TIMESTAMPTZ not TIMESTAMP?**
`TIMESTAMPTZ` stores timezone info. Without it, you get timezone bugs when servers are in different zones.

---

## 4. Primary Keys — UUID vs Integer

```sql
-- Integer (auto-increment) — simple, small, fast
id SERIAL PRIMARY KEY   -- 1, 2, 3, 4...

-- UUID — globally unique, no sequential guessing, FAANG standard

id UUID PRIMARY KEY DEFAULT gen_random_uuid()
-- 'a3f8c2d1-7b4e-4a1f-9c3d-1234567890ab'
```

**Why UUID at FAANG:**
- Can generate IDs in your app before hitting the database
- No sequential guessing (security — users can't guess `id=1001`)
- Works across distributed systems with no coordination

---

## 5. Foreign Keys — Enforcing Relationships

```sql
-- This ensures every alert MUST belong to an existing user
user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE

-- ON DELETE CASCADE: if the user is deleted, ALL their alerts are automatically deleted
-- ON DELETE RESTRICT: prevent deleting a user if they have alerts (PostgreSQL default)
-- ON DELETE SET NULL: set user_id to NULL when user is deleted
```

Without foreign keys, you can end up with "orphaned" data (alerts for users that don't exist).

---

## 6. Indexes — Make Everything Fast

```sql
-- B-tree index (default) — fast for equality and range queries
CREATE INDEX idx_alerts_user_id ON alerts(user_id);
-- Now: WHERE user_id = '...' is instant even with millions of rows

-- Composite index — for queries filtering on multiple columns
CREATE INDEX idx_listings_card_marketplace ON card_listings(card_name, marketplace);
-- Helps: WHERE card_name = 'Charizard' AND marketplace = 'ebay'

-- Partial index — only index rows matching a condition
CREATE INDEX idx_unread_alerts ON alerts(user_id) WHERE is_read = FALSE;
-- This index only contains unread alerts — much smaller and faster for unread queries
```

**When to add an index:**
- Columns you filter on in WHERE clauses
- Foreign key columns (always index these)
- Columns you ORDER BY frequently

**When NOT to add an index:**
- Columns you rarely query
- Tables with very frequent INSERTS (indexes slow down writes)

---

## 7. Triggers — Automatic Database Logic

```sql
-- This function runs automatically when a row is updated
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$   -- $$ starts/ends a code block
BEGIN
    NEW.updated_at = NOW();  -- NEW = the updated row
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Attach the trigger to the users table
CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users          -- runs BEFORE the update is committed
    FOR EACH ROW                    -- runs for every updated row
    EXECUTE FUNCTION update_updated_at_column();
```

Triggers run at the database level — they fire even if someone runs SQL directly, not through the app.

---

## 8. Database Migrations

A migration = a versioned SQL file that changes the schema.

**Rules:**
1. **Never modify an existing migration** that has been run in production
2. **Always create new migration files** for changes: `002_add_index.sql`, `003_add_column.sql`
3. **Make migrations idempotent**: use `CREATE TABLE IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`
4. Migrations run in order — `001_init.sql` → `002_add_column.sql`

This is how Netflix, Stripe, and Uber track schema changes across thousands of servers.

---

## 9. EXPLAIN ANALYZE — Debugging Slow Queries

```sql
EXPLAIN ANALYZE
SELECT * FROM cards WHERE trend_label = 'RISING' ORDER BY trending_score DESC;

-- Output shows:
-- Seq Scan = reading every row (BAD on big tables)
-- Index Scan = using an index (GOOD)
-- Rows = estimated rows processed
-- Actual time = real execution time in ms
```

Any query taking > 100ms needs investigation. `EXPLAIN ANALYZE` shows you WHY.

---

## 10. ON CONFLICT — Upsert Pattern

```sql
-- This is used everywhere in our project:
INSERT INTO price_history (card_id, date, avg_price)
VALUES ('ch-1', '2024-01-15', 450.00)
ON CONFLICT (card_id, date) DO UPDATE
    SET avg_price = EXCLUDED.avg_price;
-- If a row already exists for (card_id, date), update it instead of erroring
```

---

## 11. What to Study Next

1. **Window functions** — `ROW_NUMBER()`, `LAG()`, `LEAD()` for analytics queries
2. **CTEs (Common Table Expressions)** — `WITH cte AS (...)` for complex queries
3. **PostgreSQL JSONB** — storing and querying semi-structured data
4. **Connection pooling** — PgBouncer for managing DB connections at scale
5. **PostgreSQL full-text search** — `tsvector`, `tsquery` — alternative to ILIKE
6. **pg_stat_statements** — find your slowest queries in production
