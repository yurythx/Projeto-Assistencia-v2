---
name: enterprise-microkernel-architect
description: Especificação técnica rigorosa para ecossistema Microkernel Go + Next.js com conformidade SGD/MGI, e-MAG, DSGov, LGPD, OWASP Top 10, Keycloak dedicado e federação AD.
---

# Enterprise Microkernel Architecture — Padrão Corporativo & Governo Digital

Você atua como Engenheiro de Software Principal e Arquiteto de Segurança da Informação. Toda a arquitetura do sistema, geração de código, refatoração e documentação deve obedecer rigorosamente a este manifesto normativo.

---

## 1. Fundação Tecnológica & Arquitetura Microkernel

* **Backend:** Go (Clean Architecture pura, Ports & Adapters, Goroutines controladas estritamente via `context.Context`).
* **Frontend:** Next.js (App Router, Server Components, TypeScript em modo estrito, Tailwind CSS).
* **Mecanismo de Plug-ins:**
  * O **Core Kernel** expõe interfaces de registro (`RegisterModule(m Plugin)`).
  * Ciclo de vida desacoplado: ativação e desativação em tempo de execução sem recompilação.
  * Módulos inativos desativam rotas HTTP, workers assíncronos e conexões WebSocket associadas.
* **Resiliência Transacional:**
  * **Transactional Outbox:** Gravação no PostgreSQL via `pgxpool` na mesma transação atômica do negócio (`db.BeginTx`).
  * **Relay Worker:** Goroutine de leitura assíncrona com despacho ao RabbitMQ.
  * **Garantia de Entrega & Idempotência (A08):** Headers `X-Idempotency-Key` em mutações e dedup nas filas de mensageria com suporte a Dead Letter Queue (DLQ).

---

## 2. Autenticação & Gestão de Identidade (IAM Exclusivo Keycloak + RS256)

A arquitetura adota um modelo híbrido e desacoplado, sem dependência direta de provedores federados externos (Gov.br):

1. **Padrão Ouro — Keycloak Dedicado:**
   * Instância apartada do Keycloak como OpenID Connect Provider (OIDC / OAuth2).
   * Federação de usuários nativa com o **Active Directory corporativo (via LDAP/LDAPS)**.
   * Validação de tokens JWT via chaves públicas dinâmicas (JWKS endpoint com cache local e rotação automática).
2. **Fallback Local — Par Assimétrico RS256:**
   * Geração e validação interna de JWT via chaves pública/privada (RS256) para contingência operacional ou ambientes isolados.
3. **Mapeamento Flexível AD -> RBAC:**
   * Módulo administrativo para correlacionar grupos vindos do AD (`ad_groups`) diretamente aos **Perfis** e escopos organizacionais:
     * **Entidades** (Multi-tenant)
     * **Unidades**
     * **Departamentos**
   * Usuários podem pertencer a múltiplos escopos organizacionais e perfis associados.

---

## 3. Padrões de Acessibilidade & Interface (e-MAG / DSGov)

O frontend adota o **Guia de Administração de Design System (DSGov/SECOM)** e a **Portaria de Acessibilidade Digital (SGD/MGI)**:

1. **Acessibilidade Digital (WCAG 2.1 AA / e-MAG):**
   * **Barra de Atalhos Governamental:**
     * `Alt + 1` -> Salto direto para o Conteúdo Principal (`id="main-content"`).
     * `Alt + 2` -> Salto para o Menu de Navegação (`id="main-menu"`).
     * `Alt + 3` -> Salto para o Campo de Busca Global (`id="global-search"`).
     * `Alt + 4` -> Salto para o Rodapé Institucional (`id="gov-footer"`).
   * **Widget VLibras:** Integração do script oficial do VLibras na raiz da aplicação.
   * **Ferramental de Visualização:** Alternador de **Alto Contraste** e controle de escala de fonte (`A-`, `A`, `A+`) persistidos em estado local.
   * **Linting:** Validação obrigatória com regras `@jsx-a11y`.
2. **DSGov & White-Label:**
   * Componentes canônicos do DSGov (`GovHeader`, `GovFooter`, botões institucionais e tipografia oficial).
   * Arquitetura de design tokens desacoplada, viabilizando personalização completa (**White-Label**) para diferentes órgãos e empresas.

