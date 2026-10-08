# 📖 Guia do Desenvolvedor — Plataforma Projeto Nexus

Bem-vindo ao guia de desenvolvimento e arquitetura do **Projeto Nexus**. Este documento foi elaborado para capacitar engenheiros de software, arquitetos e equipes de TI municipais a desenvolver, manter e expandir aplicações corporativas sobre a base governamental padronizada.

---

## 🏛️ 1. Visão Geral e Arquitetura

O **Projeto Nexus** é uma plataforma corporativa modular construída segundo o padrão de **Arquitetura Microkernel (Plug-in Architecture)** combinado a um **Monólito Modular & Clean Architecture**:

- **Core System / Kernel (`internal/platform/`)**: Infraestrutura central reutilizável que fornece os serviços fundamentais da plataforma:
  - **Kernel de plug-ins** (`internal/platform/kernel`): registro (`RegisterModule`), ciclo de vida em runtime, grafo de dependências, Guard HTTP, supervisão de workers/consumidores e tópicos WebSocket.
  - IAM: Keycloak dedicado (OIDC, federação LDAP/LDAPS com o Active Directory) e fallback local RS256.
  - Segurança HTTP (Headers OWASP, CSP Nonce, Rate Limiter).
  - Privacidade LGPD (Mascaramento PII em logs e consentimento).
  - Resiliência e Concorrência (Transactional Outbox, RabbitMQ com DLQ, Idempotência, Circuit Breaker).
  - Notificações em Tempo Real (WebSocket Hub com Auth via Tickets).
  - Auditoria Imutável (PostgreSQL Append-Only).
- **Plug-ins / Módulos de Negócio (`internal/modules/`)**: Componentes de domínio de negócio totalmente isolados e desacoplados:
  - Cada plug-in possui sua própria divisão Clean Architecture (`domain`, `application`, `infrastructure`, `transport`).
  - Declaram um **Manifest** (chave, dependências, permissões, ícone, rota) e são registrados no Kernel (`internal/app/modules.go`); a ativação/desativação acontece em runtime em **Configurações → Módulos** — rotas, workers, filas, WebSocket e busca acompanham na hora, sem reiniciar.
  - Nunca importam outro plug-in: colaborações (ex.: Trâmite → Signum) passam por portas ligadas em `internal/app` e a dependência é declarada em `DependsOn`.
- **Frontend (Next.js 16 App Router & React 19)**:
  - Componentes acessíveis em conformidade com o **e-MAG 2.0 / WCAG 2.1 AA**.
  - Estilização Tailwind CSS v4 com suporte White-Label dinâmico (`BrandingProvider`).
  - Suporte a acessibilidade nativa (VLibras, Alto Contraste, atalhos de teclado).

---

## ⚡ 2. Criando um Novo Módulo Corporativo

Para criar um novo módulo funcional (ex: *Módulo de Contratos* ou *Módulo de Protocolos*), utilize a automação do Makefile:

```bash
make new-module NAME=contratos
```

Este comando gera automaticamente:
1. A estrutura Clean Architecture Go em `backend/internal/modules/contratos/`.
2. A página React em `frontend/src/app/(protected)/contratos/page.tsx`.
3. A migration Goose de banco registrando a Feature Flag do novo módulo em `backend/migrations/`.

Após gerar o módulo, aplique a migration:
```bash
make migrate-up
```

---

## 🇧🇷 3. Normas de Conformidade Governamental

Todo código desenvolvido na plataforma deve obedecer estritamente aos 5 pilares de conformidade:

### 3.1. Acessibilidade Digital (e-MAG 2.0 / WCAG 2.1 AA)
- **Atalhos de Teclado**: Todo componente deve respeitar os marcadores de foco e âncoras e-MAG:
  - `Alt+1`: Conteúdo Principal (`#conteudo`)
  - `Alt+2`: Menu Principal (`#menu`)
  - `Alt+4`: Rodapé Institucional (`#rodape`)
- **Semântica HTML**: Utilize elementos semânticos (`<header>`, `<main>`, `<nav>`, `<footer>`) e atributos ARIA explicitados (`role="dialog"`, `aria-modal="true"`, `aria-label`).

