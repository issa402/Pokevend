ALTER TABLE watchlists
    ADD COLUMN IF NOT EXISTS language_preference VARCHAR(20) DEFAULT 'BOTH';

UPDATE watchlists
   SET language_preference = 'BOTH'
 WHERE language_preference IS NULL OR language_preference = '';
