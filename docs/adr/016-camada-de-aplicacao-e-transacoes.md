# 016 — Camada de aplicação controla a transação com pgx

- **Status:** Aceito (registra a prática vigente)
- **Data:** 2026-10-02
- **Relacionado:** [ADR 004](004-outbox-listen-notify-e-rbac-modular.md) (outbox), [ADR 008](008-kernel-grafo-de-modulos-e-estrategia-de-testes.md) (estratégia de testes)

## Contexto

Os plug-ins seguem `domain` / `application` / `infrastructure` /
`transport`. Em todos eles, a camada `application` importa
`github.com/jackc/pgx/v5` e `pgxpool` para abrir a transação
(`database.WithTx`) que reúne, atomicamente, a escrita de negócio, o evento
no Transactional Outbox e o registro de auditoria encadeada. Pela Clean
Architecture "pura", a aplicação não deveria conhecer o driver do banco.

## Alternativas

1. **Porta `UnitOfWork` no domínio** (`Do(ctx, func(ctx, Tx) error)`), com
   o outbox e a auditoria também atrás de portas. Isola o driver, mas:
   - toda chamada de repositório passaria a receber um tipo opaco que,
     dentro da infraestrutura, precisa voltar a ser `pgx.Tx` (asserção de
     tipo em cada método);
   - o outbox e a auditoria são da **plataforma** (`internal/platform`) e
     já recebem `pgx.Tx` — embrulhá-los em cada módulo duplicaria código
     em 16 plug-ins sem ganho de testabilidade (os testes de falha já
     injetam erros por `database.DBTX` — `dbtest.Fail`, `ScanFail`, `Seq`);
   - o banco é decisão de plataforma (PostgreSQL: RLS de auditoria,
     LISTEN/NOTIFY, `tsvector`), não algo que um módulo troque.
2. **Manter** a aplicação dona da transação com os tipos do pgx.

## Decisão

Manter a alternativa 2, com regras explícitas:

- `domain` **nunca** importa pgx: entidades, regras e a interface
  `Repository` (que recebe `database.DBTX`, a interface mínima comum a pool
  e transação).
- `application` pode usar `*pgxpool.Pool`, `pgx.Tx` e `database.WithTx`
  **apenas** para delimitar a transação e passá-la a repositório, outbox e
  auditoria. SQL fica só em `infrastructure`.
- Toda mutação: escrita + `outbox.Write` + `audit.Record` na mesma
  transação.

## Consequências

- O código atual já está conforme; o README deixa de afirmar separação
  "estrita" e aponta para este ADR.
- Uma eventual troca de banco seria uma mudança de plataforma, tratada em
  ADR próprio.
