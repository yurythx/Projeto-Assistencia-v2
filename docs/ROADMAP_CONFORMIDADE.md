# Roadmap de Conformidade Governamental — Projeto Nexus

Origem: auditoria de conformidade de **2026-09-09** (e-MAG 2.0 / WCAG 2.1
AA, LGPD 13.709/2018, LAI 12.527/2011 + LC 131/2009, OWASP ASVS, SGD/MGI,
RFC 7807). Decisões de arquitetura em [`docs/adr/005`](adr/005-roadmap-conformidade-governamental.md),
[`006`](adr/006-rfc7807-problem-details.md), [`007`](adr/007-excecao-csp-style-src-vlibras.md).

Legenda: ✅ feito · 🟡 parcial / scaffold · ⬜ pendente · 🔒 depende de insumo externo

---

## Fase 0 — Fundação de processo

| # | Item | Estado |
|---|---|---|
| F0.1 | ADR obrigatório para contrato de API / RBAC / auditoria / middleware | ✅ ADRs 005–007; checklist no template de PR |
| F0.2 | Gates de CI: `govulncheck` + `staticcheck` + `gosec` (bloqueante) + `go vet` + `go test -race -p 1` | ✅ `.github/workflows/ci.yml`, `make backend-sec` |
| F0.3 | Checklist de conformidade no template de PR | ✅ `.github/pull_request_template.md` |
| F0.4 | Convenções de teste + migration (`up→down→up` no CI) | ✅ job `migrations-reversibility`, `make migrate-redo` |

## Fase 1 — Contenção dos riscos altos

| # | Gap | Item | Estado |
|---|---|---|---|
| F1.1 | G-01 | Rate limit global por identidade em `/api/v1` | ✅ `RateLimiters.APIGlobal`, env `API_RATE_LIMIT_*` |
| F1.2 | G-02 | `/metrics` exige bearer quando `METRICS_SCRAPE_TOKEN` definido | ✅ `requireMetricsToken` (tempo constante) |
| F1.3 | G-10 | Minimização de PII no diretório de usuários | ✅ `RoleUser` sem `PermUsersRead`; `UserListItem` sem e-mail — **breaking: confirmar com o produto** (ADR 005 §5.4) |
| F1.4 | G-07 | IP + `correlation_id` em toda `audit.Entry` de mutação | ✅ `audit.FromRequest` |
| F1.5 | G-08 | `logout` auditado | ✅ `POST /api/v1/auth/logout` + chamada no `signOut` do frontend |

## Fase 2 — Hardening HTTP e integridade da trilha

| # | Gap | Item | Estado |
|---|---|---|---|
| F2.1 | G-03 | CSP + HSTS incondicionais no backend + Swagger UI local | ✅ `SecurityHeaders`; ✅ `/docs` serve `swagger-ui-dist@5.17.14` vendorado (sem CDN, CSP `script-src 'self'`) |
| F2.2 | G-04 | Cadeia `X-Forwarded-For` confiável (`TRUSTED_PROXIES`) | ✅ `httpserver.ClientIP` |
| F2.3 | G-09 | Auto-auditoria do acesso à exportação LAI | ✅ `audit.exported` gravado a cada exportação |
| F2.4 | G-05 | Permissão `integrations:read` nas rotas de integrations | ✅ |
| F2.5 | G-06 | Padronizar o blueprint `example` | ✅ decode + validate + auditoria na tx |
| F2.6 | — | Export WORM da auditoria | ✅ `audit.WORMExporter` — bucket dedicado com **S3 Object Lock** (Compliance, `AUDIT_WORM_RETENTION_DAYS`) + cadeia de SHA-256 + migration 000007. Verificado: versão travada não pode ser sobrescrita/apagada |

## Fase 3 — Direitos do titular e transparência ativa

| # | Gap | Item | Estado |
|---|---|---|---|
| F3.1 | — | Página de Política de Privacidade + Termos versionada | 🟡 `/privacidade` publicada como **minuta**; 🔒 homologação do DPO |
| F3.2 | — | Endpoints LGPD art. 18 (`/lgpd/meus-dados`, `/solicitar-exclusao`, `/minhas-solicitacoes`) + worker de anonimização | ✅ migration 000005 + `lgpd.ErasureProcessor` |
| F3.3 | G-11 | Consentimento de visitante não autenticado | ✅ migration 000006 + `POST /lgpd/accept-anon` (público) + modal usa `useSession` |
| F3.4 | G-12 | Exportação de auditoria completa (`from/to`, paginação, JSON/XML) | ✅ |
| F3.5 | — | Endpoints de transparência ativa / dados abertos | 🟡 `internal/platform/transparency` (scaffold: datasets agregados sem PII); o órgão define os datasets reais |

