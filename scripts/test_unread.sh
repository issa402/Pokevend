#!/bin/bash
set -eou pipefail

docekr exec -i pokemontool_postgres psql -U pokemontool_user -d pokemontool \
-c  "CREATE INDEX IF NOT EXISTS idx_alerts_user_unread ON alerts(user_id, is_read) WHERE is_read = false;"

TOKEN = $(curl -s -X POST http://localhost:3000/api/auth/login \
    -H "Content-Type: application/json" \
    -d '{"email": "test@test.com", "password": "yourpassword"}' | python3 -c "import sys, json;print(json.load(sys.stdin)['token'])")

curl -s hhtp://localhost:3001/api/alerts/unread-count \
    -H "Authorization : Bearer $TOKEN"
    