# Guia de Desenvolvimento de Novos Módulos (Plug-ins) — Projeto Nexus

Este documento é o guia oficial para engenheiros que vão criar novos **módulos de negócio (plug-ins)** sobre o **Core System (Kernel)** da plataforma **Projeto Nexus**. Siga estas convenções para manter o isolamento de domínio, o desacoplamento e a consistência da **Arquitetura Microkernel**.

---

## 🏗️ Visão Geral da Arquitetura Microkernel

O Projeto Nexus utiliza o modelo **Microkernel (Plug-in Architecture)** combinado a **Camadas Limpas (Clean Architecture)** e comunicação assíncrona orientada a eventos via **Transactional Outbox**:

```
   Navegador (React / Next.js DSGov Shell)
        │  ▲
        │  │ (WebSocket Events)
        ▼  │
   BFF Proxy (/api/backend/*)
        │
        ▼
 ┌──────────────────────────────────────────────────────────┐
 │               CORE SYSTEM (KERNEL) — GO                  │
 │  (IAM, Auditoria, Rate Limit, Outbox, WS Hub, Guard)    │
 └──────────────────────────┬───────────────────────────────┘
                            │ (RegisterModule + modkit.Deps)
                            ▼
 ┌──────────────────────────────────────────────────────────┐
 │          PLUG-IN DE NEGÓCIO (ex.: patrimonio)            │
 │  Manifest · Transport -> Application -> Domain -> PG     │
 └──────────────────────────┬───────────────────────────────┘
                            │ (Transação ACID)
 ┌──────────────────────────┴───────────────────────────────┐
 │ PostgreSQL (Tabela do Módulo + Tabela Outbox)            │
 └──────────────────────────┬───────────────────────────────┘
                            │ (Outbox Worker)
                            ▼
                         RabbitMQ
                            │
                            ▼
                       WebSocket Hub -> Cliente
```

---

## ⚡ Scaffolding Automático de Módulos

Para criar um novo plug-in com 1 comando:

```bash
./scripts/create-module.sh <nome_do_modulo>
# ou
make new-module NAME=financeiro
```

O script copia o blueprint `internal/modules/example` (código que roda em produção, portanto o esqueleto já nasce compilando) e:
1. Gera `internal/modules/<nome>/` com `module.go` (Manifest do Kernel) e as camadas `domain`, `application`, `infrastructure`, `transport`.
2. Cria a migration sequencial `backend/migrations/0001NN_<nome>.sql` (goose).
3. Registra o plug-in no Kernel (`internal/app/modules.go` → `Kernel.MustRegister`).
4. Cria a página `frontend/src/app/(protected)/<nome>/page.tsx` sobre o kit (`DataState`, `useAction`, `useNexus`) e protege a rota em `src/proxy.ts`.

O módulo nasce **desativado** (`DefaultEnabled: false`); ative em **Configurações → Módulos**.

---

## 📁 Estrutura de Pastas de um Módulo

### Backend (`backend/internal/modules/financeiro/`)
```text
internal/modules/financeiro/
├── module.go          # kernel.Plugin: Manifest + RegisterRoutes/Workers/Consumers/SearchProviders…
├── domain/            # Entidades, invariantes e erros de domínio (sem dependência de infraestrutura)
├── application/       # Casos de uso: transação (database.WithTx) + outbox + auditoria
├── infrastructure/    # Repositório PostgreSQL (pgx, queries parametrizadas; full-text via nexus_search_tsquery — ADR 011)
└── transport/         # Handlers chi: Bind/Validate, RequirePermission, WriteOK/WritePage
```

### Frontend (`frontend/src/`)
```text
src/
├── app/(protected)/financeiro/   # Páginas (App Router), protegidas pelo proxy + layout (protected)
│   ├── page.tsx
│   └── [id]/page.tsx
├── components/financeiro/        # Componentes específicos do módulo
└── lib/nexus/types.ts            # Espelho TypeScript dos DTOs Go do módulo
```

---

