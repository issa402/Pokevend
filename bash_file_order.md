# Bash / Scripts: The Exact Order You Build Files
## And WHY That Order Is the FAANG Standard

Bash scripts in backend development follow a very specific purpose hierarchy.
You write them in the order they're needed: setup first, then dev utilities, then deployment.

---

## The Mental Model: Environment → Development → Operations

```
System prerequisites (install tools — once per machine)
       ↓
Infrastructure startup (Docker, databases — once per dev session)
       ↓
Application setup (migrations, seeds — once per fresh DB)
       ↓
Development utilities (reset, test data, shortcuts)
       ↓
Deployment scripts (build, push, deploy — once per release)
       ↓
Health checks and monitoring (ongoing operations)
```

---

## Step 1: `scripts/setup-dev.sh` — The First Script You Write
**Sets up everything a new developer needs from scratch.**

```bash
#!/usr/bin/env bash
# ALWAYS the first line — tells the OS which interpreter to use
# /usr/bin/env bash = find bash in the system PATH (more portable than #!/bin/bash)

# SAFETY FLAGS — always put this right after the shebang
set -euo pipefail
# -e: exit immediately if any command fails (non-zero exit code)
# -u: treat unset variables as errors (catches typos in variable names)
# -o pipefail: if any command in a pipe fails, the whole pipe fails
#   Without pipefail: "false | true" exits 0 (success) — misleadingly!

# ── Color output for readability ──────────────────────────────
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'  # No Color — reset to default

log()  { echo -e "${GREEN}[INFO]${NC}  $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC}  $1"; }
err()  { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }
# Functions defined first — Bash reads top-to-bottom, functions must be defined before use

# ── Check prerequisites ────────────────────────────────────────
check_command() {
    local cmd="$1"
    if ! command -v "$cmd" &>/dev/null; then
        err "$cmd is not installed. Install it first: https://..."
    fi
    log "✓ $cmd found"
}

check_command docker
check_command docker-compose
check_command go
check_command python3

# ── Start infrastructure ───────────────────────────────────────
log "Starting Docker infrastructure (PostgreSQL, Redis, RabbitMQ)..."
docker compose up -d postgres redis rabbitmq
# -d = detached mode (runs in background — don't block the terminal)

# ── Wait for PostgreSQL ────────────────────────────────────────
log "Waiting for PostgreSQL to be ready..."
until docker exec pokemontool_postgres pg_isready -U pokemontool_user; do
    warn "PostgreSQL not ready — retrying in 2s..."
    sleep 2  # wait 2 seconds before retrying
done
log "✓ PostgreSQL is ready"

# ── Run migrations ────────────────────────────────────────────
log "Running database migrations..."
docker exec -i pokemontool_postgres psql \
    -U pokemontool_user \
    -d pokemontool \
    < database/migrations/001_init.sql
# -i = interactive (reads from stdin — needed for < redirect)
# < file: redirect file contents to psql's stdin (pipe the SQL file in)

# ── Run seeds ─────────────────────────────────────────────────
log "Loading seed data..."
docker exec -i pokemontool_postgres psql \
    -U pokemontool_user \
    -d pokemontool \
    < database/seeds/pokemon_shows.sql

# ── Python virtualenv setup ────────────────────────────────────
log "Setting up Python virtual environments..."
for service in services/api-consumer services/analytics-engine; do
    pushd "$service" > /dev/null   # cd into directory (saves original dir on stack)
    python3 -m venv venv
    source venv/bin/activate        # activate the virtual environment
    pip install -r requirements.txt --quiet
    deactivate
    popd > /dev/null                # return to original directory
    log "✓ $service dependencies installed"
done

log ""
log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
log "✅ Setup complete! Next steps:"
log "   1. cp .env.example .env"
log "   2. Fill in your API keys in .env"
log "   3. cd server && go run main.go"
log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
```

**Why first?** Every other script assumes the environment is set up.
Migrations assume the DB is running. The app assumes the DB has tables.
setup-dev.sh is the foundation — run it once on a fresh machine, then everything else works.

---

## Step 2: `scripts/migrate.sh` — Idempotent Migration Runner

```bash
#!/usr/bin/env bash
set -euo pipefail

# Run ONE specific migration file passed as argument
# Usage: ./scripts/migrate.sh database/migrations/002_add_column.sql

MIGRATION_FILE="${1:?Usage: migrate.sh <migration-file>}"
# ${1:?message} = use $1 if set, else print message and exit
# This is "mandatory argument" pattern in Bash

if [[ ! -f "$MIGRATION_FILE" ]]; then
    echo "ERROR: Migration file not found: $MIGRATION_FILE"
    exit 1
fi
# [[ -f file ]] = true if file exists as a regular file
# [[ ]] = bash conditional (more powerful than [ ])

echo "Running migration: $MIGRATION_FILE"
docker exec -i pokemontool_postgres psql \
    -U pokemontool_user \
    -d pokemontool \
    < "$MIGRATION_FILE"

echo "✓ Migration complete: $MIGRATION_FILE"
```

