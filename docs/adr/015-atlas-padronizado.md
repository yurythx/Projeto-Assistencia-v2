# 015 — Atlas alinhado ao padrão dos plug-ins

- **Status:** Aceito e implementado — o grounding por procedimento foi substituído pelo [ADR 021](021-assistente-restrito-a-ttdd.md) (assistente só da TTDD) e a configuração da IA pelo [ADR 020](020-conexoes-de-ia.md)
- **Data:** 2026-10-02
- **Relacionado:** [ADR 009](009-revisao-dos-plugins-e-meta-de-cobertura.md) (revisão dos plug-ins), [ADR 011](011-borda-unica-ip-confiavel-e-busca-por-prefixo.md) (busca por prefixo), [ADR 013](013-permissao-com-escopo-e-heranca.md) (escopo)

## Contexto

O plug-in Atlas (procedimentos canônicos de processo no padrão SEI, Tabela
de Temporalidade e assistente procedural) entrou fora das convenções dos
demais plug-ins. A revisão encontrou:

- **Isolamento falso:** `tenant_id` vinha da query string — quem chamava
  escolhia o "tenant". A plataforma não tem multi-inquilino (uma
  implantação = uma organização; a divisão é entidade > unidade).
- **Rotas sobrepostas:** as rotas públicas e as autenticadas tinham os
  mesmos caminhos no mesmo roteador; a autenticada sobrescrevia a pública e
  a consulta anônima respondia 401.
- **`atlas:read` declarada e nunca exigida;** assistente (que pode acionar
  um modelo de linguagem) sem limite de uso nem trilha de auditoria.
- **Cadastro frágil:** *upsert* com ID novo (código repetido gravava etapas
  apontando para um procedimento inexistente), `DELETE` com erro ignorado,
  transição para etapa inexistente virando 500, URL do modelo de minuta
  sem validação (XSS armazenado com `javascript:`).
- **Typesense** como dependência: a coleção nunca era criada, o
  `filter_by` era montado por interpolação (injeção de filtro) e a chave
  tinha default fixo no `docker-compose.yml`.
- Configuração lida com `os.Getenv` dentro do módulo; corpo de erro do LLM
  gravado no log; `schema.sql` duplicado e divergente da migration;
  pastas `infra/` em vez de `infrastructure/`; sem teste HTTP; OpenAPI sem
  nenhuma rota do Atlas (o teste de contrato falhava).
- A migration `000132` tinha o bloco `DO $$ … $$` sem
  `-- +goose StatementBegin/End`: o goose cortava o SQL nos `;` internos e
  a migração falhava — nenhum deploy com o Atlas passava da etapa
  `migrate`. Ela também gravava direto em `system_modules` (tabela do
  Kernel, que registra o módulo pelo `DefaultEnabled`).
- No `docker-compose.yml`, o mesmo commit trocou `MINIO_SECRET_KEY` por
  `MINIO_ROOT_PASSWORD` no worker — em produção o worker cairia no segredo
  default e o `config.Load` recusaria subir.

## Decisão

1. **Sem tenant.** Migration `000133` remove `tenant_id`; a unicidade passa
   a ser `(codigo_processual, versao)`. Ganha `created_by`, gatilho de
   `updated_at` e CHECKs que espelham o domínio.
2. **Busca nativa.** Typesense removido. `tsvector` gerado (com
   `nexus_unaccent`) em procedimentos, etapas e peças, índices GIN —
   mesma infraestrutura da Busca Global e do Catálogo.
3. **Rotas no padrão Catálogo:**
   - públicas (limite por IP, só ativos): `GET /atlas/ttdd`,
     `/atlas/ttdd/{codigo}`, `/atlas/workflows`, `/atlas/workflows/{id}`;
   - `POST /atlas/chat`: `atlas:read` + limite próprio por identidade
     (`ATLAS_CHAT_RATE_LIMIT_*`, padrão 10/min);
   - gestão `atlas:manage` (concessão **global** — o catálogo é
     institucional, sem dono na estrutura): `GET|POST /atlas/admin/workflows`,
     `GET /atlas/admin/workflows/{id}`, `POST .../{id}/ativar|desativar`.
4. **Domínio dono das regras:** validação completa (código, nível de
   acesso com hipótese legal obrigatória se restrito/sigiloso, etapas com
   ordem única, transições para etapas existentes, formatos, assinaturas,
   URL http(s)); cadastro, ativação e desativação com evento no outbox e
   auditoria na mesma transação.
5. **Grounding determinístico.** O índice recorta candidatos (OR dos
   termos); a relevância (0..1) é calculada no domínio — fração dos termos
   da pergunta presentes no procedimento, sem stopwords e sem acento, com
   bônus para código exato e título. Só procedimentos com relevância
   ≥ 0,65 sustentam a resposta (até 3); abaixo disso, recusa padronizada
   sem chamar o modelo.
6. **IA opcional.** `ATLAS_AI_ENDPOINT` (API de chat compatível com a
   OpenAI: OpenAI, vLLM, LiteLLM, Ollama) validado no `config.Load`.
   Sem endpoint, ou se o provedor falhar, a resposta é a **síntese
   canônica** gerada dos dados homologados (modo `sintese`). O corpo de
   erro do provedor não é registrado (pode ecoar o contexto).
7. **Auditoria da consulta sem o texto da pergunta** (minimização, LGPD
   art. 6º III): modo, relevância, códigos das fontes e tamanho.
8. `nivel_acesso` descreve o **processo** que nasce do procedimento
   (nível sugerido no SEI); o roteiro em si é transparência ativa e fica
   na consulta pública.

## Consequências

- A `000132` foi corrigida no próprio arquivo — exceção à regra de não
  editar migration publicada, segura porque ela nunca chegou a ser
  aplicada (falhava sempre, e o goose desfaz a transação). A `000133` vem
  em seguida; os dados semeados continuam.
- O overlay `docker-compose.atlas.yml` passa a ter só o Ollama (sem porta
  publicada); `make atlas-modelo` baixa o modelo.
- Perfis que devem usar o assistente precisam de `atlas:read` (o
  administrador já tem `*`).
- Frontend: tipos em `lib/nexus/types.ts`, página
  dividida em componentes (`components/atlas/`), tokens do design system
  (tema escuro e alto contraste), lista navegável por teclado e detalhe
  carregado da API (a listagem não traz etapas).
