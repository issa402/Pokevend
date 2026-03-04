# PokémonTool 🃏⚡
### Real-Time Card Vendor Intelligence Platform

> **Eliminate the scroll. Automate the grind. Dominate the market.**

PokémonTool is a full-stack, distributed intelligence platform built for Pokémon card vendors who vend at shows, flip online, or build inventory. It aggregates live pricing data from eBay, TCGplayer, Facebook Marketplace, and Mercari — detects trending and downtrending cards — surfaces nearby shows — and fires instant price alerts, all from one dashboard.

---

## ✨ Features

| Feature | Description |
|---|---|
| 📈 **Trend Detection** | Rising, stable, and falling cards identified via custom analytics engine |
| 🔔 **Real-Time Alerts** | Instant SSE-pushed notifications when a watched card price changes |
| 🛒 **Multi-Marketplace** | eBay, TCGplayer, Facebook Marketplace, and Mercari monitored simultaneously |
| 💰 **Deal of the Day** | AI-curated best-value cards to buy each morning |
| 📦 **Inventory Manager** | Track your personal collection + CSV import/export |
| 🗺️ **Show Finder** | Upcoming nearby Pokémon TCG shows via Eventbrite |
| 🔑 **API Key Manager** | Securely store your own eBay / TCGplayer keys (AES-256 encrypted) |
| 📰 **News Scanner** | Auto-detects card price spikes from tournament/set reveal news |

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────┐
│               React Frontend (Vite + SSE)                │
└────────────────────────┬────────────────────────────────┘
                         │ HTTP + SSE
┌────────────────────────▼────────────────────────────────┐
│            Node.js API Gateway (Express)                 │
│     Auth · Watchlist · Alerts · Cards · Inventory        │
└───┬───────────┬──────────────────┬──────────────────────┘
    │           │                  │
    │ PostgreSQL │ MongoDB          │ Redis (cache)
    │           │                  │
┌───▼───────────▼──────────────────▼──────────────────────┐
│                   RabbitMQ Message Queue                  │
└───────┬───────────────────┬──────────────────────────────┘
        │                   │
┌───────▼──────┐   ┌────────▼────────┐   ┌───────────────┐
│ Python       │   │ Go Scraping     │   │ Python        │
│ API Consumer │   │ Service         │   │ Analytics     │
│ eBay/TCG     │   │ FB Marketplace  │   │ Engine        │
│ Notification │   │ Mercari         │   │ Trends/Deals  │
└──────────────┘   └─────────────────┘   └───────────────┘
```

---

## 🛠️ Tech Stack

| Layer | Technology | Why |
|---|---|---|
| Frontend | React 18 + Vite | Fast, component-based UI with real-time SSE updates |
| API Gateway | Node.js / Express | High-concurrency I/O, SSE management |
| eBay/TCG Worker | Python | Rich API libraries, Pydantic models |
| Analytics Engine | Python (Pandas, NumPy) | Best-in-class data science tooling |
| Scraper | Go + Playwright | Goroutine concurrency for fast browser automation |
| Automation Scripts | Bash | Dev setup, deployment, DB seeding |
| Databases | PostgreSQL + MongoDB + Redis | Relational + documents + cache |
| Message Queue | RabbitMQ | Async decoupling of all services |
| Orchestration | Docker Compose | One-command startup |

---

## 📁 Project Structure

```
/pokemontool
├── client/                     # React Frontend (Vite)
│   └── src/
│       ├── components/         # WatchlistCard, TrendChart, AlertsFeed, etc.
│       ├── pages/              # Dashboard, Watchlist, Trends, Inventory, Shows, Settings
│       ├── services/           # API client (Axios), SSE client
│       └── store/              # Redux Toolkit state slices
│
├── server/                     # Node.js API Gateway
│   ├── controllers/            # Business logic per route
│   ├── routes/                 # Express routers
│   ├── middleware/             # Auth JWT, rate limiting, error handling
│   ├── models/                 # PG + Mongoose schemas
│   ├── services/               # SSE manager, notification dispatcher
│   └── config/                 # DB connections, env config
│
├── services/
│   ├── api-consumer/           # Python: eBay + TCGplayer data ingestion
│   ├── analytics-engine/       # Python: Trend detection, deals, news scan
│   └── scraping-service/       # Go: Facebook Marketplace + Mercari scraper
│
├── database/
│   ├── migrations/             # PostgreSQL schema SQL files
│   └── seeds/                  # Seed data (Pokemon sets, sample cards)
│
├── scripts/
│   ├── setup.sh                # Install all dependencies
│   ├── start-dev.sh            # Start all services in dev mode
│   └── seed-db.sh              # Run DB migrations + seeds
│
├── docker-compose.yml          # Full stack orchestration
├── .env.example                # Environment variable template
└── README.md                   # This file
```

---

## 🚀 Quick Start

### Prerequisites
- Docker & Docker Compose
- Node.js 18+ (for local dev without Docker)
- Python 3.11+ (for local Python services)
- Go 1.21+ (for local scraping service)

### 1. Clone & Setup Environment
```bash
git clone <your-repo-url> pokemontool
cd pokemontool
cp .env.example .env
# Edit .env with your API keys
```

### 2. Install Dependencies (Local Dev)
```bash
chmod +x scripts/setup.sh
./scripts/setup.sh
```

### 3. Start the Full Stack (Docker)
```bash
docker-compose up --build
```

Or for local development without Docker:
```bash
chmod +x scripts/start-dev.sh
./scripts/start-dev.sh
```

### 4. Open the Dashboard
Visit **http://localhost:5173**

---

## 🔑 API Keys Setup

After starting the app, navigate to **Settings → API Keys** to enter:

| Platform | Where to Get It | Cost |
|---|---|---|
| **eBay** | [developer.ebay.com](https://developer.ebay.com) | Free developer account |
| **TCGplayer** | [tcgplayer.com/developers](https://tcgplayer.com/developers) | Commercial agreement required |
| **Eventbrite** | [eventbrite.com/platform](https://www.eventbrite.com/platform) | Free |
| **Facebook** | No API — scraping used instead | N/A |

All keys are encrypted with AES-256 before storage.

---

## 📊 Data Sources

| Source | Method | Data Retrieved |
|---|---|---|
| **eBay** | Buy API + Notification API | Live listings, price changes, sell-through |
| **TCGplayer** | Commercial API | Market prices, sale velocity, trends |
| **Facebook Marketplace** | Playwright (Go) | Local seller listings |
| **Mercari** | Playwright (Go) | Secondary market listings |
| **Eventbrite** | REST API | Upcoming Pokemon shows |
| **Pokemon News Sites** | BeautifulSoup scraping | Set reveals, tournament results |

---

## 📜 License

MIT — built for the community 🏆