## 🛠️ Passo a Passo

### 1. Migration (`backend/migrations/`)
Numeração sequencial, um arquivo com `Up` e `Down` (o CI roda `up → down → up`):

```sql
-- +goose Up
CREATE TABLE financeiro_titulos (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    descricao  TEXT NOT NULL,
    valor      NUMERIC(15, 2) NOT NULL,
    status     TEXT NOT NULL DEFAULT 'pendente',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS financeiro_titulos;
```

### 2. Manifest do plug-in (`module.go`)
O Manifest é o contrato com o Kernel: chave, dependências, permissões (exibidas na tela de Perfis), ícone e rota do frontend (usados para montar o menu).

```go
func (m *Module) Manifest() kernel.Manifest {
	return kernel.Manifest{
		Key:            "financeiro",
		Name:           "Financeiro",
		Description:    "Títulos a pagar e a receber.",
		DefaultEnabled: false,
		DependsOn:      []string{"signum"}, // opcional: o Kernel recusa ativar sem as dependências
		Icon:           "wallet",           // nome lucide (ver components/layout/ModuleIcon.tsx)
		Route:          "/financeiro",
		Permissions: []kernel.PermissionInfo{
			{Key: "financeiro:read", Description: "Consultar títulos"},
			{Key: "financeiro:manage", Description: "Criar e baixar títulos"},
		},
	}
}

func (m *Module) RegisterRoutes(r kernel.Routes) {
	m.handlers.RegisterRoutes(r.Authed) // r.Public: rotas anônimas com rate limit por IP
}
```

Com o módulo desativado, o **Guard** do Kernel responde 404 (`MODULE_DISABLED`) em toda a superfície HTTP, para workers e consumidores de fila, derruba as assinaturas WebSocket dos tópicos `<chave>:*` e remove o módulo da Busca Global — sem reiniciar o processo.

### 3. Caso de uso transacional (`application/`)
Negócio, evento (Transactional Outbox) e auditoria imutável commitam **na mesma transação**:

```go
err := database.WithTx(ctx, s.pool, func(ctx context.Context, tx pgx.Tx) error {
	if err := s.repo.Insert(ctx, tx, titulo) // o repositório recebe o executor (database.DBTX), nunca abre tx própria; err != nil {
		return err
	}
	if err := s.outbox.Write(ctx, tx, "financeiro.titulo.created", "financeiro_titulo", titulo.ID.String(), uuid.Nil, payload); err != nil {
		return err
	}
	return audit.NewWriter(tx).Record(ctx, audit.Meta(ctx, "financeiro.titulo.created", "financeiro_titulo", titulo.ID.String(), nil, titulo))
})
```

O Relay Worker publica o evento no RabbitMQ; consumidores deduplicam por `event_id` (DLQ para falhas definitivas). Mutações HTTP aceitam `X-Idempotency-Key` — o `apiClient` do frontend já envia uma chave nova por requisição.

### 4. Rotas HTTP (`transport/`)
Autorização `recurso:ação` sempre no middleware (A01), nunca só na tela:

```go
func (h *Handlers) RegisterRoutes(r chi.Router) {
	r.With(auth.RequirePermission(h.logger, "financeiro:read")).Get("/financeiro/titulos", h.List)
	r.With(auth.RequirePermission(h.logger, "financeiro:manage")).Post("/financeiro/titulos", h.Create)
}
```

Use `httputil.Bind` (limite de corpo + campos desconhecidos recusados + validação), `httputil.Page`/`WritePage` para listas e `httputil.WriteError` (RFC 7807 quando o cliente pede `application/problem+json`).

### 5. Tela no frontend
```tsx
"use client";
export default function FinanceiroPage() {
  const { can } = useNexus();                                    // permissões efetivas (só exibição)
  const list = useApiPage<Titulo>(withQuery("v1/financeiro/titulos", { page }));
  const { run } = useAction();                                   // toast de sucesso/erro
  // …
  return (
    <DataState loading={list.isLoading} error={list.error} empty={!list.data?.items.length}>
      {/* tabela */}
    </DataState>
  );
}
```

