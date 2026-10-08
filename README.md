# Projeto Nexus — Plataforma Enterprise Base Governamental

O **Projeto Nexus** é uma plataforma corporativa modular construída segundo o padrão de **Arquitetura Microkernel (Plug-in Architecture)** em Go (Backend) e Next.js App Router (Frontend), desenvolvida para servir como **base enterprise genérica em estrita conformidade com as normas do Governo Federal Brasileiro** (SGD/MGI, e-MAG 2.0, DSGov, LGPD, Gov.br e OWASP) para o rápido desenvolvimento e desacoplamento de novos módulos de negócio municipais e estaduais.

Ele fornece um **Core System (Kernel)** robusto com infraestrutura pronta de segurança HTTP, mensageria, outbox transacional, autenticação via **Keycloak dedicado com federação do Active Directory** (e fallback local RS256), mascaramento PII, auditoria imutável com cadeia de hash, busca full-text em PostgreSQL e um Design System governamental oficial (DSGov / e-MAG).

---

## 🏛️ Conformidade Governamental (SGD/MGI) & Recursos Globais

O Projeto Nexus foi construído sobre os 5 eixos de conformidade da Secretaria de Governo Digital (SGD/MGI). O estado de cada requisito, com a evidência (código, teste ou pendência), está no [Dossiê de Conformidade](docs/DOSSIE_CONFORMIDADE.md) e no [Roadmap](docs/ROADMAP_CONFORMIDADE.md); a declaração de acessibilidade, no [VPAT](docs/VPAT.md). Conformidade formal depende de avaliação do órgão contratante.

1. **Módulo 1: Segurança HTTP & Headers Defensivos (Go):** Headers OWASP (`HSTS`, `CSP com Nonce`, `X-Frame DENY`, `X-Content-Type nosniff`), rate-limiting e timeouts de servidor anti-DoS.
2. **Módulo 2: Privacidade & LGPD (Go):** Mascaramento nativo de PII (`slog.LogValuer` em CPF, e-mail, telefone) e rastreabilidade correlacionada (`X-Request-ID`).
3. **Módulo 3: IAM Keycloak + RS256 (Go + Next.js):** OIDC com Keycloak apartado (JWKS com cache e rotação), federação LDAP/LDAPS com o Active Directory, mapeamento de grupos do AD para Perfis e escopos (Entidade, Unidade, Departamento) e fallback local RS256.
4. **Módulo 4: Acessibilidade Digital e-MAG (Next.js):** Recursos voltados ao e-MAG 2.0 / WCAG 2.1 AA com barra de atalhos por teclado (Alt+1..4), VLibras nativo, alto contraste e-MAG e redimensionamento de fonte (A+/A-/A).
5. **Módulo 5: Identidade Visual Governamental DSGov (Next.js):** Design System oficial GovBR-DS, rodapé unificado com canais de atendimento, LGPD/LAI e suporte White-Label dinâmico.

---

## 🏗️ Arquitetura Microkernel e Recursos Prontos

- **Arquitetura Microkernel (Core System + Plug-ins):** O Kernel central (`internal/platform`) gerencia a infraestrutura, resiliência e segurança, enquanto novos módulos de negócio (`internal/modules/`) funcionam como plug-ins isolados e desacoplados.
- **Clean Architecture nos Módulos:** Módulos de negócio em camadas (`domain`, `application`, `infrastructure`, `transport`); o domínio não conhece o banco e a aplicação só delimita a transação (escrita + outbox + auditoria) — [ADR 016](docs/adr/016-camada-de-aplicacao-e-transacoes.md).
- **Módulo Modelo Template (`example`):** Blueprint prático e de referência para novos plug-ins.
- **Autenticação Dupla:** Keycloak SSO (OIDC + AD) e autenticação local com chaves RSA, Argon2id, rate limiting e lockout progressivo.
- **Estrutura organizacional multi-entidade:** Entidade > Unidade (com subunidades) > Departamento, perfis e lotações (manuais ou por grupo do AD).
- **Permissão com escopo e herança (ADR 013):** as permissões de gestão valem onde foram concedidas e nas unidades abaixo — o gestor de uma secretaria gerencia só o conteúdo dela; administração delegada do IAM por entidade/unidade.
- **Público-alvo (ADR 014):** Blog, Wiki e Agenda podem ser publicados só para secretarias e/ou unidades.
- **Outbox Transacional & RabbitMQ:** Escrita atômica no PostgreSQL e publicação assíncrona no RabbitMQ com filas Dead-Letter Queue (DLQ).
- **Auditoria Imutável:** Registros de auditoria append-only protegidos no nível do PostgreSQL.
- **WebSocket Server:** Broker WebSocket para retransmissão de notificações em tempo real aos plug-ins.
- **Object Storage (MinIO S3):** Armazenamento de arquivos via URLs pré-assinadas.
- **Busca Full-Text Nativa:** Indexação e busca otimizada no PostgreSQL via `tsvector` e `pg_trgm`.

