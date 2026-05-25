#!/usr/bin/env bash
set -euo pipefail

PORTS=(3001 5173 5432 6379 5672 15672 9090 3000 3100)
FAILED=0

echo "Pokemon port preflight check"

for PORT in "${PORTS[@]}"; do
    if ss -tulpn | grep -q ":$PORT "; then
        echo "IN USE: $PORT"
        FAILED=1
    else
        echo "FREE:   $PORT"
    fi
done

if [[ "$FAILED" -eq 1 ]]; then
    echo "One or more required ports are already in use."
    exit 1
fi

echo "All required ports are free."

	