## Fase 4 — Interoperabilidade e conformidade formal

| # | Gap | Item | Estado |
|---|---|---|---|
| F4.1 | G-13 | Respostas RFC 7807 (`application/problem+json`) | ✅ negociação por `Accept` (ADR 006, opção B); ✅ documentado no `openapi.yaml` 1.2.0 |
| F4.2 | G-14 | Aliases `/livez` `/healthz` `/readyz` | ✅ |
| F4.3 | G-15 | Skip links / landmarks / teclas de acesso | ✅ verificado — `#conteudo/#menu/#rodape` presentes e focáveis nos dois shells; `accessKey` 1–4 segue a convenção e-MAG (documentado na Declaração de Acessibilidade) |
| F4.4 | G-16 | Exceção de CSP `style-src 'unsafe-inline'` registrada | ✅ ADR 007 |
| F4.5 | — | Declaração formal de Acessibilidade + VPAT | ✅ seção "Declaração de Acessibilidade" em `/acessibilidade`; 🟡 `docs/VPAT.md` (autoavaliação WCAG 2.1 A/AA) — homologação depende de F5.2 |

## Fase 5 — Verificação externa e evidências

| # | Item | Estado |
|---|---|---|
| F5.1 | Pentest externo | 🔒 contratação externa — checklist em `docs/DOSSIE_CONFORMIDADE.md` §5 |
| F5.2 | Avaliação e-MAG por avaliador (ASES + manual + T.A.) | 🔒 avaliação externa |
| F5.3 | RIPD / DPIA | 🟡 `docs/RIPD_MODELO.md` (minuta técnica); 🔒 versão oficial pelo DPO |
| F5.4 | Dossiê de conformidade SGD/MGI | ✅ `docs/DOSSIE_CONFORMIDADE.md` (índice); itens de execução marcados nele |

## Fase 6 — Revisão do Microkernel (ADR 008)

| # | Item | Estado |
|---|---|---|
| F6.1 | Grafo de módulos (`depends_on`/`dependents`/`blocked_by`) na API e na tela de Módulos, cascata confirmada | ✅ |
| F6.2 | Teste automático de ativação: toda rota de cada plug-in responde 404 `MODULE_DISABLED` desligado | ✅ `internal/app/modules_toggle_test.go` |
| F6.3 | A01 — IAM: ninguém concede permissão/papel que não possui | ✅ |
| F6.4 | Estrutura organizacional e perfis com `ON DELETE RESTRICT` (409 em uso) | ✅ migration 000121 |
| F6.5 | Auditoria com IP e User-Agent em toda entrada (proveniência skill §5) | ✅ `audit.CaptureOrigin` |
| F6.6 | LGPD — consentimento de usuários federados e versão vigente obrigatória | ✅ |
| F6.7 | Testes de integração no CI (Postgres real), cobertura 46,5% → 70% | ✅ |
| F6.8 | OpenAPI sincronizado com o roteador (teste de contrato) | ✅ `internal/app/openapi_test.go` |

## Fase 7 — Revisão das regras de negócio dos plug-ins (ADR 009)

| # | Item | Estado |
|---|---|---|
| F7.1 | Cada plug-in revisado contra o fluxo de trabalho para o qual foi criado; regras não aplicadas corrigidas (ver ADR 009 §9.1) | ✅ |
| F7.2 | A01 — IAM: só administra uma conta quem cobre as permissões efetivas dela; conta federada sem senha local | ✅ |
| F7.3 | Estados finais imutáveis (processo concluído, evento cancelado, sala arquivada) e transição idempotente sem nova auditoria | ✅ |
| F7.4 | Erro de banco nunca mascarado como regra de negócio; curingas do LIKE digitados tratados como texto | ✅ |
| F7.5 | Auditoria: User-Agent não derruba a transação; exportação LAI nunca incompleta; cópia WORM retoma da marca d'água sem quebrar a cadeia | ✅ |
| F7.6 | Matriz de falhas (falha e transação envenenada em cada chamada do repositório) em todos os plug-ins | ✅ `scripts/genfault.py`, `database/dbtest` |
| F7.7 | Meta de 100% de cobertura por plug-in no CI (cobertura total 70% → 84,7%) | ✅ `scripts/coverage-gate.sh` |

## Fase 8 — Revisão da plataforma (ADR 010)

