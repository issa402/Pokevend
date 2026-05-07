set -euo pipefail 

echo "========= HOST =========="
hostname

echo "======== DISK =========="
df -h | awk '{print $1, $2}' | column -t

echo "======= DATE ========"
date
