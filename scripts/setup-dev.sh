#!/bin/bash
# ============================================================
# FILE: scripts/setup-dev.sh
# TYPE: Bash Setup Script
#
# WHAT IS THIS?
# A shell script that automates setting up the development environment.
# Instead of a 20-step README, new team members run ONE command:
#   bash scripts/setup-dev.sh
#
# FAANG PRACTICE: "Zero-to-running in one command."
# Every major tech company has a setup script like this. It:
#   1. Checks prerequisites (are the right tools installed?)
#   2. Creates config from a template (.env from .env.example)
#   3. Installs language dependencies (Node packages, Python venv)
#   4. Starts infrastructure (Docker containers)
#   5. Runs database migrations
#
# HOW TO RUN IT:
#   bash scripts/setup-dev.sh
# or make it executable first:
#   chmod +x scripts/setup-dev.sh
#   ./scripts/setup-dev.sh
#
# BASH CONCEPTS DEMONSTRATED:
#   #!/bin/bash, set -e, functions, $1 arguments, command -v,
#   [[ -f file ]] file tests, $() command substitution, local variables
# ============================================================

# ── set -e ───────────────────────────────────────────────────
# "Exit Immediately on Error"
# Without this: if "npm install" fails, script keeps running and breaks things silently.
# With this: any failed command immediately stops the script with an error.
# ALWAYS use set -e in production scripts.
# ─────────────────────────────────────────────────────────────
set -e

echo "🎮 PokémonTool — Development Setup"
echo "===================================="

# ── Prerequisite Checking ─────────────────────────────────────
# Before doing anything, verify all required tools are installed.
# Fail early with a clear message — saves hours of mysterious errors.
echo "Checking prerequisites..."

# A reusable function to check if a CLI command exists.
# BASH SYNTAX:
#   check_cmd()  = define a function called check_cmd
#   "$1"         = the first argument passed to this function
#   command -v   = checks if a command exists (returns 0=found, 1=not found)
#   &>/dev/null  = redirect both stdout AND stderr to /dev/null (silence output)
#   !            = negate the result (if NOT found...)

check_cmd() {
  if ! command -v "$1" &> /dev/null; then
    echo "❌ $1 is not installed. Please install it and re-run."
    exit 1  # Exit with error code 1 (non-zero = failure in bash)
  fi
  echo "✓ $1 found"
}

# Check each required tool — script stops at the first missing one
check_cmd node       # Node.js runtime (for React client dev)
check_cmd npm        # Node package manager
check_cmd python3    # Python 3 (for api-consumer and analytics-engine)
check_cmd docker     # Docker (for PostgreSQL, Redis, RabbitMQ)
check_cmd docker-compose

# ── Version Check ─────────────────────────────────────────────
# Not just "is it installed" but "is it the right version?"
# node -v = prints "v21.6.1"
# cut -d. -f1 = split by "." and take field 1 = "v21"
# tr -d 'v' = delete the 'v' character = "21"
NODE_VER=$(node -v | cut -d. -f1 | tr -d 'v')
if [ "$NODE_VER" -lt 18 ]; then
  echo "❌ Node.js 18+ is required. You have $(node -v)"
  exit 1
fi

# ── Project Root Detection ────────────────────────────────────
# Find the project root directory regardless of where the script is called from.
# BASH_SOURCE[0] = path to this script file
# dirname       = get the directory of that path
# cd ..         = go up one level (from scripts/ to project root)
# pwd           = print current directory (absolute path)
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}") /..)" && pwd)"
echo ""
echo "Project root: $ROOT"

# ── .env Setup ────────────────────────────────────────────────
# The .env file holds secrets (database passwords, API keys).
# It's in .gitignore — never committed to git.
# .env.example is committed — it's a template with placeholder values.
#
# [ ! -f "$ROOT/.env" ] = if the file does NOT exist
# cp = copy
# -f = test for file existence
if [ ! -f "$ROOT/.env" ]; then
  echo "Creating .env from .env.example..."
  cp "$ROOT/.env.example" "$ROOT/.env"
  echo "⚠️  IMPORTANT: Edit .env and add your API keys before starting services!"
else
  echo "✓ .env already exists"
fi

# ── Node.js Dependency Installation ───────────────────────────
# npm install reads package.json and downloads all listed packages
# to the node_modules/ directory.
# cd changes directory to server/, runs npm install there.
echo ""
echo "Installing Node.js server dependencies..."
cd "$ROOT/server" && npm install
echo "✓ Server dependencies installed"

echo "Installing React frontend dependencies..."
cd "$ROOT/client" && npm install
echo "✓ Client dependencies installed"

