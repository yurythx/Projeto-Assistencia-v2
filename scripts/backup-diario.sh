#!/bin/bash
set -euo pipefail

APP_DIR="/opt/projeto-assistencia-v2"
ENV_FILE="$APP_DIR/.env"
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

umask 077
mkdir -p "$BACKUP_DIR"
chmod 700 "$BACKUP_DIR"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

# O dump é gravado num .partial e só vira .sql.gz depois de validado: uma
# falha no meio (container fora, disco cheio) nunca deixa um arquivo
# truncado com cara de backup bom — nem entra na rotação no lugar de um.
DB_FILE="$BACKUP_DIR/db_${TIMESTAMP}.sql.gz"
TMP_FILE="$DB_FILE.partial"
trap 'rm -f "$TMP_FILE"' EXIT

log "Iniciando rotina de backup diário..."

# 1. Backup do Banco de Dados PostgreSQL
if docker compose -f "$APP_DIR/docker-compose.yml" exec -T postgres \
        pg_dump -U "$DB_USER" "$DB_NAME" | gzip > "$TMP_FILE" \
    && gzip -t "$TMP_FILE" \
    && [ "$(gzip -dc "$TMP_FILE" | head -c 1M | wc -c)" -gt 0 ]; then
    mv "$TMP_FILE" "$DB_FILE"
    DB_SIZE=$(du -h "$DB_FILE" | cut -f1)
    log "Backup do PostgreSQL concluído com sucesso: $DB_FILE ($DB_SIZE)"
else
    log "ERRO ao gerar backup do PostgreSQL!"
    exit 1
fi

# 2. Rotação de backups antigos (> 7 dias) — só roda depois de um backup
# bom, então uma sequência de falhas nunca apaga os últimos válidos.
log "Removendo backups com mais de $RETENTION_DAYS dias..."
find "$BACKUP_DIR" -type f -name "*.sql.gz" -mtime +"$RETENTION_DAYS" -delete
find "$BACKUP_DIR" -type f -size -100c -name "*.sql.gz" -delete

log "Rotina de backup concluída com sucesso."
