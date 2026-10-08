# syntax=docker/dockerfile:1

# ---- deps ----
FROM node:22-bookworm-slim AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

# ---- builder ----
FROM node:22-bookworm-slim AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
# NEXT_PUBLIC_* vars are inlined into the client bundle at build time, not
# read at container startup — they MUST come in as build args, not just
# runtime `environment:` in docker-compose.yml.
ARG NEXT_PUBLIC_API_URL
ARG NEXT_PUBLIC_WS_URL
# NEXT_PUBLIC_MINIO_URL entra na CSP (img-src/connect-src, montada no
# proxy.ts, também inlinado no build) — sem este ARG o build de produção
# caía no fallback de lib/env.ts e a CSP bloqueava o PUT/GET direto nas
# URLs pré-assinadas do MinIO (upload/download de arquivos quebrado).
ARG NEXT_PUBLIC_MINIO_URL
ENV NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL} \
    NEXT_PUBLIC_WS_URL=${NEXT_PUBLIC_WS_URL} \
    NEXT_PUBLIC_MINIO_URL=${NEXT_PUBLIC_MINIO_URL}
# Auth/config env is only READ at request time by server code, so build
# placeholders are enough to satisfy the fail-fast checks in lib/auth —
# the real values come from docker-compose's runtime `environment:`.
ENV KEYCLOAK_ISSUER_URL=https://build-placeholder.invalid/realms/build \
    KEYCLOAK_FRONTEND_CLIENT_ID=build-placeholder \
    KEYCLOAK_FRONTEND_CLIENT_SECRET=build-placeholder \
    NEXTAUTH_SECRET=build-placeholder-please-override-at-runtime
# Next.js "standalone" output produces a minimal self-contained server.
RUN npm run build

# ---- runtime ----
FROM node:22-bookworm-slim AS runtime
WORKDIR /app
ENV NODE_ENV=production
# wget: exigido pelo HEALTHCHECK do docker-compose.yml (`wget --spider`) —
# node:22-bookworm-slim, ao contrário da imagem Alpine anterior (busybox
# embutido), não vem com wget/curl. Sem isto o container ficava
# permanentemente "unhealthy" mesmo servindo requisições normalmente.
RUN apt-get update && apt-get install -y --no-install-recommends wget && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd -r nix && useradd -r -g nix nix

COPY --from=builder --chown=nix:nix /app/public ./public
COPY --from=builder --chown=nix:nix /app/.next/standalone ./
COPY --from=builder --chown=nix:nix /app/.next/static ./.next/static

USER nix
EXPOSE 3000
ENV PORT=3000
# O server.js standalone do Next.js escuta em process.env.HOSTNAME, e o
# Docker injeta automaticamente HOSTNAME=<id do container> em todo
# container (reflete /etc/hostname) — sem sobrescrever isso, o servidor
# escuta só no IP específico do container na rede docker, não em todas as
# interfaces. Resultado: funciona through a porta publicada (o NAT do
# Docker ainda alcança esse IP), mas HEALTHCHECK/qualquer coisa rodando
# DENTRO do próprio container batendo em localhost:3000 recebe connection
# refused, e o container fica marcado "unhealthy" apesar de estar
# funcionando. "0.0.0.0" faz o server.js escutar em toda interface.
ENV HOSTNAME="0.0.0.0"
ENTRYPOINT ["node", "server.js"]
