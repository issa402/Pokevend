# Answers: Your Questions About the Project

---

## 1. chi vs Gin — Which Is Higher Value?

**Short answer: chi is completely fine. Do NOT switch.**

Here's the full picture:

| | chi | Gin |
|---|---|---|
| **Popularity** | Used by Cloudflare, InfluxDB, hundreds of serious projects | More popular by download count |
| **Philosophy** | 100% `net/http` compatible — any handler is a chi handler | Own handler types (`gin.Context`) — harder to migrate away |
| **Middleware** | Standard `http.Handler` chain — every Go lib works | Gin-specific middleware format |
| **FAANG usage** | Used in production at major companies | Also used at major companies |
| **Performance** | Near identical (both route ~10ns) | ~same, negligible difference |
| **Idiomatic Go** | ✅ YES — uses standard library types | ❌ Not as idiomatic — own ecosystem |

**The real FAANG truth:** At Google, Uber, Netflix, Meta — they mostly use **gRPC** for internal services and raw `net/http` for external APIs. No framework at all for internal code. chi is the closest to "framework-free" Go while still getting routing.

**Switching to Gin now would be a mistake** because:
1. Gin uses `*gin.Context` instead of `http.ResponseWriter + *http.Request` — YOUR ENTIRE HANDLER LAYER changes
2. All your middleware would break
3. You'd learn Gin-specific patterns that aren't transferable

> **Bottom line:** chi IS the high-value choice. It teaches you how Go HTTP actually works.

---

## 2. Do You Need AWS First, or Can You Test Locally?

**You can test EVERYTHING locally right now — no AWS needed.**

Here's what Docker Compose gives you locally:

```
docker compose up -d

→ Starts:
  postgres   on localhost:5432   (same as AWS RDS)
  redis      on localhost:6379   (same as AWS ElastiCache)
  rabbitmq   on localhost:5672   (same as AWS AmazonMQ)
  go server  on localhost:3001   (same as EC2 container)
  api-consumer on localhost:8001 (same as EC2 container)
```

**Local = exactly the same as production, just on your machine.**

**AWS is ONLY needed for:**
- Sharing with other people (public URL)
- Running 24/7 without your computer on
- Production traffic with real users
- Performance testing at scale

**The right order:**
```
Step 1: Test locally with Docker (done on YOUR machine)
Step 2: When it works perfectly locally → deploy to EC2
Step 3: EC2 runs docker compose too — same exact setup
Step 4 (optional): Switch Postgres container → AWS RDS (better backups)
```

---

## 3. How to Test Things in the Terminal Right Now

### Prerequisites (do this once)
```bash
# Make sure Docker Desktop is running, then:
cd C:\Users\isjim\OneDrive\Desktop\Pokemon
docker compose up -d postgres redis rabbitmq
```

---

### Testing Go

```bash
# Check if Go is installed
go version

# Compile check (catches ALL type errors without running)
cd server
go build ./...
# If it prints nothing → zero errors. If it prints errors → find the file + line

# Vet check (catches common runtime bugs)
go vet ./...

# Run a specific file (not the full server)
go run main.go
# Server starts on :3001. Open another terminal to test it.

# Run unit tests (once you write them)
go test ./...
go test ./services/... -v  # verbose: shows each test name
```

---

### Testing Python

```bash
# Check Python is installed
python3 --version

# Go into an api-consumer directory
cd services/api-consumer

# Set up virtual env (once)
python3 -m venv venv
source venv/bin/activate  # Mac/Linux
# On Windows PowerShell: .\venv\Scripts\Activate.ps1

# Install dependencies
pip install -r requirements.txt

# Run a quick script to verify imports work
python3 -c "import fastapi; import pydantic; print('imports OK')"

# Run the seed script (needs postgres running)
cd C:\Users\isjim\OneDrive\Desktop\Pokemon
python3 scripts/seed_cards.py --verbose

# Run the DB report
python3 scripts/db_report.py

# Run FastAPI dev server (with auto-reload on file save)
cd services/api-consumer
uvicorn main:app --reload --port 8001
# Then open: http://localhost:8001/docs (auto-generated API docs!)
```

---

### Testing SQL

```bash
# Connect to postgres running in Docker
docker exec -it pokemontool_postgres psql -U pokemontool_user -d pokemontool

# Inside psql:
\dt                    # list all tables
\d cards               # describe the cards table (columns + types)
SELECT COUNT(*) FROM cards;
SELECT * FROM cards LIMIT 5;
\q                     # quit

# Run a migration file
docker exec -i pokemontool_postgres psql -U pokemontool_user -d pokemontool \
  < database/migrations/001_init.sql

# One-liner query from terminal (not interactive)
docker exec -it pokemontool_postgres psql -U pokemontool_user -d pokemontool \
  -c "SELECT name, price_tcgplayer FROM cards ORDER BY price_tcgplayer DESC LIMIT 5;"
```

---

### Testing Bash Scripts

```bash
# Make scripts executable (do this once)
chmod +x scripts/*.sh

# Run health check (after docker compose up)
./scripts/health-check.sh

# Run setup
./scripts/setup-dev.sh

# Debug a script line by line
bash -x scripts/health-check.sh
# -x = print each command before running it (shows exactly what's happening)
```

---

## 4. Is Our Code Good Right Now?

**Mostly yes — with a few things to know:**

### ✅ What's solid
- 8-layer Go architecture (config → models → store → services → handlers → routes → worker → main)
- Interface-based repositories (testable, swappable)
- JWT auth with middleware injection
- AES-256 API key encryption
- RabbitMQ for decoupled Python→Go communication
- SSE for real-time alerts
- Rate limiting per IP
- Parameterized SQL (no injection)

### ⚠️ Things to watch
| Issue | Status | Fix |
|---|---|---|
| `contetx.Context` typo in alert_store.go | ✅ Fixed just now | none needed |
| `GetUnreadCount` is in interface but no implementation yet | ☐ Your Task 1 | Practice Task 1 |
| `ListByUser` in store expects a `limit` arg — some callers may not pass it | ☐ Check alert_handler.go calls | probably fine |
| Python scripts need `pip install psycopg2-binary python-dotenv` | ☐ Your action | add to requirements |
| No unit tests exist yet | ☐ Your Task 11 | add in practice |

### Add psycopg2 to scripts requirements
The scripts use psycopg2 — add it if not in your root requirements. Run in project root:
```bash
pip install psycopg2-binary python-dotenv
```

---

## 5. What's a Good Order to Start Testing Right Now?

```
1. cd Pokemon && docker compose up -d postgres redis rabbitmq
2. python3 scripts/seed_cards.py --verbose         ← puts cards in DB
3. python3 scripts/db_report.py                    ← shows what's in DB
4. cd server && go build ./...                     ← make sure Go compiles
5. cd server && go run main.go                     ← start the Go server
6. (new terminal) curl http://localhost:3001/health ← verify it responds
7. ./scripts/health-check.sh                       ← full check
```

That's your "is everything working?" sequence.
