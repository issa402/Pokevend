#!/bin/bash
set -e

# Get a list of all loaded unit names, filtering out empty lines
units=$(systemctl list-units --all --no-legend --plain | awk '{print $1}')

echo "Checking units with logs from the last 10 days..."
echo "----------------------------------------------"

for sys in $units; do
    # Skip if the unit name is empty or just whitespace
    if [ -z "$sys" ]; then
        continue
    fi

    # Check if any logs exist for this specific unit
    # 2>/dev/null hides errors if a unit name is weird/invalid
    if journalctl -u "$sys" --since "10 days ago" --quiet --grep ".*" 2>/dev/null; then
        echo "Found logs for: $sys"
    fi
done