**Why separate from setup-dev.sh?** 
After initial setup, you only run NEW migrations — not all of them from scratch.
Splitting into a focused script lets you run: `./scripts/migrate.sh 002_add_column.sql`
without re-running setup tasks.

---

## Step 3: `scripts/reset-db.sh` — Development Database Reset

```bash
#!/usr/bin/env bash
set -euo pipefail

# DANGER: This script DELETES ALL DATA and rebuilds from scratch.
# For development ONLY — never run in production.
# The script asks for confirmation to prevent accidental execution.

echo "⚠️  WARNING: This will DELETE all data in the development database!"
read -p "Type 'yes' to continue: " CONFIRM
# read -p = prompt user for input, store in variable CONFIRM

if [[ "$CONFIRM" != "yes" ]]; then
    echo "Aborted."
    exit 0
fi

echo "Dropping and recreating database..."
docker exec pokemontool_postgres psql -U pokemontool_user -c "
    DROP DATABASE IF EXISTS pokemontool;
    CREATE DATABASE pokemontool;
"

echo "Running migrations..."
docker exec -i pokemontool_postgres psql \
    -U pokemontool_user -d pokemontool \
    < database/migrations/001_init.sql

echo "Loading seeds..."
docker exec -i pokemontool_postgres psql \
    -U pokemontool_user -d pokemontool \
    < database/seeds/pokemon_shows.sql

echo "✓ Database reset complete"
```

**Why this script?** During development, schemas change often.
Instead of manually dropping tables and re-running migrations,
one command resets everything cleanly. Saves hours of debugging dirty state.

**FAANG context:** Every engineer on the team has a way to instantly reset their local DB.
"Works on my machine" bugs often come from dirty local state that doesn't match CI.
reset-db.sh makes local state deterministic.

---

## Step 4: `scripts/deploy.sh` — Production Deployment

```bash
#!/usr/bin/env bash
set -euo pipefail

# Deployment script for EC2 (or any Linux server)
# Usage: ./scripts/deploy.sh
# Assumes: git is set up, docker compose is installed on the server

REMOTE_USER="ubuntu"
REMOTE_HOST="${DEPLOY_HOST:?Set DEPLOY_HOST env var}"
REMOTE_DIR="/opt/pokemontool"

log() { echo "[$(date '+%H:%M:%S')] $1"; }
# $(command) = command substitution — runs command, inserts output
# date '+%H:%M:%S' = current time as "14:23:01"

# ── Build step ─────────────────────────────────────────────────
log "Building Docker images..."
docker compose build

# ── Push step ─────────────────────────────────────────────────
log "Pushing images to registry..."
docker compose push

# ── Deploy step ────────────────────────────────────────────────
log "Deploying to $REMOTE_HOST..."
ssh "$REMOTE_USER@$REMOTE_HOST" bash << 'ENDSSH'
# Everything between << 'ENDSSH' and ENDSSH runs on the REMOTE server via SSH
# The single quotes around ENDSSH prevent local variable expansion (important!)
    set -e
    cd /opt/pokemontool
    git pull origin main
    docker compose pull
    docker compose up -d --build
    echo "✓ Deployment complete"
ENDSSH
# << 'HEREDOC' = "here document" — multiline string literal sent to the previous command
# Used for: multiline SSH commands, generating config files, sending SQL to psql

log "✓ Deployed to $REMOTE_HOST"
```

---

## Step 5: `scripts/health-check.sh` — Operations Monitoring

```bash
#!/usr/bin/env bash
set -euo pipefail

# Check all services are healthy
# Returns exit code 0 (success) or 1 (failure)
# Used by monitoring systems and deployment pipelines

API_URL="${API_URL:-http://localhost:3001}"  # :- = use default if unset

check_endpoint() {
    local name="$1"    # local = scoped to this function (doesn't leak)
    local url="$2"
    local expected="$3"

    # curl flags:
    # -s = silent (no progress bar)
    # -o /dev/null = discard response body
    # -w "%{http_code}" = print only the HTTP status code
    local status
    status=$(curl -s -o /dev/null -w "%{http_code}" "$url")

    if [[ "$status" == "$expected" ]]; then
        echo "✓ $name → HTTP $status"
    else
        echo "✗ $name → HTTP $status (expected $expected)"
        return 1  # non-zero exit from function
    fi
}

echo "=== PokémonTool Health Check ==="
check_endpoint "Go API"     "$API_URL/health"           200
check_endpoint "FastAPI"    "http://localhost:8001/docs" 200
check_endpoint "React UI"   "http://localhost:5173"      200

# Check PostgreSQL
if docker exec pokemontool_postgres pg_isready -U pokemontool_user; then
    echo "✓ PostgreSQL → ready"
else
    echo "✗ PostgreSQL → not ready"
    exit 1
fi

# Check Redis
if docker exec pokemontool_redis redis-cli ping | grep -q PONG; then
    echo "✓ Redis → PONG"
else
    echo "✗ Redis → no response"
    exit 1
fi

echo ""
echo "✅ All services healthy"
```

