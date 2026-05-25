set -euo pipefail

SCRIPTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPTS_DIR/.." && pwd)"

echo "=== Docker Status For Pokemon ==="

cd "$SCRIPTS_DIR" || { echo "Failed to enter scripts directory"; exit 1; }

SCRIPT_CONTAINERS=$(docker ps -q | wc -l)

if [ "$SCRIPT_CONTAINERS" -gt 1 ]; then
	echo "Status of POKEMON CONTAINERS IN USE ( $SCRIPT_CONTAINERS service running)"
else
	echo "STATUS: NOT in use ($SCRIPT_CONTAINERS services running)"
fi


echo "___________"

echo "MOVING TO PROJECT ROOT"
cd "$PROJECT_ROOT" || { echo "FAILED TO ENTER PROJECT ROOT"; exit 1; }
ROOT_CONTAINERS=$(docker ps -q | wc -l)

if [ "$ROOT_CONTAINERS" -gt 1 ]; then 
	echo "STATUS: IN USE ($ROOT_CONTAINERS services running)"
else
	echo "FAILED STATUS"
fi

