# 004 — Dispatcher do Outbox por LISTEN/NOTIFY e RBAC por módulo

- **Status:** Aceito
- **Data:** 2026-08-29
- **Autores:** Engenharia Projeto Nova

## Contexto

Após a entrega da governança de contratos (Kanban de liquidação `demands`,
casador automático Contrato↔Diário, geração de documentos oficiais) e duas
auditorias de segurança (OWASP/DevSecOps e Keycloak IAM/Outbox), três
pontos de arquitetura precisaram de decisão registrada:

1. **Dispatcher do Outbox por polling fixo.** `outbox.Publisher` fazia
   `SELECT ... FOR UPDATE SKIP LOCKED` a cada 2 s. Multi-réplica seguro,
   mas adiciona até 2 s de latência a toda notificação e mantém um trilho
   de `SELECT` constante mesmo com a fila vazia. A diretiva do projeto
   (`CLAUDE.md` §Outbox Dispatcher) pedia LISTEN/NOTIFY.

2. **Autorização das rotas de `demands` inexistente.** As rotas do Kanban
   de liquidação só passavam por `RequireAuthentication` — qualquer
   usuário autenticado movia card e anexava documento de compliance. Não
   havia papel para a persona do fiscal: ela só cabia em `nexus-admin` ou
   `projeto-nexus-integration-manager`, ambos privilegiados demais.

3. **`OutboxEmitter` em transação isolada (dual-write).** O casador
   `contratos.diario_matcher` gravava `contrato_diario_refs` /
   `contrato_diario_alertas` em auto-commit e depois emitia o evento de
   outbox em **outra** transação. Um crash no meio persistia a mutação
   sem o evento — e, sendo o casador idempotente, a próxima execução não
   reemitia: a notificação (inclusive alertas de fiscalização) se perdia.

## Decisão

### 4.1. Outbox dirigido por LISTEN/NOTIFY, polling como rede de segurança

- `outbox.Writer.Write` passa a executar `SELECT pg_notify('nova_outbox_channel','')`
  **na mesma transação** do `INSERT`. O `NOTIFY` do PostgreSQL é entregue
  no **commit**, então nunca acorda o Publisher para uma linha que sofreu
  rollback.
- `outbox.Publisher.Run` roda um único loop de despacho alimentado por
  três gatilhos: (a) poll imediato no boot (drena o backlog); (b) `NOTIFY`
  no canal; (c) um ticker de segurança, **elevado de 2 s para 15 s** (só
  cobre um `NOTIFY` perdido, ex. janela de reconexão).
- `listenLoop`/`listenOnce`: `LISTEN` numa conexão **dedicada** obtida via
  `pool.Acquire` + `Hijack` (uma conexão com `LISTEN` pendente não pode
  voltar ao pool); reconexão com backoff exponencial até 30 s; força um
  poll após cada (re)conexão.
- `publishPendingBatch` mantém `SELECT ... FOR UPDATE SKIP LOCKED` — a
  garantia multi-réplica não muda.
- **Verificado**: teste de integração (Postgres real) — evento despachado
  ~0,37 s após o commit (o ticker é 15 s).

### 4.2. RBAC por módulo para `demands` + papel `nova-fiscal`

- Novas permissões `demands:read` / `demands:manage`. Ler o quadro, o
  checklist e o histórico exige `read`; **mover card e anexar documento de
  compliance** exigem `manage`. As escritas passam ainda por rate limiting
  por usuário (`RateLimiters.Mutations`).
- Novo papel **`nova-fiscal`**: `demands:read` + `demands:manage` +
  `contratos:read` — opera a liquidação e consulta contratos, nada além.
- **Fora de escopo (por ora):** restrição *por etapa* (o fiscal executa as
  ações fiscais 1/3/4/5 mas não confirma sozinho os handoffs de
  contabilidade). Depende de uma política ainda não decidida e de passar a
  identidade do ator para `application.ValidateTransition`. Ponto de
  extensão documentado no código.

### 4.3. Casador Contrato↔Diário atômico (Transactional Outbox correto)

> **Nota (limpeza pós-genericização):** o módulo `diario_oficial` e o
> casador `contratos.diario_matcher` descritos nesta seção foram
> removidos do código-base; a correção de atomicidade abaixo não se
> aplica mais a nenhum código existente. Mantido apenas como registro
> histórico da decisão.

- `matchOne` processava **um contrato numa única `database.WithTx`**: os
  `INSERT` das refs/alertas e os `outbox.Write` dos eventos
  (`contrato.diario_ref.linked`, `contrato.fiscal_alert`) commitavam
  juntos ou davam rollback juntos.
- Repositório: `LinkDiarioRef`/`RecordAlertOnce` → variantes
  `...Tx(tx pgx.Tx)` (SQL compartilhado via helper com interface
  `pgxExec`); `ListDiarioRefsTx` passava a enxergar as refs
  recém-inseridas na mesma tx (corrigia um falso `SEM_VINCULO_DIARIO`
  latente).
- `OutboxEmitter` / `ContratoEventEmitter` removidos.

## Consequências

- **Positivas:** latência de notificação ~zero; carga de `SELECT` no
  Postgres cai (só quando há evento ou a cada 15 s); a persona do fiscal
  ganha menor privilégio real; o casador deixa de poder perder eventos.
- **Custos:** uma conexão do pool fica permanentemente dedicada ao
  `LISTEN` por réplica de worker (via `Hijack`).
- **Migrations relacionadas (na numeração histórica, pré-consolidação em
  `000001_baseline`):** `000035` (fila de revisão), `000036`
  (`contrato_diario_alertas`), `000037`/`000038` (índices de hardening,
  `idx_contract_occurrences_open` único), `000039` (índices trigram do
  casador) — todas removidas na genericização da base (§4.3).
