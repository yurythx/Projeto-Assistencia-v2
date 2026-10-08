#!/usr/bin/env bash
# Backup do Projeto-Assistencia-v2 (stack docker compose deste diretório):
#   db.dump        pg_dump -Fc do banco do Nexus (restaurável com restore.sh)
#   minio/         espelho de todos os buckets (arquivos, anexos, WORM)
#   caddy.tgz      CA interna do HTTPS, se existir (chave privada inclusive)
#   keycloak.tgz   volume do Keycloak de teste, se existir (melhor esforço:
#                  o realm também é reproduzível por deploy/keycloak)
#   config/        .env e secrets/ — SEM eles o backup não restaura: a senha
#                  do banco e a CONFIG_ENCRYPTION_KEY estão aqui
#   SHA256SUMS     hashes de tudo acima
#
#   scripts/backup.sh                 # grava em $BACKUP_DIR/<data-hora>
#   scripts/backup.sh --install-cron  # agenda diariamente às 02:30
#
# BACKUP_DIR (padrão /root/backups/nexus) e BACKUP_KEEP_DAYS (padrão 7).
# Fica no MESMO disco do servidor: protege contra erro humano e dados
# corrompidos, não contra perda da máquina — copie para fora (rsync/rclone).
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"
BACKUP_DIR="${BACKUP_DIR:-/root/backups/nexus}"
KEEP_DAYS="${BACKUP_KEEP_DAYS:-7}"

if [ "${1:-}" = "--install-cron" ]; then
  cat >/etc/cron.d/nexus-backup <<EOF
# Backup diário do Projeto-Assistencia-v2 (scripts/backup.sh --install-cron)
SHELL=/bin/bash
30 2 * * * root cd $ROOT && BACKUP_DIR=$BACKUP_DIR BACKUP_KEEP_DAYS=$KEEP_DAYS ./scripts/backup.sh >>/var/log/nexus-backup.log 2>&1
EOF
  chmod 644 /etc/cron.d/nexus-backup
  echo "Agendado: /etc/cron.d/nexus-backup (02:30, log em /var/log/nexus-backup.log)"
  exit 0
fi

env_val() { grep "^$1=" .env | tail -1 | cut -d= -f2- || true; }
DB_USER="$(env_val DB_USER)"; DB_USER="${DB_USER:-nexus}"
DB_NAME="$(env_val DB_NAME)"; DB_NAME="${DB_NAME:-nexus}"
PROJECT="$(docker compose config 2>/dev/null | sed -n 's/^name: //p' | head -1)"

dest="$BACKUP_DIR/$(date +%Y%m%d-%H%M%S)"
umask 077
mkdir -p "$dest/config" "$dest/minio"
log() { printf '%s %s\n' "$(date '+%F %T')" "$*"; }
log "backup -> $dest"

log "postgres: pg_dump $DB_NAME"
docker compose exec -T postgres pg_dump -U "$DB_USER" -d "$DB_NAME" -Fc --no-owner >"$dest/db.dump"

log "minio: mc mirror de todos os buckets"
docker compose run --rm --no-deps -T -v "$dest/minio:/out" --entrypoint sh minio -c \
  'export MC_HOST_l="http://$MINIO_ROOT_USER:$MINIO_ROOT_PASSWORD@minio:9000"; mc mirror --quiet --preserve l /out' >/dev/null

# Volumes opcionais: Keycloak de teste e a CA interna do Caddy (HTTPS) —
# perder a CA obriga toda máquina de teste a instalar uma raiz nova.
for vol in keycloak caddy; do
  if docker volume inspect "${PROJECT}_${vol}_data" >/dev/null 2>&1; then
    log "$vol: volume ${PROJECT}_${vol}_data"
    docker run --rm -v "${PROJECT}_${vol}_data:/data:ro" -v "$dest:/out" postgres:16-bookworm \
      tar czf "/out/$vol.tgz" -C /data .
    chmod 600 "$dest/$vol.tgz"
  fi
done

log "config: .env e secrets/"
cp .env "$dest/config/.env"
[ -d secrets ] && cp -r secrets "$dest/config/"

(cd "$dest" && find . -type f ! -name SHA256SUMS -print0 | sort -z | xargs -0 sha256sum >SHA256SUMS)
log "ok: $(du -sh "$dest" | cut -f1), $(wc -l <"$dest/SHA256SUMS") arquivos"

find "$BACKUP_DIR" -mindepth 1 -maxdepth 1 -type d -mtime +"$KEEP_DAYS" -print -exec rm -rf {} + |
  sed 's/^/removido (retenção): /'
