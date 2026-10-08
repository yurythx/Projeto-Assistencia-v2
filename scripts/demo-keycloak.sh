#!/usr/bin/env bash
# Liga o ambiente de TESTE com Keycloak próprio e dados fictícios:
#   1. completa o .env (URL e senha de admin do Keycloak, senha dos
#      usuários fictícios, COMPOSE_FILE com docker-compose.keycloak.yml);
#   2. roda scripts/deploy.sh (build + sobe tudo + espera healthy);
#   3. aplica deploy/demo/seed-demo.sql (idempotente).
#
#   scripts/demo-keycloak.sh [--no-pull]
#
# Pré-requisito: .env já gerado por scripts/deploy.sh (1ª execução).
# Idempotente: valores já presentes no .env são mantidos.
set -euo pipefail

cd "$(dirname "$0")/.."

if [ ! -f .env ]; then
  echo "Sem .env — rode scripts/deploy.sh <host> primeiro." >&2
  exit 1
fi

env_val() { grep "^$1=" .env | tail -1 | cut -d= -f2- || true; }  # chave ausente = vazio (grep sem match não pode derrubar o set -e)
set_if_missing() {
  if [ -z "$(env_val "$1")" ]; then
    if grep -q "^$1=" .env; then
      sed -i "s|^$1=.*|$1=$2|" .env
    else
      printf '%s=%s\n' "$1" "$2" >>.env
    fi
    echo "  .env: $1 definido"
  fi
}

frontend_url="$(env_val FRONTEND_URL)"
host="$(printf '%s' "$frontend_url" | sed -E 's#^https?://([^:/]+).*#\1#')"
kc_port="$(env_val HOST_KEYCLOAK_PORT)"
kc_port="${kc_port:-8180}"

echo "==> Configurando o Keycloak de teste no .env"
set_if_missing HOST_KEYCLOAK_PORT "$kc_port"
set_if_missing KEYCLOAK_PUBLIC_URL "http://$host:$kc_port"
set_if_missing KEYCLOAK_ISSUER_URL "http://$host:$kc_port/realms/nexus"
set_if_missing KEYCLOAK_ADMIN_PASSWORD "$(openssl rand -hex 16)"
# Secret do client nexus-backend (reautenticação do Signum no Keycloak).
set_if_missing KEYCLOAK_CLIENT_SECRET "$(openssl rand -hex 32)"
# Senha comum a todos os usuários fictícios (legível, para digitar nos testes).
set_if_missing DEMO_USER_PASSWORD "Teste-$(openssl rand -hex 3)"
set_if_missing COMPOSE_FILE "docker-compose.yml:docker-compose.keycloak.yml"

./scripts/deploy.sh "$@"

echo
echo "==> Aplicando os dados fictícios (estrutura organizacional + usuários)"
docker compose exec -T postgres psql -q -v ON_ERROR_STOP=1 \
  -U "$(env_val DB_USER)" -d "$(env_val DB_NAME)" <deploy/demo/seed-demo.sql

echo
echo "==> Keycloak de teste pronto"
echo "  Login no Nexus:    $frontend_url/login  (botão do Keycloak)"
echo "  Usuários:          deploy/demo/usuarios.csv (ex.: teste.admin)"
echo "  Senha dos usuários: DEMO_USER_PASSWORD no .env"
echo "  Console Keycloak:  $(env_val KEYCLOAK_PUBLIC_URL)/admin  (admin / KEYCLOAK_ADMIN_PASSWORD no .env)"
