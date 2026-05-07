#!/usr/bin/env bash
# ============================================================
# FILE: scripts/health-check.sh
# TYPE: Operations Script — Service Health Verification
#
# WHAT IS THIS?
# Checks that all PokémonTool services are alive and responding.
# Run after deployment or when something seems wrong.
#
# PRACTICE TASK #2 from practice_tasks.md
# Implement: check_endpoint(), PostgreSQL check, Redis check
# Solution (commented out) is at the bottom.
# ============================================================

set -euo pipefail

# ── YOUR IMPLEMENTATION GOES HERE ────────────────────────────
# What to implement:
#
# 1. A function: check_endpoint <name> <url> <expected_http_code>
#    - Use: curl -s -o /dev/null -w "%{http_code}" "$url"
#    - Print: ✓ <name> → HTTP <code>   (if matches expected)
#    - Print: ✗ <name> → HTTP <code>   (if not, then: exit 1)
#
# 2. Call check_endpoint for:
#    check_endpoint "Go API"   "http://localhost:3001/health" 200
#    check_endpoint "FastAPI"  "http://localhost:8001/docs"   200
#
# 3. PostgreSQL check:
#    docker exec pokemontool_postgres pg_isready -U pokemontool_user
#    → print ✓ or ✗
#
# 4. Redis check:
#    docker exec pokemontool_redis redis-cli ping | grep -q PONG
#    → print ✓ or ✗
#
# 5. Print final "✅ All services healthy" or exit 1
GREEN='\033[0;32m'; RED='\033[0;31m'; NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="$PROJECT_ROOT/docker-compose.yml"

check_endpoint() {
    local name="$1"
    local url="$2"
    local expected="$3"
    local status

    status=$(docker exec pokemontool_server wget -q -S -O /dev/null "$url" 2>&1 | awk '/HTTP\// {code=$2} END {print code}')
    status="${status:-000}"

    if [[ "$status" == "$expected" ]]; then
        echo -e "${GREEN}✓${NC} $name → HTTP $status"
    else
        echo -e "${RED}✗${NC} $name → HTTP $status (Expected $expected)"
        exit 1
    fi
}



# ── RUNNING THE CHECKS ──
# (Make sure your Go and Python servers are running before typing this!)

# Call the function (No parentheses like Go/Python!)
check_endpoint "Go API" "http://server:3001/health" 200
check_endpoint "FastAPI" "http://api-consumer:8001/health" 200

# PostgreSQL: We tell Docker to run 'pg_isready' inside the container
echo -n "Checking Postgres... "
if docker exec pokemontool_postgres pg_isready -U pokemontool_user > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Ready${NC}"
else
    echo -e "${RED}✗ Down${NC}"
    exit 1
fi

# Redis: We send a 'PING' and expect a 'PONG'
echo -n "Checking Redis... "
if docker exec pokemontool_redis redis-cli ping | grep -q PONG; then
    echo -e "${GREEN}✓ Ready${NC}"
else
    echo -e "${RED}✗ Down${NC}"
    exit 1
fi


TODAY="$(date +%F)"
echo -n "Checking rabbitmq... "
if docker compose -f "$COMPOSE_FILE" logs --since "${TODAY}T00:00:00" rabbitmq 2>/dev/null | grep -q "authenticated and granted access"; then
    echo -e "${GREEN}✓ Authenticated today${NC}"
else
    echo -e "${RED}✗ Down${NC}"
    exit 1
fi

echo -e "\n${GREEN}✅ ALL SERVICES HEALTHY${NC}"








# TODO: Write your implementation below (delete this comment)


# ============================================================
# SOLUTION (peek only when stuck):
# ============================================================
# GREEN='\033[0;32m'; RED='\033[0;31m'; NC='\033[0m'
#
# check_endpoint() {
#     local name="$1"
#     local url="$2"
#     local expected="$3"
#     local status
#     status=$(curl -s -o /dev/null -w "%{http_code}" "$url" 2>/dev/null || echo "000")
#     if [[ "$status" == "$expected" ]]; then
#         echo -e "${GREEN}✓${NC} $name → HTTP $status"
#     else
#         echo -e "${RED}✗${NC} $name → HTTP $status (expected $expected)"
#         return 1
#     fi
# }
#
# echo "=== PokémonTool Health Check ==="
#
# check_endpoint "Go API"  "http://localhost:3001/health" 200
# check_endpoint "FastAPI" "http://localhost:8001/docs"   200
#
# if docker exec pokemontool_postgres pg_isready -U pokemontool_user &>/dev/null; then
#     echo -e "${GREEN}✓${NC} PostgreSQL → ready"
# else
#     echo -e "${RED}✗${NC} PostgreSQL → not ready"; exit 1
# fi
#
# if docker exec pokemontool_redis redis-cli ping 2>/dev/null | grep -q PONG; then
#     echo -e "${GREEN}✓${NC} Redis → PONG"
# else
#     echo -e "${RED}✗${NC} Redis → no response"; exit 1
# fi
#
# echo ""
# echo "✅ All services healthy"