---

## 📂 Estrutura do Repositório

```text
.
├── backend/                   # ⚙️ Backend em Go (1.26+) — Arquitetura Microkernel
│   ├── cmd/
│   │   ├── api/               # API REST e Servidor WebSocket
│   │   ├── worker/            # Processador background RabbitMQ / Outbox
│   │   └── seedadmin/         # CLI para semente de usuário administrador local
│   ├── internal/
│   │   ├── app/               # Wiring central, injeção de dependências e roteamento
│   │   ├── domain/            # Tipos e erros primitivos de domínio
   │   ├── platform/          # 🛡️ Core System / Kernel (Auth, DB, Messaging, Outbox, Audit, WS)
│   │   └── modules/           # 🔌 Plug-ins (IAM, Auditoria, Mercúrio, Egress, Blog, Catálogo, Contato,
│   │                          #    Diretório, Agenda, Arquivos, Wiki, Busca, Signum, Trâmite, Atlas, Example)
│   ├── migrations/            # Scripts de schema PostgreSQL (Goose)
│   └── pkg/                   # Utilitários genéricos (httputil)
├── frontend/                  # 🎨 Frontend (Next.js / TypeScript / React)
│   ├── src/
│   │   ├── app/               # Routes App Router Next.js
│   │   │   ├── (protected)/   # Área autenticada: dashboard, telas dos plug-ins, auditoria e configurações
│   │   │   ├── login/         # Login
│   │   │   ├── servicos/ setores/ eventos/ contato/ verificar/  # Site público dos plug-ins
│   │   │   └── sobre/         # Documentação da Plataforma
│   │   ├── components/        # Design System (ui, layout, branding, notifications)
│   │   ├── hooks/             # Custom React Hooks
│   │   ├── lib/               # Clientes API / WS / Auth
│   │   └── types/             # TypeScript DTOs
├── docker-compose.yml         # Serviços Docker (PostgreSQL, RabbitMQ, Redis, MinIO, API, Worker, Frontend)
├── docker-compose.dev.yml     # Exposição de portas em desenvolvimento
└── Makefile                   # Atalhos de build, testes, lint e migrations
```

---

## 🧩 Catálogo de Módulos e Telas

| Módulo | Tipo | Telas autenticadas | Superfície pública |
|---|---|---|---|
| IAM & Usuários | Núcleo | Configurações → Estrutura organizacional, Perfis, Mapeamento AD, Usuários; `/perfil` | — |
| Auditoria | Núcleo | `/auditoria` (trilha, verificação da cadeia SHA-256, exportação LAI) | — |
| Mercúrio | Plug-in | `/mercurio` (chat em tempo real) | — |
| Egress | Plug-in | Configurações → Egress & Webhooks | — |
| Blog | Plug-in | `/blog` | — |
| Catálogo | Plug-in | `/gestao/servicos` | `/servicos` |
| Contato | Plug-in | `/gestao/contato` | `/contato` |
| Diretório | Plug-in | `/diretorio` | `/setores` |
| Agenda | Plug-in | `/agenda` | `/eventos` |
| Arquivos | Plug-in | `/arquivos` | — |
| Wiki | Plug-in | `/wiki` | — |
| Busca Global | Plug-in | `/busca` (campo `#global-search`, Alt+3) | — |
| Signum | Plug-in | `/signum` | `/verificar/{id}` |
| Trâmite | Plug-in | `/tramite` | — |
| Atlas | Plug-in | `/atlas` (procedimentos SEI, TTDD oficial com vigência e histórico, assistente sobre fluxos (do início ao fim) e TTDD com `atlas:read`, biblioteca de modelos de documento com versões, gestão — procedimentos, versões, modelos e atualização da TTDD — com `atlas:manage`) | API pública `GET /api/v1/atlas/ttdd` (e `/estrutura`, `/exportar`, `/{codigo}/historico`), `/workflows` e `/modelos` (com `/{id}/arquivo`) |

Os plug-ins são ativados e desativados em runtime em **Configurações → Módulos**: o menu, as páginas públicas e o sitemap acompanham o estado do Kernel. A tela mostra o grafo de dependências (hoje, **Trâmite depende de Signum**): desligar um módulo pede para desligar antes quem depende dele, e ligar pede para ligar antes as dependências.

---

## 🚀 Como Iniciar