### 3.2. Privacidade e LGPD (Lei 13.709/2018)
- **Mascaramento PII em Logs**: Nunca imprima CPF, e-mail ou telefone brutos em logs de aplicação. Utilize os envelopadores nativos do pacote `logging`:
  ```go
  logger.Info("registro acessado", slog.Any("cpf", logging.PIICPF(cpfBruto)))
  ```
- **Aceite de Termos**: O componente `<LGPDConsentModal />` verifica automaticamente o aceite formal dos termos pelo usuário e registra o evento auditável na tabela imutável `audit_logs`.

### 3.3. Identidade e autorização (Keycloak + AD, RBAC multi-escopo)
> Regras completas de estrutura, perfis, lotações e escopo (inclusive onde o
> escopo **não** restringe): [REGRAS_DE_NEGOCIO.md](REGRAS_DE_NEGOCIO.md).

- Autenticação por **Keycloak dedicado** (OIDC, tokens RS256 validados localmente via JWKS com cache e rotação) com federação do **Active Directory**; login local RS256 como contingência.
- O IAM resolve, a cada requisição (com cache invalidado entre réplicas), as **permissões efetivas** (`recurso:ação`, curingas `recurso:*` e `*`) a partir das **lotações** manuais e do **mapeamento de grupos do AD**, cada um num escopo (Entidade → Unidade → Departamento):
  ```go
  r.With(auth.RequirePermission(logger, "financeiro:manage")).Post("/financeiro/titulos", h.Create)

  identity, _ := auth.IdentityFromContext(ctx)
  if auth.HasPermission(identity, "financeiro:manage") { /* ... */ }
  ```
- **Ninguém concede o que não possui**: o IAM recusa (403) criar perfil, lotar ou mapear grupo do AD com permissões que o administrador não tem, e só quem tem acesso total concede o papel `nexus-admin`.

---

## 📑 4. Transparência e Lei de Acesso à Informação (LAI)

Para auditorias e relatórios de controle interno (CGU/TCE):
- Toda mutação no banco dispara entradas na tabela imutável `audit_logs`, com **ator, IP de origem e `correlation_id`** (`audit.FromRequest`). Há ainda uma **cópia WORM diária** para o object storage, encadeada por SHA-256 (`audit.WORMExporter`).
- Exportação da trilha (permissão `audit:read`): `GET /api/v1/audit/export?from=&to=&action=&format=&cursor=` — `format` ∈ `csv|json|xml` (ou header `Accept`), `from/to` em `AAAA-MM-DD` ou RFC3339 (default: últimos 30 dias, teto 366), paginação keyset via `cursor` (`X-Next-Cursor`). Cada exportação é ela mesma auditada (`audit.exported`).
- **Transparência ativa** (LAI art. 8º), rotas **públicas** em `/api/v1/transparencia/*` (JSON/CSV/XML): `datasets` (dicionário), `plataforma`, `integracoes`, `auditoria/acoes`. Datasets agregados, sem PII — o órgão define os datasets reais (`internal/platform/transparency`).
- **Direitos do titular (LGPD art. 18)**: `GET /api/v1/lgpd/meus-dados` (acesso + portabilidade), `POST /api/v1/lgpd/solicitar-exclusao` (anonimização pelo worker), `GET /api/v1/lgpd/minhas-solicitacoes`.

---

## 🧭 4.1. Convenções de conformidade (roadmap SGD/MGI)

Ver `docs/ROADMAP_CONFORMIDADE.md` e `docs/adr/005`. Vale para todo PR:

### Trilha de auditoria em toda mutação
Todo handler de POST/PUT/PATCH/DELETE constrói a entrada a partir de
`audit.FromRequest(r)` — nunca uma `audit.Entry{}` literal — para que
`ip_address` e `correlation_id` sejam sempre preenchidos (§49):

```go
entry := audit.FromRequest(r)
entry.Action = "modulo.recurso.acao"
entry.ResourceType = "recurso"
entry.ResourceID = id.String()
entry.Metadata = map[string]any{...} // NUNCA segredo/senha/token aqui
_ = h.audit.Record(r.Context(), entry)
```

