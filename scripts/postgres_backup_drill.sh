#!/usr/bin/env bash
# ============================================================
# FILE: scripts/postgres_backup_drill.sh
# TYPE: Infrastructure Script - PostgreSQL backup and restore drill
#
# WHY THIS EXISTS
# Health checks tell you whether the app is alive right now.
# A backup drill tells you whether the business can survive data loss.
#
# WHAT IT DOES
#   backup         Create a compressed custom-format pg_dump backup.
#   verify FILE    Prove a backup is readable with pg_restore --list.
#   list           Show saved backups.
#   restore FILE   Restore a backup into the current database, with a safety prompt.
#
# HOW TO RUN
#   ./scripts/postgres_backup_drill.sh backup
#   ./scripts/postgres_backup_drill.sh list
#   ./scripts/postgres_backup_drill.sh verify backups/postgres/pokemontool_20260511_120000.dump
#   ./scripts/postgres_backup_drill.sh restore backups/postgres/pokemontool_20260511_120000.dump
#   ./scripts/postgres_backup_drill.sh restore backups/postgres/pokemontool_20260511_120000.dump --yes
# ============================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="$PROJECT_ROOT/.env"
BACKUP_DIR="$PROJECT_ROOT/backups/postgres"

POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-pokemontool_postgres}"
RETENTION_DAYS="${RETENTION_DAYS:-7}"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

usage() {
    cat <<EOF
Pokemon Postgres backup drill

Usage:
  $0 backup
  $0 list
  $0 verify <backup-file>
  $0 restore <backup-file> [--yes]

Environment overrides:
  POSTGRES_CONTAINER=$POSTGRES_CONTAINER
  RETENTION_DAYS=$RETENTION_DAYS
EOF
}

env_value() {
    local key="$1"
    local default="$2"

    if [[ ! -f "$ENV_FILE" ]]; then
        printf '%s' "$default"
        return
    fi

    local line
    line="$(grep -E "^${key}=" "$ENV_FILE" | tail -n 1 || true)"

    if [[ -z "$line" ]]; then
        printf '%s' "$default"
        return
    fi

    local value="${line#*=}"
    value="${value%$'\r'}"
    value="${value%\"}"
    value="${value#\"}"
    value="${value%\'}"
    value="${value#\'}"

    printf '%s' "$value"
}

postgres_user() {
    env_value "POSTGRES_USER" "pokemontool_user"
}

postgres_db() {
    env_value "POSTGRES_DB" "pokemontool"
}

require_docker() {
    if ! command -v docker >/dev/null 2>&1; then
        echo -e "${RED}docker command not found${NC}" >&2
        exit 1
    fi
}

require_postgres_container() {
    if ! docker inspect "$POSTGRES_CONTAINER" >/dev/null 2>&1; then
        echo -e "${RED}Postgres container not found: $POSTGRES_CONTAINER${NC}" >&2
        echo "Start the stack first: docker compose up -d postgres" >&2
        exit 1
    fi

    if ! docker exec "$POSTGRES_CONTAINER" pg_isready -U "$(postgres_user)" -d "$(postgres_db)" >/dev/null 2>&1; then
        echo -e "${RED}Postgres is not ready inside container: $POSTGRES_CONTAINER${NC}" >&2
        exit 1
    fi
}

backup_database() {
    require_docker
    require_postgres_container
    mkdir -p "$BACKUP_DIR"

    local db
    local user
    local timestamp
    local filename
    local host_file
    local container_file

    db="$(postgres_db)"
    user="$(postgres_user)"
    timestamp="$(date +%Y%m%d_%H%M%S)"
    filename="${db}_${timestamp}.dump"
    host_file="$BACKUP_DIR/$filename"
    container_file="/tmp/$filename"

    echo "Creating Postgres backup..."
    echo "  database:  $db"
    echo "  container: $POSTGRES_CONTAINER"
    echo "  output:    $host_file"

    docker exec "$POSTGRES_CONTAINER" \
        pg_dump \
        -U "$user" \
        -d "$db" \
        --format=custom \
        --no-owner \
        --no-privileges \
        --file "$container_file"

    docker cp "$POSTGRES_CONTAINER:$container_file" "$host_file"
    docker exec "$POSTGRES_CONTAINER" rm -f "$container_file"

    verify_backup "$host_file"
    prune_old_backups

    echo -e "${GREEN}Backup complete:${NC} $host_file"
}

verify_backup() {
    local backup_file="${1:-}"

    require_docker
    require_postgres_container

    if [[ -z "$backup_file" ]]; then
        echo -e "${RED}Missing backup file${NC}" >&2
        usage
        exit 1
    fi

    if [[ ! -f "$backup_file" ]]; then
        echo -e "${RED}Backup file not found: $backup_file${NC}" >&2
        exit 1
    fi

    echo "Verifying backup can be read..."

    if docker exec -i "$POSTGRES_CONTAINER" pg_restore --list < "$backup_file" >/dev/null; then
        echo -e "${GREEN}Backup is readable:${NC} $backup_file"
    else
        echo -e "${RED}Backup verification failed:${NC} $backup_file" >&2
        exit 1
    fi
}

list_backups() {
    mkdir -p "$BACKUP_DIR"

    echo "Saved Postgres backups:"

    if ! find "$BACKUP_DIR" -maxdepth 1 -type f -name "*.dump" -print -quit | grep -q .; then
        echo "  none yet"
        return
    fi

    find "$BACKUP_DIR" -maxdepth 1 -type f -name "*.dump" -printf "  %TY-%Tm-%Td %TH:%TM  %s bytes  %p\n" | sort -r
}

restore_backup() {
    local backup_file="${1:-}"
    local assume_yes="${2:-}"

    require_docker
    require_postgres_container

    if [[ -z "$backup_file" ]]; then
        echo -e "${RED}Missing backup file${NC}" >&2
        usage
        exit 1
    fi

    if [[ ! -f "$backup_file" ]]; then
        echo -e "${RED}Backup file not found: $backup_file${NC}" >&2
        exit 1
    fi

    verify_backup "$backup_file"

    local db
    local user
    db="$(postgres_db)"
    user="$(postgres_user)"

    if [[ "$assume_yes" != "--yes" ]]; then
        echo -e "${YELLOW}Restore will overwrite objects in database: $db${NC}"
        read -r -p "Type RESTORE to continue: " confirmation

        if [[ "$confirmation" != "RESTORE" ]]; then
            echo "Restore cancelled."
            exit 0
        fi
    fi

    echo "Restoring backup into $db..."

    docker exec -i "$POSTGRES_CONTAINER" \
        pg_restore \
        -U "$user" \
        -d "$db" \
        --clean \
        --if-exists \
        --no-owner \
        --no-privileges \
        < "$backup_file"

    echo -e "${GREEN}Restore complete.${NC}"
}

prune_old_backups() {
    mkdir -p "$BACKUP_DIR"

    find "$BACKUP_DIR" \
        -maxdepth 1 \
        -type f \
        -name "*.dump" \
        -mtime "+$RETENTION_DAYS" \
        -print \
        -delete
}

main() {
    local command="${1:-}"

    case "$command" in
        backup)
            backup_database
            ;;
        list)
            list_backups
            ;;
        verify)
            verify_backup "${2:-}"
            ;;
        restore)
            restore_backup "${2:-}" "${3:-}"
            ;;
        -h|--help|help|"")
            usage
            ;;
        *)
            echo -e "${RED}Unknown command: $command${NC}" >&2
            usage
            exit 1
            ;;
    esac
}

main "$@"
