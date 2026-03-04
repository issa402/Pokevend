#!/bin/bash
# ============================================================
# PokémonTool — Development Setup Script
# Run this once after cloning to set up your dev environment.
# Usage: bash scripts/setup-dev.sh
# ============================================================

set -e  # Exit immediately on any error

echo "🎮 PokémonTool — Development Setup"
echo "===================================="

# ---- Check prerequisites ----
echo "Checking prerequisites..."

check_cmd() {
  if ! command -v "$1" &> /dev/null; then
    echo "❌ $1 is not installed. Please install it and re-run."
    exit 1
  fi
  echo "✓ $1 found"
}

check_cmd node
check_cmd npm
check_cmd python3
check_cmd docker
check_cmd docker-compose

NODE_VER=$(node -v | cut -d. -f1 | tr -d 'v')
if [ "$NODE_VER" -lt 18 ]; then
  echo "❌ Node.js 18+ is required. You have $(node -v)"
  exit 1
fi

# ---- Set up environment ----
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
echo ""
echo "Project root: $ROOT"

if [ ! -f "$ROOT/.env" ]; then
  echo "Creating .env from .env.example..."
  cp "$ROOT/.env.example" "$ROOT/.env"
  echo "⚠️  IMPORTANT: Edit .env and add your API keys before starting services!"
else
  echo "✓ .env already exists"
fi

# ---- Install Node.js dependencies ----
echo ""
echo "Installing Node.js server dependencies..."
cd "$ROOT/server" && npm install
echo "✓ Server dependencies installed"

echo "Installing React frontend dependencies..."
cd "$ROOT/client" && npm install
echo "✓ Client dependencies installed"

# ---- Install Python dependencies ----
echo ""
echo "Setting up Python virtual environments..."

setup_venv() {
  local SERVICE_DIR="$1"
  local SERVICE_NAME="$2"
  echo "  → $SERVICE_NAME..."
  cd "$SERVICE_DIR"
  python3 -m venv .venv
  source .venv/bin/activate 2>/dev/null || . .venv/Scripts/activate 2>/dev/null
  pip install --quiet -r requirements.txt
  deactivate
}

setup_venv "$ROOT/services/api-consumer"      "api-consumer"
setup_venv "$ROOT/services/analytics-engine"  "analytics-engine"
echo "✓ Python environments ready"

# ---- Start infrastructure via Docker ----
echo ""
echo "Starting infrastructure (PostgreSQL, MongoDB, Redis, RabbitMQ)..."
cd "$ROOT"
docker-compose up -d postgres mongo redis rabbitmq
echo "Waiting 10 seconds for databases to initialize..."
sleep 10

# ---- Run database migrations ----
echo ""
echo "Running database migrations..."
docker exec -i pokemontool-postgres psql -U pokemontool_user -d pokemontool < "$ROOT/database/migrations/001_init.sql" && echo "✓ Migrations applied"
docker exec -i pokemontool-postgres psql -U pokemontool_user -d pokemontool < "$ROOT/database/seeds/pokemon_shows.sql"  && echo "✓ Seed data loaded"

echo ""
echo "============================================"
echo "✅ Setup complete! To start development:"
echo ""
echo "   Terminal 1 (backend):  cd server && npm run dev"
echo "   Terminal 2 (frontend): cd client && npm run dev"
echo "   Terminal 3 (Python):   cd services/api-consumer && python main.py"
echo ""
echo "🌍 Dashboard: http://localhost:5173"
echo "🔌 API:       http://localhost:3001"
echo "============================================"