- O item de menu aparece sozinho a partir do Manifest (`Route`/`Icon`) quando o módulo está ativo — nada a editar no `Sidebar`.
- Chamadas sempre por `apiClient`/`useApiQuery` (BFF `/api/backend/*`: o token nunca chega ao navegador).
- Uploads: peça um `UploadTicket` ao backend e envie direto ao MinIO com `putToTicket` (`lib/nexus/upload.ts`).
- Tempo real: `useTopic("financeiro:…", handler)` assina um tópico autorizado pelo `WSProvider` do plug-in.

---

## ⚡ Boas Práticas e Regras de Ouro

1. **Zero-Mock Policy:** Nunca utilize dados simulados hardcoded em telas de produção. Utilize fallbacks seguros com indicação de tela vazia (`EmptyState`).
2. **Resiliência CSP:** Nunca utilize atributos `style=""` inline. Utilize tokens de design do Tailwind CSS pré-configurados para conformidade com a política Content-Security-Policy com Nonce.
3. **Internacionalização e Acessibilidade (e-MAG):** Mantenha rótulos em português brasileiro (`pt-BR`) e garanta suporte a leitor de tela (`aria-label`, `role`, contraste).
4. **Regras de negócio no domínio e na aplicação, nunca só na tela ou nas struct-tags.** Estados finais (cancelado, arquivado, concluído) recusam mutação, e repetir a transição para o estado atual não gera nova auditoria nem novo evento.
5. **Erro de banco nunca vira regra.** Uma falha na checagem de ciclo, de acesso ou de existência é propagada como erro, e não mascarada como "ciclo", "sem acesso" ou "não encontrado". Curingas digitados (`%`, `_`) são texto: use `strpos`/`starts_with` ou escape-os.
6. **100% de cobertura por plug-in (CI).** O módulo gerado já nasce com a matriz de falhas (`application/faults_test.go`) e os testes do repositório com `database/dbtest`; escreva o teste de sistema do fluxo em `internal/app/module_<m>_http_test.go`. Mudou a interface `Repository`? Regenere `faultrepo_test.go` com `scripts/genfault.py`. Detalhes no ADR 009.
7. **Gestão com escopo (ADR 013).** Se o módulo tem uma permissão de gestão
   (`<m>:manage`) sobre recursos que pertencem a uma unidade, dê ao recurso
   uma **unidade dona** (`unidade_id`, `ON DELETE SET NULL`; sem dona =
   institucional, só a gestão global), inclua a permissão em
   `auth.ScopedPermissions` e em `scripts/iam-scope-report.sql` (um teste
   confere que as duas listas andam juntas), e decida cada ação com
   `auth.Can(identity, perm, auth.InUnidade(dona))` — **nunca** com
   `auth.HasPermission`, que só diz se a pessoa tem a permissão em *algum*
   lugar (serve para menus e para o `RequirePermission` da rota). Listas
   filtram no SQL por `auth.CoverageOf`; fora do escopo, 403 para agir e 404
   para ler. Permissão de plataforma (fora de `ScopedPermissions`) só vale
   com concessão global. No frontend, `UnidadeDonaSelect` oferece só as
   unidades que a pessoa gerencia.
8. **Público-alvo (ADR 014).** Conteúdo interno que pode ser dirigido a uma
   secretaria usa uma tabela `<recurso>_publico` (FK `RESTRICT` para
   entidade/unidade) e o pacote `platform/publico`: `Tabela.Condicao` no
   SQL das listas e da busca (público, autor e gestão da dona),
   `Tabela.SemPublico` no que vai ao site, `Carregar`/`Gravar` no
   repositório e `auth.NoPublico` no detalhe. Evento de Outbox de conteúdo
   com público leva `"restrito": true` (a difusão geral em tempo real o
   descarta). No frontend, `PublicoAlvoPicker` e `PublicoBadge`.
