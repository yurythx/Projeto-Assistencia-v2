# 005 — Roadmap de conformidade governamental (auditoria SGD/MGI 2026-09)

- **Status:** Em execução
- **Data:** 2026-09-09
- **Autores:** Engenharia Projeto Nexus

## Contexto

Uma auditoria de conformidade (e-MAG 2.0 / WCAG 2.1 AA, LGPD 13.709/2018,
LAI 12.527/2011 + LC 131/2009, OWASP ASVS, SGD/MGI, RFC 7807) varreu todo
o código-fonte e identificou 16 gaps (G-01 a G-16) mais um conjunto de
capacidades ausentes. A fundação da plataforma é sólida (auditoria
imutável no banco, CSP com nonce, BFF sem token no browser, bcrypt +
lockout timing-safe, SQL 100% parametrizado); os gaps são de **completude
e minimização**, não de arquitetura.

O roadmap completo (6 fases, ~29 iniciativas) está em
`docs/ROADMAP_CONFORMIDADE.md`. Este ADR registra as **decisões de
arquitetura** que a execução exige.

## Decisão

### 5.1. Rate limiting global em `/api/v1` (G-01)

- Novo `RateLimiters.APIGlobal` (`ratelimit.PostgresLimiter`, bucket
  `api_global`), aplicado como middleware a todo o grupo `/api/v1` com
  chave = `identity.Subject` (fallback: `httpserver.ClientIPKey`).
- Reusa a infraestrutura de janela fixa em Postgres (sem Redis, §7).
- Parametrizável: `API_RATE_LIMIT_WINDOW_SECONDS` (60),
  `API_RATE_LIMIT_MAX` (600). *Fail-open* mantido (uma instabilidade do
  limiter não pode virar indisponibilidade do endpoint).

### 5.2. `/metrics` autenticável (G-02)

- `httpserver.Options.MetricsToken`: quando não vazio, `/metrics` exige
  `Authorization: Bearer <METRICS_SCRAPE_TOKEN>` com comparação em tempo
  constante. Vazio preserva o comportamento aberto (dev / rede interna).
- Alternativa operacional recomendada em produção: publicar a porta de
  métricas só na rede interna do orquestrador.

### 5.3. IP + correlação em toda `audit.Entry` de mutação (G-07, G-08)

- Novo helper `audit.FromRequest(*http.Request) audit.Entry` — preenche
  `IPAddress` (via `r.RemoteAddr`, `sanitizeIP` no `Record`) e
  `CorrelationID` (de `logging.CorrelationID`). Todo handler de mutação
  passa a construir a entrada a partir dele.
- Novo `POST /api/v1/auth/logout` (dentro do grupo autenticado) grava
  `audit.ActionLogout`. Não invalida o token (stateless).
- Blueprint `example`: a linha de auditoria é gravada **na mesma
  transação** do INSERT + outbox (`audit.NewWriter(tx)`).

### 5.4. Minimização de PII no diretório de usuários (G-10)

- `RoleUser` perde `PermUsersRead`. Um usuário comum enxerga só a si
  mesmo via `GET /api/v1/me` (que exige apenas autenticação).
- `GET /api/v1/users` passa a devolver `UserListItem` (sem `email` nem
  `last_seen_at`). O e-mail completo só aparece em `GET /users/{id}`
  (nexus-auditor / nexus-admin) e em `/me` (o próprio titular).
- **Mudança de comportamento:** um `nexus-user` puro deixa de listar o
  diretório e, com G-05, deixa de ver as integrações. Confirmado como
  desejável (menor privilégio); revertível num único commit se o produto
  discordar.

### 5.5. Hardening HTTP no backend (G-03, G-04, G-14)

- `SecurityHeaders`: `Content-Security-Policy: default-src 'none';
  frame-ancestors 'none'; base-uri 'none'` e HSTS **incondicionais**
  (antes CSP inexistente; HSTS só com `r.TLS != nil`, nil atrás de proxy
  TLS). `/docs` sobrescreve a CSP para o Swagger UI.
- `httpserver.ClientIP(r, trustedProxies)` — só honra `X-Forwarded-For`
  quando `RemoteAddr` ∈ `TRUSTED_PROXIES` (CIDRs validados no boot).
  `lgpd.handleAccept` deixa de ler o header cru.
- Aliases `/livez`, `/healthz`, `/readyz` (nomes canônicos k8s).

### 5.6. Fases 3–5 — implementado neste ciclo

- **RFC 7807 (G-13):** resolvido pela Opção B do ADR 006 (negociação de
  conteúdo por `Accept`, sem quebra) — `httputil.WriteError`.
- **Exceção CSP `style-src 'unsafe-inline'` (G-16):** ADR 007.
- **Direitos do titular (LGPD art. 18):** migrations `000005`/`000006`,
  endpoints em `internal/platform/lgpd` (`/meus-dados`,
  `/solicitar-exclusao`, `/minhas-solicitacoes`, `/accept-anon` público)
  e worker `lgpd.ErasureProcessor` (anonimização — nunca DELETE).
- **Transparência ativa (LAI art. 8º, LC 131):**
  `internal/platform/transparency` — rotas públicas JSON/CSV/XML com
  datasets agregados (scaffold; o órgão define os datasets reais).
- **Export WORM da auditoria (F2.6):** `audit.WORMExporter` +
  `000007_audit_worm_exports`; o object-lock do bucket fica na infra.
- **Página de Privacidade (F3.1) e Declaração de Acessibilidade (F4.5):**
  `/privacidade` e `/acessibilidade` — publicadas como minutas técnicas,
  aguardando homologação do DPO / avaliação e-MAG externa.
- **RIPD (F5.3) e Dossiê (F5.4):** `docs/RIPD_MODELO.md`,
  `docs/DOSSIE_CONFORMIDADE.md` — insumo técnico; o RIPD oficial é do DPO.
- **Ainda dependem de terceiro:** pentest externo (F5.1) e avaliação
  e-MAG por avaliador humano com tecnologia assistiva (F5.2).

## Consequências

- **Positivas:** os três riscos de maior severidade (DoS autenticado,
  PII entre titulares, trilha incompleta em operação crítica) fechados
  com mudanças localizadas; blueprint volta a ser um bom exemplo.
- **Custos:** mais uma linha em `rate_limit_buckets` por identidade
  ativa; `nexus-user` puro perde acesso a listagens (ver 5.4);
  `TRUSTED_PROXIES` precisa ser configurado corretamente em cada
  ambiente com proxy reverso, senão o IP real do cliente não é
  registrado.
- **Migrations relacionadas:** `000005_data_subject_requests.sql`.