| # | Item | Estado |
|---|---|---|
| F8.1 | Cada pacote de `internal/platform` e a composição (`internal/app`) revisados; correções em ADR 010 §10.1 | ✅ |
| F8.2 | Outbox com backoff por evento e reprocessamento auditado (`POST /monitoring/outbox/requeue`, `monitoring:manage`) | ✅ migration 000123 |
| F8.3 | LGPD art. 18: dados pessoais dos plug-ins (`PersonalData`) no pacote de exportação e na eliminação, com retentativas | ✅ migration 000124 |
| F8.4 | Anti-SSRF: formas IPv6 que embutem IPv4 bloqueadas | ✅ |
| F8.5 | e-MAG: contraste AA exigido nos tokens de cor do branding | ✅ |
| F8.6 | RabbitMQ e MinIO reais no CI; meta de 100% por pacote (plug-ins, plataforma, app, pkg) | ✅ `scripts/coverage-gate.sh` |
| F8.7 | Cópia WORM com uma única réplica exportando cada dia (advisory lock no Postgres) | ✅ |
| F8.8 | OpenAPI com schemas de corpo, resposta e query gerados dos tipos Go, verificados no CI | ✅ `internal/app/openapi_schemas_test.go` |
| F8.9 | Grafo visual de dependências na tela de Módulos | ✅ |
| F8.10 | Testes E2E (Playwright) dos fluxos críticos contra a stack completa, no CI | ✅ `frontend/e2e`, `docs/e2e.md` |
| F8.11 | Contrato frontend × backend verificado pelo compilador (tipos gerados do OpenAPI) | ✅ `frontend/src/lib/nexus/contract.types.ts` |
| F8.12 | `npm audit` bloqueante no CI; Next 16.3.6 (RCE crítica GHSA-2xp9-vwfh-vxw4) | ✅ |

---

## Parâmetros novos e como são usados

| Variável | Default | Efeito |
|---|---|---|
| `API_RATE_LIMIT_WINDOW_SECONDS` | `60` | Janela do rate limiter global de `/api/v1` |
| `API_RATE_LIMIT_MAX` | `600` | Requisições por identidade por janela; excedente → `429 RATE_LIMITED` |
| `METRICS_SCRAPE_TOKEN` | *(vazio)* | Vazio: `/metrics` aberto. Definido: exige `Authorization: Bearer <token>` |
| `TRUSTED_PROXIES` | *(vazio)* | CIDRs cujo `X-Forwarded-For` é confiável; fora deles, só `RemoteAddr` |
| `LOGIN_LOCKOUT_IP_THRESHOLD` | `30` | Falhas de login local por IP antes do bloqueio progressivo do IP (1 min dobrando até 1 h; a conta bloqueia com 5, até 24 h) |
| `TRUST_PROXY_HEADERS` (frontend) | *(vazio)* | `true` só atrás de proxy de borda que sobrescreve o `X-Forwarded-For`: o frontend passa a repassar o IP do visitante à API |
| `HOST_BIND` (compose) | `0.0.0.0` | Interface das portas publicadas; `127.0.0.1` com o proxy HTTPS como única entrada |
| `AUDIT_WORM_BUCKET` | `nexus-audit-worm` | Bucket dedicado (com Object Lock) da cópia WORM da auditoria |
| `AUDIT_WORM_RETENTION_DAYS` | `1825` | Retenção Compliance por objeto WORM (5 anos) |

Todos aceitam o padrão `<VAR>_FILE` (Docker/K8s secrets). Limiters
`anon_consent` (10/60s por IP) e `public_read` (60/60s por IP) são fixos.

## Endpoints novos

| Método | Rota | Auth | Descrição |
|---|---|---|---|
| POST | `/api/v1/auth/logout` | sim | audita o fim de sessão |
| GET | `/api/v1/lgpd/meus-dados` | sim | pacote JSON de dados do titular (art. 18) |
| POST | `/api/v1/lgpd/solicitar-exclusao` | sim | pedido de anonimização |
| GET | `/api/v1/lgpd/minhas-solicitacoes` | sim | acompanhamento |
| POST | `/api/v1/lgpd/accept-anon` | **não** | consentimento de visitante (rate-limited por IP) |
| GET | `/api/v1/transparencia/{datasets,plataforma,integracoes,auditoria/acoes}` | **não** | dados abertos (JSON/CSV/XML) |
| GET | `/api/v1/audit/export?from=&to=&action=&format=&cursor=` | `audit:read` | trilha LAI completa; auto-auditada |
| GET | `/livez` `/healthz` `/readyz` | não | aliases de sonda |
