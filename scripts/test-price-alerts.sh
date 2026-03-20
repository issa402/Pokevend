#!/bin/bash

echo "Running Price Alert Scripts"
docker exec -i pokemontool_postgres psql -U pokemontool_user -d pokemontool \
    < database/migration/002_price_alerts.sql

echo "Verifying table"
docker exec -it pokemontool_postgres psql -U 
pokemontool_user -d pokemontool \
    -c "\d price_alerts_settings"

echo "Testing API"
curl -s -X POST http://localhost:3000/api/price-alerts \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"cardName": "Charizard", "threshold": 80, "direction" : "BELOW"}'
echo -e "\n TEST SCRIPT FINISHED"