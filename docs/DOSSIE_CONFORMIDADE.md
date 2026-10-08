# Dossiê de Conformidade — Projeto Nexus (SGD/MGI)

Índice das evidências de conformidade governamental, para auditoria
interna (controladoria) e externa (CGU/TCE) e para o processo de
homologação junto à Secretaria de Governo Digital.

- **Última atualização:** 2026-09-09
- **Branch de referência:** `feat/conformidade-roadmap`
- **Escopo:** backend Go 1.26 (microkernel) + frontend Next.js 16

---

## 1. Normas atendidas e evidência

| Norma | Evidência no repositório |
|---|---|
| e-MAG 2.0 / WCAG 2.1 AA (Lei 13.146/2015) | `/acessibilidade` (com Declaração de Acessibilidade), `docs/VPAT.md`, `components/layout/EMagAccessibilityBar.tsx`, `eslint-plugin-jsx-a11y` no CI |
| LGPD (Lei 13.709/2018) | `/privacidade`, `internal/platform/lgpd/*`, migrations 000002/000005/000006, `docs/RIPD_MODELO.md` |
| LAI (Lei 12.527/2011) + LC 131/2009 | `audit_logs` append-only + cópia WORM, `GET /api/v1/audit/export` (CSV/JSON/XML, filtros), `internal/platform/transparency/*` |
| Portaria SGD/SEDGG 2.154 | `internal/platform/auth/*` (OIDC/JWKS, níveis Bronze/Prata/Ouro) |
| DSGov / GovBR-DS | `frontend/src/components/ui/*`, `globals.css`, `Footer.tsx` |
| e-PING | `docs/openapi.yaml`, `/openapi.json`, `/docs`, RFC 7807 via `Accept` |
| OWASP ASVS / Top 10 | `/sobre` (matriz), `httpserver/middleware.go`, CI (govulncheck/staticcheck/gosec) |

## 2. Decisões de arquitetura (ADRs)

| ADR | Assunto |
|---|---|
| `docs/adr/002` | Resiliência e governança enterprise |
| `docs/adr/003` | Hardening RSA do login local |
| `docs/adr/004` | Outbox por LISTEN/NOTIFY + RBAC modular |
| `docs/adr/005` | Roadmap de conformidade governamental (este ciclo) |
| `docs/adr/006` | RFC 7807 por negociação de conteúdo |
| `docs/adr/007` | Exceção de CSP `style-src` (VLibras) |
| `docs/adr/013` | Permissão com escopo e herança (RBAC multi-escopo) |
| `docs/adr/017`, `019`, `022` | TTDD oficial (CCPAD): vigência, histórico e atualização auditada |
| `docs/adr/020`, `021` | IA governada: conexões com chave cifrada, autorização e mascaramento para fornecedor externo (LGPD art. 33); assistente restrito ao acervo do Atlas — fluxos e TTDD (ADR 023) |
| `docs/adr/024` | Biblioteca de modelos de documento: versões imutáveis com SHA-256, arquivo conferido pelo conteúdo, download pela API sem autoria nas respostas públicas, alterações auditadas |

## 3. Rastreamento do roadmap

`docs/ROADMAP_CONFORMIDADE.md` — estado por item das 6 fases.

## 4. Pipeline de verificação (CI)

`.github/workflows/ci.yml` — a cada push/PR:

| Job | Verifica |
|---|---|
| backend-ci | `govulncheck`, `staticcheck`, `gosec` (bloqueante), `go vet`, `go test -race -p 1` |
| migrations-reversibility | `goose up → down-to 0 → up` |
| frontend-ci | `npm audit --audit-level=high` (bloqueante), ESLint (jsx-a11y), `tsc --noEmit`, `vitest` com pisos de cobertura, `next build` |
| docker-build-healthcheck | build das imagens + todos os serviços `healthy` + E2E Playwright dos fluxos críticos |

## 5. Evidências operacionais a anexar (execução)

- [ ] Saída verde do CI no merge para `main` (link do run).
- [ ] Relatório de **pentest externo** (F5.1) + retest dos achados.
- [ ] Laudo de **avaliação e-MAG por avaliador humano** com tecnologia assistiva (F5.2) — homologa `docs/VPAT.md`.
- [ ] **RIPD oficial** assinado pelo DPO (a partir de `docs/RIPD_MODELO.md`).
- [x] **Object Lock / retenção** na cópia WORM da auditoria — bucket dedicado `AUDIT_WORM_BUCKET` criado com Compliance mode pelo próprio worker (F2.6).
- [ ] Definição da **política de retenção** por categoria de dado (`AUDIT_WORM_RETENTION_DAYS` é só a da trilha).
- [ ] Designação e publicação do **Encarregado (DPO)**.
- [ ] Definir `METRICS_SCRAPE_TOKEN` e `TRUSTED_PROXIES` no ambiente de produção.

## 6. Parâmetros de produção a definir

| Variável | Para quê |
|---|---|
| `METRICS_SCRAPE_TOKEN` | fechar `/metrics` (hoje aberto) |
| `TRUSTED_PROXIES` | CIDRs do(s) proxy(s) reverso(s) — sem isso o IP real não é registrado |
| `CONFIG_ENCRYPTION_KEY`, `NEXTAUTH_SECRET`, `DB_PASSWORD`, `RABBITMQ_DEFAULT_PASS`, `MINIO_*` | segredos reais (o backend recusa subir em produção com os valores de exemplo) |
| `API_RATE_LIMIT_*` | ajuste fino do teto por identidade, se necessário |
