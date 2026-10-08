#!/usr/bin/env bash
# Coloca a stack atrás do Caddy com HTTPS (CA interna) — idempotente.
#
#   scripts/enable-https.sh [host]    # padrão: o host de FRONTEND_URL
#
#   1. copia o .env para .env.bak-<data> e troca as URLs públicas para
#      https (443 frontend, 8443 API, 9443 MinIO, 8543 Keycloak);
#   2. TRUSTED_PROXIES = sub-rede da rede docker e HOST_BIND=127.0.0.1
#      (portas diretas fora da rede; o IP real do cliente
#      chega no X-Forwarded-For do Caddy);
#   3. sobe o Caddy, extrai a raiz da CA para secrets/ca/nexus-ca.crt;
#   4. scripts/deploy.sh (rebuild: o frontend embute as URLs no build);
#   5. Keycloak de teste: aponta as URLs de redirecionamento do client
#      nexus-frontend para o novo endereço (sem reimportar o realm).
#
# Depois: cada máquina de teste instala http://<host>/nexus-ca.crt como
# autoridade confiável (ver docs/DEPLOY.md).
set -euo pipefail

cd "$(dirname "$0")/.."
[ -f .env ] || { echo "Sem .env — rode scripts/deploy.sh <host> primeiro." >&2; exit 1; }

env_val() { grep "^$1=" .env | tail -1 | cut -d= -f2- || true; }
set_kv() {
  if grep -q "^$1=" .env; then sed -i "s|^$1=.*|$1=$2|" .env; else printf '%s=%s\n' "$1" "$2" >>.env; fi
}

host="${1:-$(env_val HTTPS_PUBLIC_HOST)}"
host="${host:-$(env_val FRONTEND_URL | sed -E 's#^https?://([^:/]+).*#\1#')}"
[ -n "$host" ] || { echo "Informe o host: scripts/enable-https.sh <ip-ou-nome>" >&2; exit 1; }

echo "==> .env -> https://$host (cópia em .env.bak-$(date +%Y%m%d-%H%M%S))"
cp -p .env ".env.bak-$(date +%Y%m%d-%H%M%S)"
set_kv HTTPS_PUBLIC_HOST "$host"
set_kv FRONTEND_URL "https://$host"
set_kv API_PUBLIC_URL "https://$host:8443"
# WebSocket na mesma origem do site (o Caddy encaminha /ws para a API).
set_kv WEBSOCKET_PUBLIC_URL "wss://$host/ws"
set_kv MINIO_PUBLIC_URL "https://$host:9443"
if grep -q "^KEYCLOAK_PUBLIC_URL=" .env; then
  set_kv KEYCLOAK_PUBLIC_URL "https://$host:8543"
  set_kv KEYCLOAK_ISSUER_URL "https://$host:8543/realms/nexus"
fi
project="$(docker compose config 2>/dev/null | sed -n 's/^name: //p' | head -1)"
subnet="$(docker network inspect "${project}_nexus_internal" --format '{{range .IPAM.Config}}{{.Subnet}}{{end}}' 2>/dev/null || true)"
[ -n "$subnet" ] && set_kv TRUSTED_PROXIES "$subnet"
# Caddy vira a ÚNICA entrada: as portas diretas (frontend, API, MinIO,
# Keycloak) só escutam no próprio servidor. Abertas na rede, elas deixavam
# o cliente forjar o X-Forwarded-For e escapar dos limites por IP.
set_kv HOST_BIND 127.0.0.1
files="$(env_val COMPOSE_FILE)"
files="${files:-docker-compose.yml}"
case ":$files:" in *":docker-compose.https.yml:"*) ;; *) set_kv COMPOSE_FILE "$files:docker-compose.https.yml" ;; esac

echo "==> Subindo o Caddy e extraindo a raiz da CA"
docker compose up -d caddy
mkdir -p secrets/ca
for _ in $(seq 1 30); do
  docker compose cp caddy:/data/caddy/pki/authorities/local/root.crt secrets/ca/nexus-ca.crt >/dev/null 2>&1 && break
  sleep 2
done
[ -s secrets/ca/nexus-ca.crt ] || { echo "Caddy não gerou a CA — veja: docker compose logs caddy" >&2; exit 1; }
chmod 644 secrets/ca/nexus-ca.crt
openssl x509 -in secrets/ca/nexus-ca.crt -noout -subject -enddate | sed 's/^/  /'

./scripts/deploy.sh --no-pull "$host"

if docker compose ps --services --status running | grep -qx keycloak; then
  echo "==> Keycloak: redirecionamentos do client nexus-frontend -> https://$host"
  pw="$(env_val KEYCLOAK_ADMIN_PASSWORD)"
  docker compose exec -T keycloak bash -c "
    kc=/opt/keycloak/bin/kcadm.sh
    \$kc config credentials --server http://localhost:8080 --realm master --user admin --password '$pw' >/dev/null
    id=\$(\$kc get clients -r nexus -q clientId=nexus-frontend --fields id --format csv --noquotes)
    \$kc update clients/\$id -r nexus \
      -s 'redirectUris=[\"https://$host/*\"]' -s 'webOrigins=[\"https://$host\"]' \
      -s 'attributes.\"post.logout.redirect.uris\"=https://$host/*'"
fi

echo
echo "==> HTTPS ativo"
echo "  Nexus:       https://$host"
echo "  Instale a CA em cada máquina de teste: http://$host/nexus-ca.crt"