---

## Complete File Creation Order

```
Phase 1: Development Setup
  1.  scripts/setup-dev.sh         ← full environment setup from scratch

Phase 2: Development Utilities  
  2.  scripts/migrate.sh           ← run a specific migration
  3.  scripts/reset-db.sh          ← wipe and rebuild local DB

Phase 3: Operations
  4.  scripts/deploy.sh            ← build + push + deploy to production
  5.  scripts/health-check.sh      ← verify all services are up
```

---

## Essential Bash Concepts for Backend Engineers

```bash
# ── Variables ─────────────────────────────────────────────────
NAME="pokemontool"
echo "$NAME"          # always quote variables (prevents word splitting)
echo "${NAME}_db"     # use {} to separate variable from surrounding text

# ── Conditionals ──────────────────────────────────────────────
if [[ -f "file.txt" ]]; then echo "file exists"; fi
if [[ -d "dir" ]]; then echo "directory exists"; fi
if [[ -z "$VAR" ]]; then echo "VAR is empty"; fi
if [[ -n "$VAR" ]]; then echo "VAR is not empty"; fi
if [[ "$A" == "$B" ]]; then echo "strings equal"; fi
if [[ $NUM -gt 10 ]]; then echo "greater than 10"; fi

# ── Loops ─────────────────────────────────────────────────────
for file in *.sql; do echo "Processing: $file"; done
for i in {1..5}; do echo "$i"; done
while true; do sleep 5; echo "still running"; done

# ── Functions ─────────────────────────────────────────────────
my_function() {
    local arg1="$1"   # $1, $2, $3 = positional arguments
    local arg2="$2"
    echo "Got: $arg1 $arg2"
    return 0          # explicit success (0 = success, non-zero = failure)
}
my_function "hello" "world"

# ── Exit codes ────────────────────────────────────────────────
command && echo "success"         # && = run second if first succeeds
command || echo "failed"          # || = run second if first fails
command || { echo "fail"; exit 1; }  # {} groups commands (note the semicolons)

# ── Command substitution and arithmetic ──────────────────────
DATE=$(date +%Y-%m-%d)            # capture command output
COUNT=$((5 + 3))                  # arithmetic (not floating point)
FILES=$(ls *.go | wc -l)          # count Go files

# ── Redirects ─────────────────────────────────────────────────
command > file.txt      # redirect stdout to file (overwrite)
command >> file.txt     # redirect stdout to file (append)
command 2>&1            # redirect stderr to stdout (combine)
command < file.txt      # redirect file to stdin
command &>/dev/null     # discard ALL output (stdout + stderr)

# ── Pipes ─────────────────────────────────────────────────────
docker logs container | grep ERROR          # filter logs
cat file.txt | sort | uniq | wc -l         # pipeline: sort → dedup → count
ps aux | grep go | awk '{print $2}'        # find Go PIDs

# ── Heredoc (multiline strings) ───────────────────────────────
cat << EOF > config.yaml
database:
  host: $DB_HOST     # variables ARE expanded with unquoted EOF
  port: 5432
EOF

cat << 'EOF' > script.sh
echo "$NOT_EXPANDED"  # variables are NOT expanded with quoted 'EOF'
EOF

# ── set -euo pipefail (the safety net) ───────────────────────
# Always start scripts with these. They catch:
# -e: command failures (non-zero exit)
# -u: undefined variables (typos)
# -o pipefail: pipeline failures (piped command failed)

# ── Useful tools ──────────────────────────────────────────────
curl -s http://localhost:3001/health | python3 -m json.tool  # pretty-print JSON
jq '.status' response.json         # parse JSON with jq
grep -r "TODO" .                    # recursive search
sed -i 's/old/new/g' file.txt      # in-place string replacement
awk '{print $1}' file.txt          # print first column
xargs: reads stdin and builds argument lists
  cat cards.txt | xargs -I{} curl http://api/{}/price    # one request per line
```

---

## The Rule Behind the Order

```
setup-dev.sh     ← used once (new engineer onboarding)
migrate.sh       ← used when schema changes (daily during dev)
reset-db.sh      ← used when local state is dirty (weekly)
deploy.sh        ← used when releasing to production (per-PR or daily)
health-check.sh  ← used to verify deployments (always after deploy)
```

Each script has ONE job. A script that does setup AND deploys AND runs health checks
is hard to reason about and breaks in unexpected ways.
Single-responsibility applies to scripts too.
