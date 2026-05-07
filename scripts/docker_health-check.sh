#!/usr/bin/env bash
# ============================================================
# FILE: scripts/docker_health-check.sh
# TYPE: Operations Script — Docker-Only Service Health Verification
# ============================================================

set -euo pipefail

GREEN='\033[0;32m'; RED='\033[0;31m'; NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="$PROJECT_ROOT/docker-compose.yml"

echo "=== PokemonTool Docker Health Check ==="

echo -n "Checking Go API... "
if docker exec pokemontool_server wget -q -O - http://127.0.0.1:3001/health 2>/dev/null | grep -q '"status":"ok"'; then
    echo -e "${GREEN}✓ Ready${NC}"
else
    echo -e "${RED}✗ Down${NC}"
    exit 1
fi

echo -n "Checking FastAPI... "
if docker exec pokemontool_api_consumer python -c "import urllib.request; print(urllib.request.urlopen('http://127.0.0.1:8001/health', timeout=5).status)" 2>/dev/null | grep -q '^200$'; then
    echo -e "${GREEN}✓ Ready${NC}"
else
    echo -e "${RED}✗ Down${NC}"
    exit 1
fi

echo -n "Checking Postgres... "
if docker exec pokemontool_postgres pg_isready -U pokemontool_user > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Ready${NC}"
else
    echo -e "${RED}✗ Down${NC}"
    exit 1
fi

echo -n "Checking Redis... "
if docker exec pokemontool_redis redis-cli ping 2>/dev/null | grep -q PONG; then
    echo -e "${GREEN}✓ Ready${NC}"
else
    echo -e "${RED}✗ Down${NC}"
    exit 1
fi

echo -n "Checking RabbitMQ... "
TODAY="$(date +%F)"
if docker compose -f "$COMPOSE_FILE" logs --since "${TODAY}T00:00:00" rabbitmq 2>/dev/null | grep -q "authenticated and granted access"; then
    echo -e "${GREEN}✓ Authenticated today${NC}"
else
    echo -e "${RED}✗ Down${NC}"
    exit 1
fi

echo -e "\n${GREEN}✅ ALL SERVICES HEALTHY${NC}"
