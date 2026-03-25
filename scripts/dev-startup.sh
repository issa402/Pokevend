#!/bin/bash
set -e

# Create logs directory if it doesn't exist (crucial so the redirects don't fail)
mkdir -p logs

echo "🚀 STARTING POKEMON TOOL..."

# 1. INFRASTRUCTURE (Postgres, Redis, Rabbit)
docker-compose up -d
echo "✓ Docker containers starting..."
sleep 10

# 2. PYTHON (api-consumer)
echo "📦 Preparing Python Environment..."
# Go into the python folder
cd services/api-consumer
if [ ! -d "venv" ]; then
    python3 -m venv venv
    ./venv/bin/pip install fastapi uvicorn pika aio-pika python-dotenv
fi

# Start Python in background (send logs back up to the root logs folder)
./venv/bin/python -m uvicorn main:app --port 8001 > ../../logs/python.log 2>&1 &
PYTHON_PID=$!
cd ../.. # Back to root

# 3. GO (server)
echo "✓ Starting GO API on :3001..."
cd server
# Start Go in background
go run main.go > ../logs/go.log 2>&1 &
GO_PID=$!
cd .. # Back to root

# 4. HEALTH CHECK
echo "🔍 Running Health Checks..."
./scripts/health-check.sh 

echo -e "\n🔥 ALL SYSTEMS LIVE!"
echo "View Python logs: tail -f logs/python.log"
echo "View Go logs:     tail -f logs/go.log"

# This kills both background servers when you hit Ctrl+C
trap "kill $PYTHON_PID $GO_PID; exit" SIGINT SIGTERM
wait
