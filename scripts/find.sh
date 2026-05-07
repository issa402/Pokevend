#!/bin/bash
set -e
SCRIPTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPTS_DIR/.." && pwd)"
for file in $(find "$PROJECT_ROOT" -type f -name "*.md"); do 
    echo "file: $file"
done