# ── Python Virtual Environments ───────────────────────────────
# A virtual environment (venv) is an isolated Python environment.
# Each service gets its OWN venv so their dependencies don't conflict.
# Example: api-consumer needs httpx 0.27, analytics-engine needs requests 2.31
# With separate venvs, both work independently.
#
# python3 -m venv .venv  = create virtual environment in .venv folder
# source .venv/bin/activate = activate it (Linux/Mac)
# . .venv/Scripts/activate  = activate it (Windows, via Git Bash)
# pip install -r requirements.txt = install all packages listed in requirements.txt
# deactivate = leave the virtual environment
echo ""
echo "Setting up Python virtual environments..."

# Reusable function to set up a Python venv for a service
# $1 = SERVICE_DIR (first argument), $2 = SERVICE_NAME (second argument)

setup_venv() {
  local SERVICE_DIR="$1"   # local = variable only exists inside this function
  local SERVICE_NAME="$2"
  echo "  → $SERVICE_NAME..."
  cd "$SERVICE_DIR"
  python3 -m venv .venv

  # Try Linux/Mac activation first, fall back to Windows (Git Bash)
  # 2>/dev/null = silently ignore errors (if one activation path doesn't exist)
  source .venv/bin/activate 2>/dev/null || . .venv/Scripts/activate 2>/dev/null

  pip install --quiet -r requirements.txt  # --quiet = less output
  deactivate  # exit the venv after installing
}

setup_venv "$ROOT/services/api-consumer"      "api-consumer"
setup_venv "$ROOT/services/analytics-engine"  "analytics-engine"
echo "✓ Python environments ready"

# ── Start Docker Infrastructure ───────────────────────────────
# Only start the infrastructure containers (Postgres, Redis, RabbitMQ)
# not the application containers — we run those directly during dev.
#
# docker-compose up -d = start containers in "detached" mode (background)
# Without -d, Docker fills your terminal with logs and you can't type.
echo ""
echo "Starting infrastructure (PostgreSQL, Redis, RabbitMQ)..."
cd "$ROOT"
docker-compose up -d postgres redis rabbitmq

# Sleep while PostgreSQL initializes. Postgres takes a few seconds to
# start accepting connections after the container starts.
echo "Waiting 10 seconds for databases to initialize..."
sleep 10

# ── Database Migrations ───────────────────────────────────────
# Apply the SQL schema to the fresh PostgreSQL container.
#
# docker exec -i = run a command inside an existing container
#   -i = "interactive" (allows stdin so we can pipe SQL)
# pokemontool-postgres = the container name (from docker-compose.yml)
# psql = PostgreSQL command-line client
#   -U pokemontool_user = connect as this user
#   -d pokemontool      = use this database
# < "file.sql" = feed the SQL file as stdin to psql
echo ""
echo "Running database migrations..."
docker exec -i pokemontool-postgres psql -U pokemontool_user -d pokemontool \
  < "$ROOT/database/migrations/001_init.sql" && echo "✓ Migrations applied"

docker exec -i pokemontool-postgres psql -U pokemontool_user -d pokemontool \
  < "$ROOT/database/seeds/pokemon_shows.sql"  && echo "✓ Seed data loaded"

# ── Final Instructions ────────────────────────────────────────
# Always tell the developer what to do next.
# This removes confusion after setup.
echo ""
echo "============================================"
echo "✅ Setup complete! To start development:"
echo ""
echo "   Terminal 1 (Go backend): cd server && go run main.go"
echo "   Terminal 2 (frontend):   cd client && npm run dev"
echo "   Terminal 3 (Python):     cd services/api-consumer && uvicorn main:app --reload"
echo ""
echo "🌍 Dashboard: http://localhost:5173"
echo "🔌 Go API:    http://localhost:3001"
echo "📊 FastAPI:   http://localhost:8001/docs"
echo "============================================"

# ============================================================
# TODO #1 (Practice): Add Go installation check
# The Go binary (go) needs to be installed for local development.
# Add a check_cmd go line in the prerequisites section.
# Then add a go version check similar to the Node.js version check:
#   go version outputs "go version go1.22.0 windows/amd64"
# Use cut and tr to extract the version number and verify it's >= 1.21
# HINT: go version | awk '{print $3}' | tr -d 'go' extracts "1.22.0"
# ============================================================

# ============================================================
# TODO #2 (Practice): Add a health check after startup
# After running migrations, verify the server actually works.
# Use curl to hit the health endpoint and check the response.
# HINT: curl -s http://localhost:3001/health
# -s = silent (no progress bar)
# If curl fails (server not up), print an error message and exit 1.
# BONUS: wait up to 30 seconds for the server to be ready (retry loop)
# ============================================================
