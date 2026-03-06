# Bash & Linux Engineering Guide
## Scripts, DevOps, and Linux — What Every Backend Engineer Must Know

---

## 1. Why Bash Matters at FAANG

Every backend engineer runs servers on Linux. Bash is how you:
- Automate setup and deployment
- Manage Docker containers
- Debug production issues
- Run database migrations
- Write CI/CD pipeline steps (GitHub Actions, Jenkins)

You can't google everything when production is down at 3am. Know bash.

---

## 2. Bash Script Fundamentals

### The Shebang
```bash
#!/bin/bash
# First line of every bash script.
# '#!' = shebang — tells the OS which interpreter to use
# '/bin/bash' = use the bash shell (not sh, not zsh)
```

### `set -e` — Exit on Error (Critical)
```bash
set -e
# Without this: if a command fails, the script KEEPS RUNNING
# With this: script immediately exits on any failed command
# ALWAYS use this in production scripts — prevents partial setup states

set -euo pipefail  # The full version used at FAANG:
# -e = exit on error
# -u = error on undefined variables
# -o pipefail = if any command in a pipe fails, fail the whole pipe
```

### Variables
```bash
# Assignment (no spaces around =)
NAME="PokémonTool"
PORT=3001
IS_PROD=true

# Use variables with $
echo "Starting $NAME on port $PORT"

# Command substitution — store command output in a variable
NODE_VER=$(node -v)          # $(command) = run command, capture output
DATE=$(date '+%Y-%m-%d')     # current date

# Default values — if var is empty, use a default
PORT=${PORT:-3001}           # use $PORT if set, else use 3001
```

### Conditionals
```bash
if [ "$PORT" -lt 1024 ]; then     # -lt = less than (for numbers)
    echo "Port must be >= 1024"
    exit 1
fi

if [ -f ".env" ]; then            # -f = file exists
    echo ".env found"
elif [ -d "config" ]; then        # -d = directory exists
    echo "config dir found"
else
    echo "nothing found"
fi

# Check command exists
if command -v docker &> /dev/null; then
    echo "Docker installed"
fi
# &>/dev/null = redirect both stdout and stderr to /dev/null (silence output)
```

### Functions
```bash
# Define a function
check_cmd() {
    local cmd="$1"    # $1 = first argument to function, local = scoped to function
    if ! command -v "$cmd" &> /dev/null; then
        echo "❌ $cmd is not installed"
        exit 1
    fi
    echo "✓ $cmd found"
}

# Call the function
check_cmd docker
check_cmd node
check_cmd python3
```

### Loops
```bash
# For loop over a list
for SERVICE in api-consumer analytics-engine scraping-service; do
    echo "Setting up $SERVICE..."
    cd "services/$SERVICE"
    python3 -m venv .venv
    cd ../..
done

# While loop
ATTEMPT=0
while [ $ATTEMPT -lt 5 ]; do
    if ping -c 1 localhost &>/dev/null; then
        echo "Server is up!"
        break
    fi
    echo "Waiting... attempt $ATTEMPT"
    sleep 2
    ATTEMPT=$((ATTEMPT + 1))    # arithmetic: $((expression))
done
```

---

## 3. Essential Linux Commands

### File System Navigation
```bash
pwd                     # Print working directory (where am I?)
ls -la                  # List all files with permissions and sizes
cd /path/to/dir         # Change directory
cd ..                   # Go up one level
cd -                    # Go back to previous directory
mkdir -p path/to/dir    # Create directory and all parents
cp -r source/ dest/     # Copy directory recursively
mv old-name new-name    # Move or rename
rm -rf directory/       # Delete directory (CAREFUL — no undo!)

# Find files
find . -name "*.go"     # Find all Go files from current directory
find . -name "*.py" -not -path "./.venv/*"   # Exclude .venv
```

### Reading Files
```bash
cat file.txt            # Print entire file
head -20 file.txt       # First 20 lines
tail -50 file.txt       # Last 50 lines
tail -f server.log      # Follow log file in real-time (Ctrl+C to stop)
grep "ERROR" server.log # Find lines containing "ERROR"
grep -r "card_name" .   # Recursive search through all files
```

### Pipes and Redirection
```bash
# | (pipe) = send output of left command as input to right command
docker ps | grep pokemon          # filter docker output
cat migrations.sql | psql -U user # feed SQL file to psql

# > redirect output to file (overwrites)
echo "hello" > output.txt

# >> append to file
echo "hello again" >> output.txt

# 2>&1 = redirect stderr to stdout (capture errors too)
node server.js > server.log 2>&1
```

### Process Management
```bash
ps aux                  # List all running processes
ps aux | grep node      # Find Node.js processes
kill 1234               # Gracefully stop process with PID 1234
kill -9 1234            # Force kill (SIGKILL) — use as last resort

# Background processes
node server.js &        # Run in background (returns PID)
jobs                    # List background jobs
fg                      # Bring background job to foreground

# nohup — keep running after you disconnect SSH
nohup node server.js > server.log 2>&1 &
```

### Environment Variables
```bash
export PORT=3001        # Set variable for this shell AND child processes
echo $PORT              # Read variable

# Load .env file into current shell
export $(cat .env | grep -v '^#' | xargs)
# grep -v '^#' = skip comment lines
# xargs = convert lines to key=value pairs
```

---

## 4. Docker Commands You Must Know

```bash
# Build an image
docker build -t pokemontool-server .           # . = use current directory Dockerfile
docker build -t pokemontool-server:v1.2 .      # with version tag

# Run a container
docker run -d -p 3001:3001 --name api pokemontool-server
# -d = detached (background)
# -p host:container = port mapping
# --name = container name

# View running containers
docker ps
docker ps -a    # all containers including stopped

# View logs
docker logs pokemontool_server -f    # -f = follow (live tail)

# Execute command inside running container
docker exec -it pokemontool_postgres bash   # open shell inside postgres container
docker exec -i pokemontool_postgres psql -U pokemontool_user -d pokemontool < migration.sql

# Stop and remove
docker stop pokemontool_server
docker rm pokemontool_server

# Docker Compose
docker compose up -d postgres redis             # start specific services
docker compose down                             # stop all services
docker compose down -v                          # stop + delete volumes (wipes database!)
docker compose logs -f server                   # follow server logs
docker compose ps                               # status of all services
```

---

## 5. SSH — Connecting to EC2

```bash
# Connect to your EC2 instance
ssh -i path/to/key.pem ubuntu@<EC2-IP>

# Copy files to EC2 (SCP = secure copy)
scp -i key.pem -r ./Pokemon ubuntu@<EC2-IP>:~/pokemontool

# Run a command on remote server without staying connected
ssh -i key.pem ubuntu@<EC2-IP> "cd ~/pokemontool && docker compose ps"

# Port forwarding — access EC2's localhost:3001 as your localhost:3001
ssh -i key.pem -L 3001:localhost:3001 ubuntu@<EC2-IP>
```

---

## 6. What to Study Next

1. **Cron jobs** — `crontab -e` for scheduled tasks on Linux
2. **Systemd** — managing services that start on boot (`systemctl start`, `enable`)
3. **Nginx** — reverse proxy to route traffic to your Go server
4. **awk and sed** — powerful text processing for log parsing
5. **Linux file permissions** — `chmod`, `chown`, `644` vs `755`
6. **Environment management** — `.bashrc` vs `.bash_profile` vs `.env`