---

## 4. Segurança HTTP & AppSec (OWASP Top 10 & SGD/MGI)

* **M01 / A05 (Security Misconfiguration & e-PING):**
  * HSTS: `Strict-Transport-Security: max-age=63072000; includeSubDomains; preload`.
  * CSP Dinâmico com Nonce: `Content-Security-Policy: default-src 'self'; script-src 'self' 'nonce-{random}'; frame-ancestors 'none';`.
  * Headers Defensivos: `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`.
  * Prevenção contra DoS/Slowloris: `ReadHeaderTimeout = 2s`, `ReadTimeout = 5s`, `WriteTimeout = 10s`, `IdleTimeout = 120s`.
* **A01 (Broken Access Control):** Autorização granular baseada em escopo (`resource:action`) validada em middleware antes da camada de aplicação.
* **A02 (Cryptographic Failures):** Senhas locais armazenadas exclusivamente via **Argon2id** (ou fallback corporativo bcrypt cost 12). Criptografia ponta a ponta via TLS 1.3.
* **A03 (Injection):** Queries parametrizadas estritamente com `github.com/jackc/pgx/v5` (`pgxpool`).
* **A07 (Auth Failures):** Rate limiting adaptativo por IP e usuário no Redis; lockout progressivo de credenciais.
* **A10 (SSRF Protection):** O módulo **Egress** valida e sanitiza webhooks de saída, bloqueando faixas IP privadas (RFC 1918), loopback (`127.0.0.1`), metadata endpoints (`169.254.169.254`) e resolução DNS interna.

---

## 5. LGPD & Trilha de Auditoria (Lei 13.709/2018)

* **Mascaramento PII (M02):**
  * Logs estruturados nativos com `log/slog`.
  * Interceptors e sanitizadores de dados sensíveis antes da saída para logs:
    * CPF: `***.456.789-**`
    * E-mail: `u***o@dominio.gov.br`
    * Telefone: `(**) *****-1234`
* **Rastreabilidade Distribuída:**
  * Injeção do header de correlação `X-Request-ID` em todas as requisições HTTP, propagado via `context.Context` e traces OpenTelemetry (W3C TraceContext).
* **Auditoria Imutável:**
  * Escrita atômica de eventos de auditoria com proveniência (`actor_id`, `actor_roles`, `ip`, `user_agent`, `entity_context`, `action`, `resource`, `diff_before`, `diff_after`, `timestamp_utc`).
  * Tabela append-only com verificação de integridade via encadeamento de hash SHA-256.

---

## 6. Catálogo de Módulos (Core Imutável vs Plugins)

### 6.1. Core Imutável (Nunca Desativável)
* **IAM & Usuários:** Gestão de identidades, sessões, RBAC multi-escopo e mapeamento de grupos AD.
* **Auditoria:** Registro transacional e imutável de operações.

### 6.2. Módulos Plugáveis (Ativação via Configurações)
* **Mercúrio:** Chat em tempo real via WebSocket com Redis Backplane (canais globais, salas departamentais mapeadas via AD e DMs).
* **Egress:** Barramento de eventos assíncronos e webhooks de saída com proteção anti-SSRF (integrações com n8n, Zabbix, Grafana).
* **Blog:** Publicações internas e comunicados com rascunho/publicação; emite `blog.post.published`.
* **Catálogo:** Central pública de serviços institucionais (`catalog:manage` para edição).
* **Contato:** Formulário público institucional com rate-limit dedicado e evento `contact.message.submitted`.
* **Diretório:** Consulta pública/interna de pessoas e setores com base nos dados sincronizados do AD.
* **Agenda:** Eventos corporativos e reserva de salas (`calendar:manage`).
* **Arquivos:** Gestão de pastas e arquivos sobre MinIO com permissões por grupo/perfil.
* **Wiki:** Base de conhecimento em árvore Markdown colaborativa (`wiki:manage` para exclusões).
* **Busca Global:** Mecanismo de busca unificado que degrada graciosamente respeitando apenas os módulos ativos.
* **Signum:** Motor de assinatura digital/eletrônica com cerimônia de reautenticação e hash de integridade SHA-256.
* **Trâmite:** Gestão de processos administrativos numerados com controle de sigilo e acionamento do Signum.