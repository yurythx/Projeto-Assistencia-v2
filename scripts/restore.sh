#!/usr/bin/env bash
# Restaura um backup de scripts/backup.sh.
#
#   scripts/restore.sh --verify <dir>          # NÃO altera nada: confere os
#       hashes, restaura o dump num banco temporário, compara contagens
#       com o banco atual e descarta o temporário
#   scripts/restore.sh --yes <dir> [--minio]   # SUBSTITUI o banco atual
#       (e, com --minio, sobrepõe os objetos do MinIO) pelo backup
#
# O .env e secrets/ do backup não são copiados de volta sozinhos: se esta
# máquina for nova, copie <dir>/config/.env e <dir>/config/secrets ANTES
# (a senha do banco e a CONFIG_ENCRYPTION_KEY precisam ser as do backup).
set -euo pipefail

cd "$(dirname "$0")/.."
mode="${1:-}"; src="${2:-}"; with_minio="${3:-}"
if [ -z "$src" ] || [ ! -f "$src/db.dump" ] || { [ "$mode" != "--verify" ] && [ "$mode" != "--yes" ]; }; then
  sed -n '2,13p' "$0" >&2
  exit 2
fi
src="$(cd "$src" && pwd)"

env_val() { grep "^$1=" .env | tail -1 | cut -d= -f2- || true; }
DB_USER="$(env_val DB_USER)"; DB_USER="${DB_USER:-nexus}"
DB_NAME="$(env_val DB_NAME)"; DB_NAME="${DB_NAME:-nexus}"
psql() { docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U "$DB_USER" "$@"; }

echo "==> Conferindo hashes"
(cd "$src" && sha256sum --quiet -c SHA256SUMS) && echo "  íntegro"

counts() {
  psql -d "$1" -At -c "SELECT 'users='||(SELECT count(*) FROM users)||' unidades='||(SELECT count(*) FROM unidades)||' processos='||(SELECT count(*) FROM tramite_processos)||' auditoria='||(SELECT count(*) FROM audit_logs)"
}

restore_into() {
  psql -d postgres -q -c "DROP DATABASE IF EXISTS \"$1\" WITH (FORCE)" -c "CREATE DATABASE \"$1\" OWNER \"$DB_USER\""
  docker compose exec -T postgres pg_restore -U "$DB_USER" -d "$1" --no-owner --exit-on-error <"$src/db.dump"
}

if [ "$mode" = "--verify" ]; then
  tmp="${DB_NAME}_restore_check"
  echo "==> Restaurando em banco temporário ($tmp)"
  restore_into "$tmp"
  echo "  backup: $(counts "$tmp")"
  echo "  atual:  $(counts "$DB_NAME")"
  psql -d postgres -q -c "DROP DATABASE \"$tmp\" WITH (FORCE)"
  echo "  objetos no backup do MinIO: $(find "$src/minio" -type f | wc -l)"
  echo "==> Verificação concluída (nada foi alterado)"
  exit 0
fi

echo "==> Parando API, worker e frontend"
docker compose stop backend-api backend-worker frontend
echo "==> Substituindo o banco $DB_NAME pelo backup"
restore_into "$DB_NAME"
echo "  $(counts "$DB_NAME")"
if [ "$with_minio" = "--minio" ]; then
  echo "==> Sobrepondo os objetos do MinIO"
  docker compose run --rm --no-deps -T -v "$src/minio:/in:ro" --entrypoint sh minio -c \
    'export MC_HOST_l="http://$MINIO_ROOT_USER:$MINIO_ROOT_PASSWORD@minio:9000"; mc mirror --quiet --overwrite /in l' >/dev/null
fi
echo "==> Subindo de novo"
docker compose up -d backend-api backend-worker frontend
echo "==> Restauração concluída"
