set -euo pipefail

echo "READING HOST"
hostname

echo "TIME OF INCIDENT"
date 

SCRIPTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
STAMP="$(date +%Y+%m+%d_%H%M%S)"
REPORT_DIR="$SCRIPTS_DIR/logs/incident/$STAMP"

COMPOSER_FILE="$PROJECT_DIR/docker-compose.yml"
OVERRIDE_FILE="$PROJECT_DIR/docker-compose.local-no-postgres-port.yml"

mkdir -p "$REPORT_DIR"

log() {
    ehco "$*" | te -a "$REPORT_DIR/summary.txt"

}

capture () {
    local name="$1"
    shift

    log "Capturing: $name"
    {
        echo "COMMAND: $*"
        echo "TIME: $(date)"
        echo
        "@"
    } > "$REPORT_DIR/$name.txt" 2>&1 || true
}

log "Pokemon incident snapshot"
log "Report directory: $REPORT_DIR"
log "STARTED AT: $(date)"
log ""

capture "host_date" date
capture "host_uptime" uptime
capture "host_disk" df -h
capture "host_memory" free -h

capture "Docker_state" docker ps --format "table {{.Names}}\t{{.Staus}}\t{{.Ports}}"
capture "DOCKER COMPSOE PS" docker compose -f "$COMPOSE_FILE" -f "$OVERRIDE_FILE" ps

capture "port listening" ss -tulpn

capture "frontend_health" curl -sS -i http://localhost:5173
capture "api_health" curl -sS -i http://localhost:3001/health

capture "rabbitmq" docker exec pokemontool_rabbitmq rabbitmqctl list_queues name messages messages_ready messages_unacknowledged consumers

# Recent logs from important services.
capture "server_logs" docker logs --tail 200 pokemontool_server
capture "client_logs" docker logs --tail 100 pokemontool_client
capture "api_consumer_logs" docker logs --tail 200 pokemontool_api_consumer
capture "analytics_logs" docker logs --tail 200 pokemontool_analytics
capture "postgres_logs" docker logs --tail 100 pokemontool_postgres
capture "rabbitmq_logs" docker logs --tail 100 pokemontool_rabbitmq
capture "redis_logs" docker logs --tail 100 pokemontool_redis


log ""
log "Quick error scan:"
grep -RniE "error|failed|panic|fatal|exception|traceback|refused|timeout" "$REPORT_DIR" \ | head -80 \ | tee "$REPORT_DIR/error_scan.txt || true

log ""
log "Snapshot complete"
log "Read this first: $REPORT_DIR/summary.txt"
log "Error scan: $REPORT_DIR/error_scan.txt"


echo "READING GIT STATUS"
git status
git branch




