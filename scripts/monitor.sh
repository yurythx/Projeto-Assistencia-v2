#!/usr/bin/env bash
# Monitor da produção: roda a cada 5 minutos (scripts/monitor.sh
# --install-cron) e avisa quando algo QUEBRA ou VOLTA — nunca repete o mesmo
# alerta enquanto o estado não muda.
#
# Verifica:
#   api        GET /health da API responde "ok"
#   containers todo serviço do compose está healthy (ou rodando, sem healthcheck)
#   disco      uso da partição raiz abaixo de DISK_LIMIT (padrão 80%)
#   backup     o último backup (scripts/backup.sh) tem menos de BACKUP_MAX_HOURS (padrão 26h)
#
# Canal do aviso, em /etc/default/assistencia-monitor (o Ansible grava a partir
# do inventário). Sem nenhum, o alerta vai só para o syslog (tag
# assistencia-monitor) — e ninguém é avisado:
#   ALERT_WEBHOOK_URL   POST JSON {"text": "..."} — Slack, Google Chat, Teams
#                       (Incoming Webhook), Discord (/slack), Mattermost
#   ALERT_TELEGRAM_TOKEN + ALERT_TELEGRAM_CHAT   bot do Telegram
#
#   scripts/monitor.sh                 # verifica agora (e avisa se mudou)
#   scripts/monitor.sh --status        # só mostra o estado, sem avisar
#   scripts/monitor.sh --install-cron  # agenda a cada 5 minutos
set -uo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"
# shellcheck disable=SC1091
[ -f /etc/default/assistencia-monitor ] && . /etc/default/assistencia-monitor

API_URL="${API_URL:-http://127.0.0.1:8010/health}"
DISK_LIMIT="${DISK_LIMIT:-80}"
BACKUP_DIR="${BACKUP_DIR:-/root/backups/nexus}"
BACKUP_MAX_HOURS="${BACKUP_MAX_HOURS:-26}"
STATE_FILE="${STATE_FILE:-/var/lib/assistencia-monitor/estado}"
HOST_LABEL="${HOST_LABEL:-$(hostname)}"

if [ "${1:-}" = "--install-cron" ]; then
  cat >/etc/cron.d/assistencia-monitor <<EOF
# Monitor do Projeto-Assistencia-v2 (scripts/monitor.sh --install-cron)
SHELL=/bin/bash
*/5 * * * * root $ROOT/scripts/monitor.sh >/dev/null 2>&1
EOF
  chmod 644 /etc/cron.d/assistencia-monitor
  echo "Agendado: /etc/cron.d/assistencia-monitor (a cada 5 minutos)"
  exit 0
fi

# Cada verificação imprime "ok" ou o motivo da falha.
check_api() {
  local corpo
  corpo="$(curl -s -m 10 "$API_URL" 2>/dev/null)" || { echo "API não responde em $API_URL"; return; }
  if printf '%s' "$corpo" | grep -q '"status":"ok"'; then echo ok; else echo "API respondeu sem status ok"; fi
}

check_containers() {
  local ruins
  # Serviços de uma vez só (migrate, *-init) terminam de propósito: só contam
  # se terminarem com erro.
  ruins="$(docker compose ps -a --format '{{.Service}} {{.State}} {{.ExitCode}} {{.Health}}' 2>/dev/null |
    awk '{umavez = ($1 == "migrate" || $1 ~ /-init$/)}
         umavez && $2 == "exited" && $3 == "0" {next}
         $2 != "running" || ($4 != "" && $4 != "healthy") {print $1 "(" $2 ($4 != "" ? "/" $4 : "") ")"}' |
    tr '\n' ' ')"
  if [ -n "$ruins" ]; then echo "containers com problema: $ruins"; else echo ok; fi
}

check_disco() {
  local uso
  uso="$(df --output=pcent / | tail -1 | tr -dc '0-9')"
  if [ "${uso:-0}" -ge "$DISK_LIMIT" ]; then echo "disco em ${uso}% (limite ${DISK_LIMIT}%)"; else echo ok; fi
}

check_backup() {
  local ultimo idade
  ultimo="$(find "$BACKUP_DIR" -mindepth 1 -maxdepth 1 -type d ! -name '*.partial' -printf '%T@\n' 2>/dev/null | sort -n | tail -1)"
  if [ -z "$ultimo" ]; then echo "nenhum backup em $BACKUP_DIR"; return; fi
  idade=$(( ($(date +%s) - ${ultimo%.*}) / 3600 ))
  if [ "$idade" -ge "$BACKUP_MAX_HOURS" ]; then echo "último backup tem ${idade}h (limite ${BACKUP_MAX_HOURS}h)"; else echo ok; fi
}

enviar() {
  local texto="$1"
  logger -t assistencia-monitor "$texto" 2>/dev/null || true
  if [ -n "${ALERT_WEBHOOK_URL:-}" ]; then
    curl -s -m 15 -H 'Content-Type: application/json' \
      -d "$(printf '{"text":%s}' "$(printf '%s' "$texto" | python3 -c 'import json,sys;print(json.dumps(sys.stdin.read()))')")" \
      "$ALERT_WEBHOOK_URL" >/dev/null || logger -t assistencia-monitor "falha ao enviar o webhook"
  fi
  if [ -n "${ALERT_TELEGRAM_TOKEN:-}" ] && [ -n "${ALERT_TELEGRAM_CHAT:-}" ]; then
    curl -s -m 15 "https://api.telegram.org/bot${ALERT_TELEGRAM_TOKEN}/sendMessage" \
      --data-urlencode "chat_id=${ALERT_TELEGRAM_CHAT}" --data-urlencode "text=${texto}" >/dev/null ||
      logger -t assistencia-monitor "falha ao enviar para o Telegram"
  fi
}

mkdir -p "$(dirname "$STATE_FILE")"
touch "$STATE_FILE"
novo_estado=""
mensagens=()
for nome in api containers disco backup; do
  resultado="$("check_$nome")"
  novo_estado+="$nome=$resultado"$'\n'
  anterior="$(grep "^$nome=" "$STATE_FILE" | head -1 | cut -d= -f2-)"
  if [ "${1:-}" = "--status" ]; then
    printf '%-10s %s\n' "$nome" "$resultado"
    continue
  fi
  # Só avisa na mudança: ok -> falha (ALERTA) e falha -> ok (RESOLVIDO).
  if [ "$resultado" != "ok" ] && [ "$resultado" != "$anterior" ]; then
    mensagens+=("ALERTA [$HOST_LABEL] $nome: $resultado")
  elif [ "$resultado" = "ok" ] && [ -n "$anterior" ] && [ "$anterior" != "ok" ]; then
    mensagens+=("RESOLVIDO [$HOST_LABEL] $nome: voltou ao normal")
  fi
done
[ "${1:-}" = "--status" ] && exit 0

printf '%s' "$novo_estado" >"$STATE_FILE"
for m in "${mensagens[@]}"; do enviar "$m"; done
exit 0