Quando a mutação roda dentro de `database.WithTx`, use `audit.NewWriter(tx)`
para que a linha de auditoria commite/reverta junto (ver o blueprint
`internal/modules/example`).

### RBAC — sempre testar o caminho negativo
Toda rota nova sob `/api/v1` que não seja auto-serviço (`/me`) passa por
`auth.RequirePermission(...)`. O teste tem que cobrir o **403 sem a
permissão**, não só o 200 com ela.

### Entrada
`httputil.DecodeJSON` + `httputil.Validate` (struct-tags
`validate:"required,max=..."`). `json.NewDecoder` cru é proibido — não
aplica limite de corpo nem rejeita campo desconhecido.

### Migrations
Toda migration tem `-- +goose Down`. Exercite antes de abrir o PR:
`make migrate-redo` (up → down-to 0 → up). O CI roda o mesmo.

### SAST local
`make backend-sec` roda `govulncheck` + `staticcheck` + `gosec` — o mesmo
conjunto que bloqueia o CI.

### Parâmetros de hardening
`API_RATE_LIMIT_WINDOW_SECONDS`, `API_RATE_LIMIT_MAX`,
`METRICS_SCRAPE_TOKEN`, `TRUSTED_PROXIES`, `LOGIN_LOCKOUT_IP_THRESHOLD`
(backend), `TRUST_PROXY_HEADERS` (frontend) e `HOST_BIND` (compose) — ver
`.env.example`, a tabela em `docs/ROADMAP_CONFORMIDADE.md` e o
[ADR 011](adr/011-borda-unica-ip-confiavel-e-busca-por-prefixo.md).

### IP do visitante
A API só confia no `X-Forwarded-For` vindo de `TRUSTED_PROXIES`. Código do
frontend que chama a API **do lado do servidor** em nome do visitante (BFF,
login local, proxy público) repassa o IP com `forwardedFor()` de
`lib/api/clientIp.ts` — nunca copie o cabeçalho do cliente direto: sem
proxy de borda confiável ele é forjável.

### Busca full-text
Filtros de texto nos repositórios usam
`search @@ nexus_search_tsquery('<config>', $n)` (migration 000125), nunca
`websearch_to_tsquery` direto: a função soma a busca por prefixo ("vacina"
acha "vacinação") e mantém a sintaxe de operadores (aspas, `-termo`, `or`).

---

## 🛠️ 5. Comandos Úteis do Makefile

| Comando | Descrição |
| :--- | :--- |
| `make dev` | Sobe toda a stack (Postgres, RabbitMQ, MinIO, Go API, Frontend) em modo desenvolvimento |
| `make build` | Compila o backend Go e gera o bundle de produção do Next.js |
| `make test` | Executa 100% das suítes de teste unitário do backend e frontend |
| `make lint` | Executa auditoria estática de código e acessibilidade (`jsx-a11y`) |
| `make backend-sec` | SAST do backend: `govulncheck` + `staticcheck` + `gosec` (mesmo do CI) |
| `make migrate-up` | Executa as migrations de banco pendentes via Goose |
| `make migrate-redo` | Testa a reversibilidade: up → down-to 0 → up |
| `make seed-admin` | Cria ou reseta a conta do administrador local com senha segura |

---

## 🧪 6. Testes

A suíte cobre três níveis, todos rodando no CI (job de backend com Postgres 16 real):

