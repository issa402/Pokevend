#!/bin/bash
set -e

#lsof -i :3001 to see PID and then kill -9 PID to kill 
SCRIPTS_DIR=$(cd -- "$(dirname -- "$0")" && pwd) #since the parnethis where dirname an d0 is in is inisde the main parnethesis that runs first the $ mean use that value. The $0 means the script name and the -- means for type safety so that the name isnt cofnused
PROJECT_ROOT=$(cd -- "$SCRIPTS_DIR/.." && pwd)
LOG_DIR="$SCRIPTS_DIR/logs/$(date +%Y-%m-%dT%H:%M:%S)"
PYTHON_DIR="$PROJECT_ROOT/services/api-consumer"
GO_DIR="$PROJECT_ROOT/server"

# Create logs directory if it doesn't exist
mkdir -p "$LOG_DIR"

echo "🚀 STARTING POKEMON TOOL..."

# 1. INFRASTRUCTURE (Postgres, Redis, Rabbit)
cd "$PROJECT_ROOT"

ensure_container() {
    local service_name="$1"
    local container_name="$2"

    local running_id
    local existing_id

    running_id=$(docker ps -q -f "name=^${container_name}$") #-q means quelitly -f means file name and we have name inside parenthesis for that value. The $ in the begginign returns the value. We call it name so that if the conatiner has spaces it wont break and it filters out the ID wher ethe name matches.  The ^ means the name must start exactly here and the $ at the end means the name must end excalty here
    existing_id=$(docker ps -aq -f "name=^${container_name}$")

    if [ -n "$running_id" ]; then
        echo "✓ $container_name is already running"
    elif [ -n "$existing_id" ]; then
        echo "↻ $container_name exists but is stopped. Starting it..."
        docker start "$container_name"
    else
        echo "➕ $container_name does not exist. Creating it with docker compose..."
        docker compose up -d "$service_name"
    fi
}

ensure_container "postgres" "pokemontool_postgres"
ensure_container "redis" "pokemontool_redis"
ensure_container "rabbitmq" "pokemontool_rabbitmq"

echo "✓ Docker containers readying..."
sleep 10


# 3. GO (server)
echo "✓ Starting GO API on :3001..."
cd "$GO_DIR"
go run main.go > "$LOG_DIR/go.log" 2>&1 &
GO_PID=$!
cd "$PROJECT_ROOT"


# 2. PYTHON (api-consumer)
echo "📦 Preparing Python Environment..."
cd "$PYTHON_DIR"

if [ ! -d "venv" ]; then # if venv not in the directory
    python3 -m venv venv
    ./venv/bin/pip install -r requirements.txt
fi


./venv/bin/python -m uvicorn main:app --port 8001 > "$LOG_DIR/python.log" 2>&1 &     # the 2>&1 means 2 means Errors and 1 is normal output . So sending both errors and logs ouputs. The & means run the command in the background and move to the next line
PYTHON_PID=$! # Stores the PID so it can be killed later
cd "$PROJECT_ROOT"

echo "Waiting for Go server and Python Server to start"
sleep 10

# 4. HEALTH CHECK
echo "🔍 Running Health Checks..."
"$SCRIPTS_DIR/health-check.sh"

echo -e "\n🔥 ALL SYSTEMS LIVE!"
echo "View Python logs: tail -f $LOG_DIR/python.log"
echo "View Go logs:     tail -f $LOG_DIR/go.log"

trap 'kill "$PYTHON_PID" "$GO_PID" 2>/dev/null; exit' SIGINT SIGTERM  # trap means if any interupting signal like ctrl c you must stop. The kill commands sends a shutdown command to the specific ID numbers of the python and GO. 2>/dev/null means the errors should be not be returned . exit means the script can die. SIGINT is what happens when you click ctrl c and SIGTERM means Pleas stop signal from the compyer
sleep 10
GO_PID=$(lsof -t -i :3001)   
PYTHON_PID=$(lsof -t -i :8001)  
if [ "$GO_PID" ]; then
    kill -9 "$GO_PID"
else
    echo "NO SUCH PROCESS RUNNING"
fi
if [ "$PYTHON_PID" ]; then
    kill -9 "$PYTHON_PID"
else
    echo "NO SUCH PROCESS RUNNING"
fi
echo "SYSTEM GOOD" #SINCE WE USED the & at the end in line 56 and 63 to keep both instances running and move to the next line so wait tell it to stay open as long as the bacjgroudn proces are running
