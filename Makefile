include .env
export 

.PHONY migrate verify test-alert up down

migrate:
		docker exec -i pokemontool_postgres psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) < database/migrations/002_price_alerts.sql

verfiy:
		docker exec it pokemontool_postgres sql -U $(POSTGRES_USER) -d $(POSTGRES_DB) -c "\d price_alerts_settings"

test-alert:
		curl -s -X POST http://localhost:3001/api/price-alerts \
			-H "Content-Type: application/json" \
			-d '{"cardName": "Charizard", "threshold": 80.00, "direction": "BELOW"}'

up: 
		docker-compose up -d

down:
		docker-compose down 