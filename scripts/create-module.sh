#!/usr/bin/env bash
# ==============================================================================
# Scaffolding de um novo plug-in do Projeto-Assistencia-v2 (Microkernel).
#
# Gera o módulo a partir do blueprint "example" — o mesmo código que roda em
# produção, então o esqueleto já nasce compilando e seguindo as regras da
# plataforma:
#   - Clean Architecture (domain / application / infrastructure / transport);
#   - kernel.Plugin com Manifest (chave, permissões, ícone, rota) — o módulo
#     é ativado/desativado em runtime em Configurações > Módulos;
#   - Transactional Outbox + auditoria na MESMA transação do negócio;
#   - queries parametrizadas (pgx) e autorização "recurso:ação" no middleware.
#
# Uso:     ./scripts/create-module.sh <nome>        (ou: make new-module NAME=<nome>)
# Exemplo: ./scripts/create-module.sh contratos
# ==============================================================================

set -euo pipefail

if [ $# -lt 1 ]; then
    echo "Uso: $0 <nome-do-modulo>   (letras minúsculas, dígitos e _; ex.: contratos)" >&2
    exit 1
fi

NAME=$(echo "$1" | tr '[:upper:]' '[:lower:]' | tr '-' '_')
if ! [[ "$NAME" =~ ^[a-z][a-z0-9_]{1,30}$ ]]; then
    echo "Erro: nome inválido '$NAME' — use letras minúsculas, dígitos e _ (começando por letra)." >&2
    exit 1
fi
TITLE="$(tr '[:lower:]' '[:upper:]' <<< "${NAME:0:1}")${NAME:1}"
TITLE="${TITLE//_/ }"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="${ROOT}/backend/internal/modules/example"
DST="${ROOT}/backend/internal/modules/${NAME}"
MODULES_GO="${ROOT}/backend/internal/app/modules.go"
MIGRATIONS="${ROOT}/backend/migrations"
PAGE_SRC="${ROOT}/frontend/src/app/(protected)/exemplos/page.tsx"
PAGE_DIR="${ROOT}/frontend/src/app/(protected)/${NAME}"
PROXY="${ROOT}/frontend/src/proxy.ts"

for reserved in example iam audit auditoria kernel platform app; do
    if [ "$NAME" = "$reserved" ]; then
        echo "Erro: '$NAME' é um nome reservado." >&2
        exit 1
    fi
done
if [ -e "$DST" ] || [ -e "$PAGE_DIR" ]; then
    echo "Erro: o módulo '$NAME' já existe (${DST} ou ${PAGE_DIR})." >&2
    exit 1
fi

echo "🚀 Gerando o plug-in '${NAME}' a partir do blueprint 'example'…"

# Substituições do blueprint -> novo módulo (ordem importa: mais específicas antes).
rename() {
    sed -i \
        -e "s#internal/modules/example#internal/modules/${NAME}#g" \
        -e "s#^package example\$#package ${NAME}#" \
        -e "s#auth\.PermExampleManage#auth.Permission(\"${NAME}:manage\")#g" \
        -e "s#example:manage#${NAME}:manage#g" \
        -e "s#nexus\.example#nexus.${NAME}#g" \
        -e "s#example_items#${NAME}_items#g" \
        -e "s#example_item#${NAME}_item#g" \
        -e "s#example\.item\.#${NAME}.item.#g" \
        -e "s#\"example\\.\([A-Z]\)#\"${NAME}.\1#g" \
        -e "s#\"example: #\"${NAME}: #g" \
        -e "s#live example module#live ${NAME} module#g" \
        -e "s#Key = \"example\"#Key = \"${NAME}\"#" \
        -e "s#\"/examples\"#\"/${NAME}\"#g" \
        -e "s#v1/examples#v1/${NAME}#g" \
        -e "s#\"/exemplos\"#\"/${NAME}\"#g" \
        -e "s#Name:           \"Módulo Modelo\"#Name:           \"${TITLE}\"#" \
        -e "s#Blueprint de referência para novos plugins (desativado por padrão).#Plug-in ${TITLE} (gerado pelo scaffolding).#" \
        -e "s#Criar itens do módulo-modelo#Gerenciar ${TITLE}#" \
        "$@"
}

# 1. Backend: cópia do blueprint com os nomes trocados.
cp -r "$SRC" "$DST"
find "$DST" -name '*.go' -print0 | while IFS= read -r -d '' f; do rename "$f"; done
sed -i -e "s#^// Package example é o módulo-modelo (blueprint) de um plugin do Nexus:#// Package ${NAME} é o plug-in ${TITLE} (gerado a partir do blueprint):#" "${DST}/module.go"

# 2. Migration (goose, numeração sequencial).
LAST=$(ls "$MIGRATIONS" | grep -E '^[0-9]{6}_.*\.sql$' | sort | tail -1 | cut -c1-6)
NEXT=$(printf '%06d' $((10#$LAST + 1)))
MIGRATION="${MIGRATIONS}/${NEXT}_${NAME}.sql"
cat > "$MIGRATION" <<EOF
-- +goose Up
-- Plug-in ${TITLE} (internal/modules/${NAME}) — gerado por scripts/create-module.sh.
CREATE TABLE ${NAME}_items (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title         TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'ACTIVE',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_${NAME}_items_created_at ON ${NAME}_items (created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS ${NAME}_items;
EOF

# 3. Registro no Kernel (internal/app/modules.go): import + MustRegister.
IMPORT_LINE="	\"github.com/yurythx/projeto-assistencia-v2/internal/modules/${NAME}\""
sed -i "s#^	\"github.com/yurythx/projeto-assistencia-v2/internal/modules/example\"\$#&\n${IMPORT_LINE}#" "$MODULES_GO"
sed -i "s#^		example.New(deps),\$#&\n		${NAME}.New(deps),#" "$MODULES_GO"
if ! grep -q "${NAME}.New(deps)" "$MODULES_GO"; then
    echo "⚠️  Não consegui registrar no Kernel automaticamente — adicione ${NAME}.New(deps) em ${MODULES_GO}." >&2
fi
if command -v gofmt >/dev/null 2>&1; then
    gofmt -w "$MODULES_GO" "$DST"
fi

# 4. Frontend: página do plug-in (mesmo kit do blueprint) + rota protegida.
mkdir -p "$PAGE_DIR"
cp "$PAGE_SRC" "${PAGE_DIR}/page.tsx"
rename "${PAGE_DIR}/page.tsx"
sed -i \
    -e "s#export default function ExemplosPage#export default function ${TITLE// /}Page#" \
    -e "s#eyebrow=\"Módulo-modelo\" title=\"Exemplo (blueprint)\"#eyebrow=\"Plug-in\" title=\"${TITLE}\"#" \
    "${PAGE_DIR}/page.tsx"
if ! grep -q "\"/${NAME}\"," "$PROXY"; then
    sed -i "s#^  \"/exemplos\",\$#&\n  \"/${NAME}\",#" "$PROXY"
fi

echo "✅ Plug-in '${NAME}' gerado:"
echo "   - Backend:   ${DST}"
echo "   - Migration: ${MIGRATION}"
echo "   - Kernel:    registrado em ${MODULES_GO}"
echo "   - Frontend:  ${PAGE_DIR}/page.tsx (rota protegida em src/proxy.ts)"
echo
echo "Próximos passos:"
echo "   1. Modele o domínio (domain/) e ajuste a migration antes de aplicá-la (make migrate-up)."
echo "   2. Declare as permissões reais no Manifest (module.go) — elas aparecem em Configurações > Perfis."
echo "   3. Ative o módulo em Configurações > Módulos (nasce desativado, DefaultEnabled=false)."
