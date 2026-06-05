#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL_DIR="${LAST30DAYS_SKILL_DIR:-$HOME/.agents/skills/last30days}"
ENGINE="$SKILL_DIR/scripts/last30days.py"
SAVE_DIR="${LAST30DAYS_SAVE_DIR:-$ROOT_DIR/reports/last30days}"
DAYS="${LAST30DAYS_DAYS:-30}"
SEARCH="${LAST30DAYS_SEARCH:-reddit,hackernews,github,web,polymarket}"
MODE="--deep"

usage() {
  cat <<'USAGE'
Usage: scripts/run_last30days_vendor_pulse.sh [--quick] [--deep] [--days N] [--search SOURCES]

Runs a curated last30days market pulse for Pokemon graded-card vendor intelligence.

Examples:
  scripts/run_last30days_vendor_pulse.sh
  scripts/run_last30days_vendor_pulse.sh --quick
  scripts/run_last30days_vendor_pulse.sh --search reddit,hackernews,github,web,polymarket,youtube,tiktok
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --quick)
      MODE="--quick"
      shift
      ;;
    --deep)
      MODE="--deep"
      shift
      ;;
    --days)
      DAYS="${2:?--days requires a value}"
      shift 2
      ;;
    --search)
      SEARCH="${2:?--search requires a value}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ ! -f "$ENGINE" ]]; then
  echo "last30days skill engine not found at $ENGINE" >&2
  echo "Install it with: npx skills add mvanhorn/last30days-skill -g -a codex" >&2
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required to run last30days." >&2
  exit 1
fi

mkdir -p "$SAVE_DIR"
PLAN_FILE="$(mktemp "${TMPDIR:-/tmp}/pokemon-last30days-plan.XXXXXX.json")"
trap 'rm -f "$PLAN_FILE"' EXIT

cat > "$PLAN_FILE" <<'JSON'
{
  "intent": "Find recent social, marketplace, and seller pain signals that help price and source Pokemon graded cards for ecommerce vendors.",
  "freshness_mode": "last_30_days",
  "cluster_mode": "vendor_market_pulse",
  "raw_topic": "Pokemon graded card vendor pricing and sourcing intelligence",
  "subqueries": [
    {
      "label": "vendor pain",
      "search_query": "Pokemon card vendors pricing slabs eBay watchers sold comps Reddit last 30 days",
      "ranking_query": "seller and vendor pain points around pricing, watchers, bids, sold comps, card show inventory, margins, and automation",
      "sources": ["reddit", "hackernews", "github", "web"],
      "weight": 1.2
    },
    {
      "label": "buyer demand",
      "search_query": "Pokemon slab PSA 10 trending cards eBay sold comps collectors Reddit last 30 days",
      "ranking_query": "specific Pokemon graded cards, sets, or products collectors discuss as rising demand, mispriced, liquid, or risky",
      "sources": ["reddit", "youtube", "tiktok", "web"],
      "weight": 1.1
    },
    {
      "label": "platform ops",
      "search_query": "eBay Shopify Odoo card seller inventory pricing automation complaints last 30 days",
      "ranking_query": "operational problems sellers have with eBay, Shopify, Odoo, inventory sync, repricing, alerts, and marketplace research",
      "sources": ["reddit", "hackernews", "github", "web"],
      "weight": 1.0
    }
  ],
  "source_weights": {
    "reddit": 1.3,
    "hackernews": 0.9,
    "github": 0.8,
    "youtube": 1.1,
    "tiktok": 1.1,
    "web": 1.0,
    "polymarket": 0.5
  },
  "notes": [
    "Use this as a market-intelligence input, not as direct pricing truth.",
    "Prioritize signals that can feed Finder watchlists, Seller Hub research targets, alert tuning, and vendor-facing product copy."
  ]
}
JSON

echo "Running Pokemon vendor pulse via last30days..."
echo "Save dir: $SAVE_DIR"
echo "Sources: $SEARCH"

python3 "$ENGINE" \
  "Pokemon graded card vendor pricing and sourcing intelligence" \
  --emit md \
  "$MODE" \
  --days "$DAYS" \
  --search "$SEARCH" \
  --plan "$PLAN_FILE" \
  --save-dir "$SAVE_DIR" \
  --save-suffix "pokemon-vendor-pulse"

