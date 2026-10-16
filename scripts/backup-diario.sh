#!/bin/bash
set -euo pipefail

ENV_FILE="/opt/projeto-assistencia-v2/.env"
if [ -f "$ENV_FILE" ]; then
    set -a
    # shellcheck disable=SC1090
    source "$ENV_FILE"
    set +a
fi

DB_USER="${DB_USER:-nexus}"
DB_NAME="${DB_NAME:-nexus}"
BACKUP_DIR="/var/backups/projeto-assistencia"
LOG_FILE="/var/log/assistencia-backup.log"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
RETENTION_DAYS=7

mkdir -p "$BACKUP_DIR"
chmod 700 "$BACKUP_DIR"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

log "Iniciando rotina de backup diário..."

# 1. Backup do Banco de Dados PostgreSQL
DB_FILE="$BACKUP_DIR/db_${TIMESTAMP}.sql.gz"
if docker compose -f /opt/projeto-assistencia-v2/docker-compose.yml exec -T postgres pg_dump -U "$DB_USER" "$DB_NAME" | gzip > "$DB_FILE"; then
    DB_SIZE=$(du -h "$DB_FILE" | cut -f1)
    log "Backup do PostgreSQL concluído com sucesso: $DB_FILE ($DB_SIZE)"
else
    log "ERRO ao gerar backup do PostgreSQL!"
    exit 1
fi

# 2. Rotação de backups antigos (> 7 dias)
log "Removendo backups com mais de $RETENTION_DAYS dias..."
find "$BACKUP_DIR" -type f -name "*.sql.gz" -mtime +"$RETENTION_DAYS" -exec rm -f {} \;

log "Rotina de backup concluída com sucesso."