| Nível | Onde | O que garante |
| :--- | :--- | :--- |
| Unidade | `*_test.go` junto do código; `frontend/src/**/*.test.ts(x)` | Regras de domínio, Kernel (grafo, cascata, réplicas), mascaramento PII, componentes e acessibilidade |
| Sistema (API real) | `backend/internal/app/*_http_test.go` | Cada módulo pela API completa (autenticação, IAM, Guard, auditoria, outbox): permissões (403), validação (422), fluxos felizes e de erro |
| Ativação de módulos | `internal/app/modules_toggle_test.go` | Para **cada** plug-in, todas as rotas descobertas automaticamente respondem `404 MODULE_DISABLED` quando desligado e voltam ao religar; dependências aplicadas nos dois sentidos |
| Contrato | `internal/app/openapi_test.go`, `openapi_schemas_test.go` | `docs/openapi.yaml` descreve exatamente as rotas montadas, e os schemas de corpo, resposta e query batem com os tipos Go de cada handler |
| Matriz de falhas | `internal/modules/<m>/application/faults_test.go` | Cada chamada ao repositório de cada caso de uso falha, ou envenena a transação logo depois, e o erro chega a quem chamou: nada é engolido, nada fica gravado pela metade |
| Repositório | `internal/modules/<m>/infrastructure/*_test.go` | Toda falha do banco (consulta, leitura de linha, `rows.Err`) é propagada, usando os fakes de `database/dbtest` |

**Meta: 100% de cobertura por pacote.** O CI roda `scripts/coverage-gate.sh` sobre o perfil do `go test`. Precisam estar em 100%: cada plug-in de `internal/modules/*`, cada pacote de `internal/platform/*` (menos os dublês `dbtest` e `storagetest`), `internal/app` e `pkg/*`. Quando algum não está, o script lista as linhas descobertas (ver ADRs 009 e 010).

Os testes de mensageria, armazenamento e composição rodam contra RabbitMQ e MinIO reais quando `TEST_RABBITMQ_URL` e `TEST_MINIO_ENDPOINT`/`TEST_MINIO_ACCESS_KEY`/`TEST_MINIO_SECRET_KEY` estão definidas; sem elas, são pulados. Para subir os dois localmente:

```bash
docker run -d --name nx-rabbit -p 5672:5672 -e RABBITMQ_DEFAULT_USER=nexus -e RABBITMQ_DEFAULT_PASS=nexus \
  -e RABBITMQ_DEFAULT_VHOST=nexus rabbitmq:3.13-management-alpine
docker run -d --name nx-minio -p 9000:9000 -e MINIO_ROOT_USER=nexusadmin -e MINIO_ROOT_PASSWORD=nexussecret123 \
  pgsty/minio:RELEASE.2026-08-04T00-00-00Z server /data
export TEST_RABBITMQ_URL=amqp://nexus:nexus@localhost:5672/nexus \
  TEST_MINIO_ENDPOINT=localhost:9000 TEST_MINIO_ACCESS_KEY=nexusadmin TEST_MINIO_SECRET_KEY=nexussecret123
```

Rodando localmente (os testes de integração pulam sem banco):

```bash
# Postgres de teste com as migrations
createdb nexus_test
DB_HOST=localhost DB_PORT=5432 DB_NAME=nexus_test DB_USER=nexus DB_PASSWORD=... go run ./cmd/migrate up

cd backend
TEST_DATABASE_URL="postgres://nexus:...@localhost:5432/nexus_test?sslmode=disable" \
  go test -race -p 1 -coverpkg=./internal/... ./...

# Meta de cobertura por pacote (a mesma do CI):
TEST_DATABASE_URL=... go test -p 1 -coverpkg=./internal/...,./pkg/... -coverprofile=coverage.out ./...
../scripts/coverage-gate.sh coverage.out

# Mudou a interface Repository de um plug-in? Regenere o decorador de falhas:
#   scripts/genfault.py (uso no cabeçalho do script)

# Novo endpoint ou DTO alterado? Regenere o contrato. As respostas de handlers
# que devolvem struct saem dos tipos Go (a description à mão é mantida); texto à
# mão só prevalece onde o handler devolve map ou negocia CSV/XML (ADR 010 §10.5):
UPDATE_OPENAPI=1 TEST_DATABASE_URL=... go test ./internal/app -run TestOpenAPIMatchesRouter

cd ../frontend && npm test

# Depois de regenerar o OpenAPI, atualize os tipos do contrato no frontend.
# O tsc confere cada tipo de src/lib/nexus/types.ts contra o que o backend envia:
npm run gen:api && npx tsc --noEmit
```

Convenções: todo endpoint novo ganha teste do caminho negativo (403 sem a permissão) e, se for mutação, confere a auditoria/outbox; todo plug-in novo entra automaticamente no teste de ativação.
