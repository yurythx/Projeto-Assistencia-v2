# 008 — Kernel: grafo de módulos, invariantes de segurança do IAM e estratégia de testes

- **Status:** Aceito
- **Data:** 2026-09-26
- **Escopo:** `internal/platform/kernel`, `internal/modules/iam`, `internal/platform/audit`, `internal/platform/lgpd`, frontend (Configurações → Módulos), CI

## Contexto

A migração para o Microkernel (skill *enterprise-microkernel-architect*)
deixou o backend compilando, com os plug-ins registrados, mas uma revisão
completa encontrou quatro lacunas:

1. **Grafo de dependências invisível.** O Kernel aplicava `DependsOn`
   (Trâmite → Signum), mas o status público só dizia `enabled`. A UI não
   tinha como mostrar quem depende de quem, nem explicar um módulo
   configurado como ativo e inativo por causa de uma dependência. Os 409
   citavam só o primeiro bloqueador, escolhido pela ordem de iteração de
   um map.
2. **Testes de integração que nunca rodavam.** Os testes que tocam o
   Postgres pulam sem `TEST_DATABASE_URL`, e o job de backend do CI não tinha
   banco. A cobertura real, medida com `-coverpkg`, era de 46,5%, e a camada
   HTTP dos módulos ficava entre 8% e 40%.
3. **Falhas que só aparecem pela API completa.** Com os testes de sistema
   escritos, apareceram escalada de privilégio no IAM, exclusões em cascata
   silenciosas, auditoria sem IP/User-Agent e consentimento LGPD nunca
   gravado para usuários federados. Os detalhes estão em *Decisão*.
4. **Contrato OpenAPI defasado.** O `/docs` descrevia rotas removidas
   (feature flags, integrações) e nenhuma das 100+ rotas dos plug-ins.

## Decisão

### 8.1. Grafo de módulos exposto nos dois sentidos

`GET /api/v1/system/modules` passa a devolver, por módulo:

| Campo | Significado |
|---|---|
| `enabled` | estado **efetivo**: configurado ativo e com todas as dependências (transitivas) ativas |
| `configured` | estado desejado gravado pelo administrador |
| `depends_on` | dependências declaradas no Manifest |
| `dependents` | quem declara dependência neste módulo |
| `blocked_by` | dependências diretas inativas agora |

Os erros `409` (`ErrDependencyMissing` e `ErrDependentActive`) listam
**todos** os módulos envolvidos, em ordem de registro. O frontend planeja a
cascata com uma função pura (`lib/modules/plan.ts`): desligar um módulo leva
antes os dependentes, de cima para baixo; ligar leva antes as dependências,
de baixo para cima. A cascata sempre pede confirmação. O backend continua
sendo a autoridade, porque recusa qualquer ordem inválida. O `ModuleGate`
troca a página de um módulo desativado por uma explicação, que inclui a
dependência que o bloqueia.

### 8.2. Invariantes de segurança do IAM (A01)

- **Ninguém concede o que não possui.** Permissões novas de um perfil, e os
  perfis atribuídos por lotação ou mapeamento do AD, precisam estar cobertos
  pelas permissões efetivas de quem concede.
- **`nexus-admin` equivale a `*`.** Só quem já tem acesso total concede ou
  retira esse papel.
- **Estrutura e perfis com `ON DELETE RESTRICT`** (migration 000121).
  Excluir uma entidade, unidade, departamento ou perfil em uso responde
  `409`, sem apagar em silêncio lotações e mapeamentos. Excluir o
  **usuário** continua levando as lotações dele.
- **Desbloqueio administrativo completo.** Limpa o banco e também o
  lockout progressivo distribuído no Redis, pela porta
  `modkit.Deps.ResetLoginLockout`.

### 8.3. Proveniência completa na auditoria

O middleware `audit.CaptureOrigin` roda depois de `TrustedRealIP` e guarda
no contexto o IP real e o User-Agent. `audit.FromContext`/`Meta` os incluem
em toda entrada gravada pelos casos de uso, cumprindo a skill §5
(`actor_id`, `actor_roles`, `ip`, `user_agent`, `entity_context`, diff,
hash encadeado).

### 8.4. Consentimento LGPD de usuários federados

`/lgpd/status` e `/lgpd/accept` resolvem o id interno pelo IAM, como os
endpoints do titular (art. 18) já faziam. O aceite só vale para a versão
**vigente** dos termos (`409 TERM_OUTDATED`). O modal consulta o servidor
para usuários logados e usa o proxy público para visitantes.

### 8.5. Operações destrutivas exigem intenção explícita

- **Arquivos.** Excluir uma pasta com conteúdo exige `?recursive=true`,
  caso contrário a resposta é `409 FOLDER_NOT_EMPTY`.
- **Trâmite.** Processo sigiloso é estritamente nominal: nem
  `tramite:manage` lê sem credencial, e a resposta é 404, sem revelar que o
  processo existe. A unidade de origem credencia o destinatário **antes** de
  tramitar, e a interface avisa disso.

### 8.6. Estratégia de testes (pirâmide)

| Nível | Local | Garante |
|---|---|---|
| Unidade | pacotes Go / `frontend/src/**/*.test.ts(x)` | domínio, Kernel (grafo, transitividade, NOTIFY entre réplicas), PII, componentes |
| Sistema | `internal/app/*_http_test.go` (router real + Postgres + Redis em memória + storage em memória) | cada módulo pela API: 403, 422, fluxos, auditoria e outbox na mesma transação |
| Ativação | `internal/app/modules_toggle_test.go` | para **cada** plug-in, as rotas são descobertas pelo próprio `RegisterRoutes` e todas respondem `404 MODULE_DISABLED` desligadas e voltam religadas; um plug-in novo entra sozinho |
| Contrato | `internal/app/openapi_test.go` | o OpenAPI descreve exatamente as rotas montadas; `UPDATE_OPENAPI=1` regenera preservando o que já foi escrito à mão |

O CI provisiona Postgres 16, aplica as migrations e roda
`go test -race -p 1 -coverpkg=./internal/...`. A cobertura do backend foi
de 46,5% para 70%, e o frontend foi de 134 para 174 testes.

### 8.7. Convenções REST

- Criação responde `201 Created`; os módulos IAM, Agenda (salas), Mercúrio
  (salas) e Egress foram alinhados a isso.
- Mutações aceitam `X-Idempotency-Key`. O `apiClient` do frontend gera uma
  chave por requisição e o BFF a repassa ao backend.

## Consequências

- **Positivas.** O grafo de módulos fica visível e operável sem conhecer o
  código. Regressões de ativação, autorização ou contrato quebram o CI. As
  invariantes de segurança do IAM passam a estar codificadas e testadas.
- **Custos.** O job de backend fica mais lento, por causa do Postgres e das
  migrations. Excluir itens da estrutura organizacional agora exige remover
  antes os vínculos, o que é intencional.
- **Pendências.** O esqueleto gerado para as rotas no OpenAPI usa corpo e
  resposta genéricos: os schemas detalhados de cada módulo podem ser
  descritos à mão, e o gerador os preserva. A visualização gráfica do grafo
  (hoje em listas) fica para uma próxima iteração. *(Resolvidas no ADR 010,
  seção 10.5.)*
