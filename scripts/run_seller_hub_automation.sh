#!/usr/bin/env bash
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
service_dir="$project_root/services/api-consumer"
python_bin="$service_dir/venv/bin/python"

if [[ ! -x "$python_bin" ]]; then
  echo "Missing API consumer virtualenv: $python_bin" >&2
  exit 1
fi

export POSTGRES_HOST="${POSTGRES_HOST:-127.0.0.1}"
export POSTGRES_PORT="${POSTGRES_PORT:-55433}"

cd "$service_dir"
exec "$python_bin" seller_hub_research_batch.py "$@"
