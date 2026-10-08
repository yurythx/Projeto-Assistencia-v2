# Projeto Nexus — Frontend

Dashboard em Next.js (App Router) + TypeScript para o Projeto Nexus. Veja o
[README raiz do repositório](../README.md) para a visão geral completa do
projeto, a configuração do Keycloak e como rodar toda a stack via Docker
Compose.

## Desenvolvimento local

```bash
cp .env.example .env.local   # preencha as URLs do backend (Keycloak é opcional — há login local)
npm install
npm run dev
```

## Scripts

| Comando | Finalidade |
|---|---|
| `npm run dev` | Inicia o servidor de desenvolvimento |
| `npm run build` | Build de produção (`.next/standalone`) |
| `npm run analyze` | `next experimental-analyze` — UI interativa do bundle (Turbopack; passe `-- --output` pra só gravar em disco, sem servidor) |
| `npm run lint` | ESLint |
| `npm run typecheck` | `tsc --noEmit` |
| `npm test` | Roda a suíte Vitest uma vez |
| `npm run test:watch` | Vitest em modo watch |
| `npm run test:e2e` | Playwright, contra uma stack real já rodando (ver README raiz, "Testes") |
| `npm run test:e2e:ui` | Idem, com a UI interativa do Playwright |
| `npm run format` | Prettier, modo write |

## Organização

- `src/app` — rotas (App Router): site público (`/`, `/sobre`, `/servicos`,
  `/setores`, `/eventos`, `/contato`, `/verificar/{id}`), `/login`, a área
  autenticada em `(protected)/**` (dashboard, telas de cada plug-in,
  `/auditoria`, `/configuracao/*`, `/perfil`), o route handler do NextAuth e
  os proxies BFF `/api/backend/*` (autenticado) e `/api/public/*` (allowlist
  anônima).
- `src/lib/nexus` — `NexusProvider` (identidade efetiva via `GET /me` e estado
  dos módulos via `GET /system/modules`), `types.ts` (espelho dos DTOs Go) e
  `upload.ts` (upload direto ao MinIO por URL pré-assinada, SHA-256 local).
- `src/components/nexus` — peças de tela reutilizadas pelos plug-ins
  (`PageHeader`, `DataState`, `Pagination`, `ConfirmButton`, `Markdown`
  seguro, `useAction`).
- `src/lib/auth` — configuração do NextAuth: login local usuário/senha
  (CredentialsProvider, RS256) **ou** SSO Keycloak (Authorization Code +
  PKCE); sessão em JWT nos dois casos.
- `src/lib/api` — `client.ts`: cliente tipado para Client Components; sempre chama o proxy BFF na
  mesma origem, para que o access token nunca chegue ao JS do navegador. `swr.ts`/`SWRProvider.tsx`:
  integração com SWR (dedupe/cache/revalidação) para o pouco que ainda é `"use client"` +
  busca de dados depois do carregamento inicial — a maioria das páginas busca em Server Components,
  ver `src/app/**/page.tsx`.
- `src/lib/websocket` — cliente WebSocket com reconexão, backoff e
  heartbeat.
- `src/lib/validation` — schemas Zod que validam todo payload de WebSocket
  antes de ele ser usado.
- `src/components/ui` — o kit de componentes compartilhado, sem regra de
  negócio.
- `src/proxy.ts` — o `proxy.ts` do Next.js 16 (antigo `middleware.ts`),
  responsável por proteger as rotas autenticadas (`(protected)/**`) e por
  gerar o CSP com nonce em cada requisição.

## Sistema visual (§ redesenho 2026-08)

Direção: **documento oficial / carimbo de repartição**. Um acento só, gasto
no elemento de assinatura; tudo em volta fica quieto.

- **Cor** — tokens em `src/app/globals.css`. Base institucional
  (`surface` / `surface-border` / `muted` / `primary` azul `#1b365d`). O
  acento de assinatura é `--seal` (tinta de carimbo `#8a1c1c` claro /
  `#d1524a` escuro / `#ff5b5b` e-MAG), usado **só** em: selo da marca,
  numeração de etapa, códigos (OWASP), marcador de alerta. Nunca como cor
  de UI genérica. Os aliases `border` / `card` / `muted-foreground`
  apontam para os tokens base — código novo usa os nomes base.
- **Tipografia** — `Newsreader` (serifada de notícia) **só** em `h1`/`h2`
  (título de página e de seção; regra global no `globals.css`). `Inter` no
  corpo e nos dados. `Geist Mono` só onde o monoespaçado significa algo
  (nº de contrato, matrícula, competência, código de autenticidade).
- **Cabeçalho de página** — padrão único: eyebrow `.dateline` (fio em
  tinta de carimbo + versalete mono, ex.: `COMPETÊNCIA · AGOSTO/2026`) →
  `<h1 className="text-2xl font-semibold">` → descrição `text-sm text-muted`.
  Sem ícone decorativo no `<h1>`.
- **Elemento de assinatura** — `src/components/ui/Seal.tsx`, um carimbo
  circular. Marca d'água discreta no login e nos documentos oficiais
  imprimíveis; não repetir pela interface toda.
- **Marca** — `src/components/ui/Logo.tsx` e `src/app/icon.svg` são o mesmo
  desenho (selo com "N"), mantidos em sincronia à mão. **Sem
  `<defs>`/gradiente**: dois `<Logo>` no mesmo DOM (ex.: login desktop +
  mobile) com IDs de gradiente repetidos quebram a instância visível.
- **Modais** — todos sobre o `<dialog>` nativo via `ui/ModalShell` (Esc,
  clique-no-backdrop, captura de foco, fundo `inert`, trava de scroll,
  transição por `@starting-style`). `ui/Dialog` é a variante estruturada
  (título/descrição/corpo rolável/rodapé fixo) com prop `size`
  (sm/md/lg/xl).
- **Quality floor** — responsivo até 390px, `focus-visible` em tudo,
  `prefers-reduced-motion` respeitado (`globals.css`), modo e-MAG alto
  contraste (`[data-high-contrast]`).
