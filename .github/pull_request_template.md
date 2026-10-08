<!--
Template de PR — Projeto Nexus (base governamental SGD/MGI).
Preencha o checklist de conformidade; um item que não se aplica marque com
[N/A] e uma frase de justificativa. PRs que tocam contrato de API, RBAC,
esquema de audit_logs ou a pilha de middleware exigem um ADR em docs/adr/.
-->

## O que muda e por quê



## Checklist de conformidade

### Acessibilidade (e-MAG 2.0 / WCAG 2.1 AA) — só se toca o frontend
- [ ] Todo controle interativo tem rótulo acessível (`<label htmlFor>`, `aria-label` ou `aria-labelledby`)
- [ ] Foco visível em 100% dos alvos novos; navegação só-teclado funciona
- [ ] Contraste ≥ 4.5:1 (texto normal) / 3:1 (texto grande e ícones de UI)
- [ ] Imagens com `alt`; ícones decorativos com `aria-hidden`
- [ ] `npm run lint` (inclui `eslint-plugin-jsx-a11y`) passou

### LGPD (Lei 13.709/2018)
- [ ] Nenhum dado pessoal (CPF, e-mail, telefone, endereço) em log — usa `logging.PIICPF/PIIEmail/PIIPhone` quando precisa registrar
- [ ] Nenhum dado pessoal em payload de API além do estritamente necessário (minimização, art. 6º III)
- [ ] Nenhum segredo em `NEXT_PUBLIC_*`, em log ou em `metadata` de `audit_logs`

### Trilha de auditoria (§49)
- [ ] Toda mutação (POST/PUT/PATCH/DELETE) grava `audit_logs` com **ator, ação, recurso, IP de origem e correlation_id** — via `audit.FromRequest(r)`
- [ ] A escrita de auditoria é atômica com a mutação quando faz parte de uma transação (`audit.NewWriter(tx)`)

### Segurança (OWASP)
- [ ] Rota nova protegida por `RequirePermission` adequado — testado o caminho negativo (403 sem a permissão)
- [ ] Entrada validada (`httputil.DecodeJSON` + `httputil.Validate`), nunca `json.NewDecoder` cru
- [ ] SQL 100% parametrizado; zero concatenação de string
- [ ] `go test ./... -p 1 -race` e (frontend) `npm test` verdes

### Migrations
- [ ] Toda migration tem `-- +goose Down` e foi exercitada localmente (`make migrate-redo`)

### ADR
- [ ] Este PR **não** toca contrato de API / RBAC / esquema de auditoria / middleware — OU há um ADR em `docs/adr/` referenciado aqui: `docs/adr/00X-...md`

## Como testar