### Pré-requisitos
- Docker & Docker Compose
- Go 1.26+ (opcional para rodar local fora do container)
- Node.js 20+ (opcional para rodar frontend fora do container)

### 1. Configurar Variáveis de Ambiente
```bash
cp .env.example .env
```

### 2. Gerar Chave RSA para Autenticação Local
```bash
mkdir -p secrets
openssl genrsa -out secrets/local_auth_private_key.pem 2048
chmod 644 secrets/local_auth_private_key.pem
```

### 3. Subir o Ambiente Docker
```bash
make dev
# Ou via docker compose:
docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build
```

### 4. Semear o Usuário Administrador Inicial
```bash
make seed-admin
```

### 5. Acessar a Aplicação
- **Frontend (Painel Nexus):** [http://localhost:3002](http://localhost:3002)
- **API Healthcheck:** [http://localhost:8002/health](http://localhost:8002/health)

### Regras de negócio
Estrutura organizacional (entidades, unidades, departamentos), perfis, lotações, onde o escopo vale, o público-alvo e as regras de cada módulo: [`docs/REGRAS_DE_NEGOCIO.md`](docs/REGRAS_DE_NEGOCIO.md). As decisões estão nos ADRs ([`docs/adr/`](docs/adr/README.md)) — em especial o [013](docs/adr/013-permissao-com-escopo-e-heranca.md) (escopo) e o [014](docs/adr/014-publico-alvo.md) (público-alvo).

### Serviços obrigatórios e opcionais
A stack mínima (`docker-compose.yml`) é PostgreSQL, RabbitMQ, Redis, MinIO, API, worker e frontend — a busca de todos os módulos (inclusive o Atlas) é a full-text do PostgreSQL. São **opcionais**, por overlay no `COMPOSE_FILE`: Keycloak de teste (`docker-compose.keycloak.yml`), HTTPS com Caddy (`docker-compose.https.yml`, recomendado em qualquer rede) e IA local (`docker-compose.ia.yml`, Ollama, serviço `ia-local`). Sem Keycloak vale o login local. A IA do assistente do Atlas é configurada na tela **Configurações → Inteligência artificial** (IA local ou fornecedores como OpenAI, Azure, Gemini, Claude, com a chave cifrada); sem IA, o assistente responde com a síntese canônica dos procedimentos.

### Deploy em servidor
`./scripts/deploy.sh <ip-ou-dns>` gera `.env` com segredos fortes, a chave RSA e sobe a stack em produção; depois `make prod-seed-admin`. HTTPS com o Caddy como única entrada (`scripts/enable-https.sh`), Keycloak de teste com uma prefeitura fictícia (`make demo-keycloak`, `make demo-popular`), cenários ponta a ponta (`make demo-test`) e backup diário (`scripts/backup.sh`): ordem completa em [`docs/DEPLOY.md`](docs/DEPLOY.md). O servidor de teste (limites, verificação e incidentes) está em [`docs/SERVIDOR_TESTE.md`](docs/SERVIDOR_TESTE.md).

---

## 🧪 Testes e Qualidade de Código

```bash
make test                 # backend + frontend

# Backend com os testes de integração (Postgres real, como no CI)
cd backend
TEST_DATABASE_URL="postgres://nexus:SENHA@localhost:5432/nexus_test?sslmode=disable" \
  go test -race -p 1 -coverpkg=./internal/... ./...

cd frontend && npm test   # Vitest + Testing Library
```

- **Todos os módulos** são exercitados pela API real (`backend/internal/app/*_http_test.go`): permissões, validação, fluxos e auditoria.
- **Ativar/desativar**: um teste descobre todas as rotas de cada plug-in e exige `404 MODULE_DISABLED` com o módulo desligado; o grafo de dependências é aplicado e exibido.
- **Contrato**: `docs/openapi.yaml` é verificado contra as rotas montadas.
- Detalhes em [`docs/GUIDE_DESENVOLVEDOR.md`](docs/GUIDE_DESENVOLVEDOR.md#-6-testes) e no [ADR 008](docs/adr/008-kernel-grafo-de-modulos-e-estrategia-de-testes.md).

---

## 🛠️ Como Criar um Novo Módulo de Negócio

Para adicionar um novo módulo à aplicação (ex.: `patrimonio`):

1. **Scaffolding Automático:** Execute o script de geração de módulo:
   ```bash
   ./scripts/create-module.sh patrimonio
   ```
2. **Guia Completo:** Consulte a documentação detalhada da arquitetura em [`docs/GUIDE_MODULOS.md`](docs/GUIDE_MODULOS.md).
3. **Módulo Blueprint:** Utilize a implementação de referência em `backend/internal/modules/example/` e no menu **"Módulo Modelo"** (`/exemplos` no frontend).
