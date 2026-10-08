#!/usr/bin/env bash
# ==============================================================================
# Meta de cobertura por módulo (plugin) — roda no CI depois do `go test`.
#
# Lê o perfil gerado por `go test -coverpkg=./internal/...,./pkg/... -coverprofile=...`
# (vários pacotes de teste somados: o mesmo bloco aparece uma vez por binário,
# então vale a MAIOR contagem de cada bloco) e exige 100% de cobertura de
# instruções em cada diretório de internal/modules/<plugin>, em cada pacote de
# internal/platform/<pacote> (menos os dublês dbtest/storagetest) e em
# internal/app (composição) e em pkg/<pacote>. Lista as linhas descobertas quando falha.
#
# Uso: scripts/coverage-gate.sh backend/coverage.out [meta=100]
# ==============================================================================
set -euo pipefail

PROFILE=${1:?uso: $0 <coverage.out> [meta]}
TARGET=${2:-100}

awk -v target="$TARGET" '
NR == 1 && /^mode:/ { next }
{
    key = $1; stmts = $2; count = $3
    if (!(key in n) || count > c[key]) c[key] = count
    n[key] = stmts
}
END {
    for (key in n) {
        file = key; sub(/:.*/, "", file)
        if (match(file, /internal\/modules\/[^\/]+\//)) {
            mod = substr(file, RSTART + 17, RLENGTH - 18)
        } else if (match(file, /internal\/platform\/[^\/]+\//)) {
            mod = "platform/" substr(file, RSTART + 18, RLENGTH - 19)
            # dublês de teste (fakes de banco/armazenamento), não produção
            if (file ~ /\/(dbtest|storagetest)\//) continue
        } else if (file ~ /internal\/app\//) {
            mod = "app"
        } else if (match(file, /\/pkg\/[^\/]+\//)) {
            mod = substr(file, RSTART + 1, RLENGTH - 2)
        } else {
            continue
        }
        total[mod] += n[key]
        if (c[key] > 0) {
            covered[mod] += n[key]
        } else if (n[key] > 0) {
            line = key; sub(/^[^:]*:/, "", line); sub(/\..*/, "", line)
            short = file; sub(/.*internal\//, "", short)
            miss[mod] = miss[mod] "\n      " short ":" line
        }
    }
    fail = 0
    printf "%-30s %10s\n", "módulo", "cobertura"
    for (mod in total) {
        pct = 100 * covered[mod] / total[mod]
        printf "%-30s %9.1f%%\n", mod, pct
        if (pct + 1e-9 < target) {
            fail = 1
            printf "    ✗ abaixo da meta de %s%% — linhas descobertas:%s\n", target, miss[mod]
        }
    }
    if (fail) exit 1
    printf "\n✓ todos os módulos em %s%%\n", target
}' "$PROFILE"
