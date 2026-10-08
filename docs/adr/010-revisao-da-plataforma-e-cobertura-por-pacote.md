# 010 — Revisão da plataforma e meta de 100% de cobertura por pacote

- **Status:** Aceito
- **Data:** 2026-09-26
- **Escopo:** todos os pacotes de `internal/platform/*`, a composição (`internal/app`), `pkg/httputil`, o CI

## Contexto

O ADR 009 levou cada plug-in a 100% de cobertura. O núcleo em que todos eles
se apoiam não tinha a mesma exigência. Esse núcleo é:

- o Kernel;
- o outbox e a mensageria;
- a LGPD;
- a autenticação;
- os limitadores;
- o armazenamento;
- a composição de dependências.

Parte dele nunca rodava contra a infraestrutura de verdade. Um erro ali
atinge todos os módulos de uma vez.

Aplicamos a este núcleo o mesmo método usado nos plug-ins. Para cada pacote:

1. ler a responsabilidade dele;
2. confrontá-la com o código;
3. corrigir o que não se sustentava;
4. cobrir cada caminho com teste, inclusive as falhas de banco, broker e armazenamento.

## Decisão

### 10.1. Correções por pacote

| Pacote | Principais correções |
|---|---|
| **kernel** | O backoff do `LISTEN` volta ao mínimo quando a conexão se restabelece, e o erro é logado.<br>O supervisor zera o backoff depois de uma execução longa. |
| **outbox** | Cada evento tem o seu próprio backoff (`next_attempt_at`, migration 000123) e há um teto de 20 tentativas.<br>O lote para na primeira falha de publicação e é abortado se não conseguir registrar o status.<br>Nova rota `POST /monitoring/outbox/requeue` para reprocessar eventos com falha. Ela exige a permissão nova `monitoring:manage`, é auditada e aparece como botão no painel de Monitoramento. |
| **lgpd** | A capacidade `PersonalData` passa a ser implementada pelos plug-ins que guardam dados pessoais (hoje, o Diretório).<br>O pacote de exportação traz os dados de cada módulo; se algum falha, a resposta é 500, nunca um pacote incompleto.<br>A eliminação anonimiza os dados dos módulos na mesma transação e tenta de novo até 5 vezes (migration 000124). |
| **messaging** | O retry de uma mensagem volta só para a fila que falhou. Antes, ele era republicado no exchange e reentregue a todas as filas inscritas.<br>Corrigida uma corrida na reconexão. Os intervalos de backoff passam a ser fixados por conexão.<br>Uma `RABBITMQ_URL` malformada agora falha na hora, sem esperar os 45 s de tolerância do boot. |
| **audit** | A cópia WORM exporta cada dia numa transação própria, sob um advisory lock do Postgres (`pg_try_advisory_xact_lock`). Com várias réplicas do worker, só uma exporta; as outras desistem até o próximo tick. Antes, duas réplicas liam a mesma marca d'água e gravavam o mesmo dia duas vezes no bucket imutável.<br>O registro da exportação na trilha acontece depois do commit: se falhar, só gera aviso, sem desfazer a marca d'água de objetos já gravados. |
| **storage** | `Get` agora devolve `ErrObjectNotFound` na hora para um objeto inexistente, em vez de o erro aparecer no meio da leitura.<br>Uma retenção WORM zero ou negativa é recusada. |
| **idempotency** | Uma chave abandonada (a requisição morreu no meio) pode ser retomada depois de 5 minutos.<br>A query string entra no hash da requisição. |
| **localauth** | O teto do bloqueio progressivo deixa de depender do formato de texto de `time.Duration`. |
| **netguard** | Passam a ser bloqueadas as formas IPv6 que embutem um IPv4 privado: IPv4-compatible, NAT64 local, Teredo e 6to4. |
| **transparency** | O XML dos dados abertos é escapado.<br>Uma falha de contagem responde 500 em vez de publicar um zero falso.<br>`?dias` fora de 1–365 responde 400. |
| **keycloakconfig** | O segredo salvo nunca é enviado a outro issuer, nem no teste de conexão nem ao salvar. |
| **branding** | O logo aceita só `https://` ou um caminho local de fato (`//host` e `/\host` são recusados).<br>Tokens de cor com contraste abaixo do WCAG AA (4,5:1) são recusados, conforme exige o e-MAG. |
| **ws** | Derrubar um tópico avisa os clientes (`topic.dropped`), e o Mercúrio mostra o aviso na tela.<br>O intervalo de ping/pong passa a ser configurável. |
| **logging** | A máscara de PII também vale para erros e `fmt.Stringer`. |
| **metrics** | Os gauges do pool do Postgres são registrados uma única vez. Um segundo registro só troca o pool observado, em vez de entrar em pânico. |
| **telemetry** | Uma falha ao criar o exporter OTLP desliga o tracing e registra o erro, sem derrubar a API. |
| **httputil** | `Query` agora corta por caractere, não por byte. Cortar no meio de um "ç" gerava UTF-8 inválido, que o Postgres rejeitava com um 500. |
| **app** | `NewDependencies` foi separado em `config.Load` e `build`. A declaração da topologia do RabbitMQ virou uma função testável.<br>`supervised` fixa o seu backoff quando é criado.<br>O servidor de métricas do worker espera o listener encerrar. |

### 10.2. Código removido

Removido o código sem uso em produção, que só inflava a superfície:

- o pacote `jobs`, morto desde o Microkernel;
- o limitador de taxa em Postgres;
- o circuit breaker (`resilience`) e as métricas sem uso, junto com a dependência `gobreaker`;
- o `InMemoryLimiter` do httpserver, usado só em teste.

### 10.3. Infraestrutura de teste real

O CI roda, além do Postgres:

- **RabbitMQ** (serviço do job), para confirms, retry, DLQ e reconexão, e para o Relay do outbox contra o broker de verdade;
- **MinIO** (subido num passo do job), para operações de objeto, URLs pré-assinadas e object-lock/WORM.

As variáveis de ambiente são `TEST_RABBITMQ_URL`, `TEST_MINIO_ENDPOINT`,
`TEST_MINIO_ACCESS_KEY` e `TEST_MINIO_SECRET_KEY`. Sem elas, os testes
correspondentes são pulados.

`internal/app` monta as dependências reais, via `build`, contra essa
infraestrutura e prova duas coisas:

- cada dependência obrigatória falha rápido, com a etapa no erro: Postgres, Redis, assinador, OIDC, cifra, proxies, RabbitMQ, MinIO, Kernel e topologia;
- as opcionais só geram aviso: o bucket do MinIO e a configuração persistida do Keycloak.

### 10.4. Meta de cobertura

`scripts/coverage-gate.sh` passa a exigir 100% em:

- cada plug-in de `internal/modules/*`, como no ADR 009;
- cada pacote de `internal/platform/*`, menos os dublês de teste `dbtest` e `storagetest`;
- `internal/app`;
- cada pacote de `pkg/*`.

O perfil vem de `go test -race -p 1 -coverpkg=./internal/...,./pkg/...`.

### 10.5. Pendências do ADR 008

**Schemas do contrato gerados a partir do código.** Os schemas saem de
`internal/app/openapi_schemas_test.go`. O passo a passo:

1. o teste acha, pelo nome em runtime, a função que atende cada rota montada;
2. analisa essa função estaticamente, com `go/packages` e `go/types`;
3. extrai da análise:
   - o tipo passado a `httputil.Bind`/`DecodeJSON`, que vira o corpo da requisição;
   - o tipo passado a `WriteOK`, `WriteCreated`, `WriteAccepted`, `WritePage` e `WriteJSON`, que vira o `data` da resposta, com o código HTTP;
   - `WriteNoContent`, que vira o 204;
   - `httputil.Query`, `Page`, `OptionalUUIDQuery` e `r.URL.Query().Get`, que viram parâmetros de query.

Os tipos Go viram `components/schemas`:

| No Go | No schema |
|---|---|
| tag `json` | nome da propriedade |
| ponteiro | `nullable` |
| `validate` (required, max, min, oneof, email, uuid) | `required`, `maxLength`/`maxItems`/`maximum`, `enum`, `format` |
| constantes declaradas com o tipo | `enum` |

Das 172 rotas, só uma continua com schema genérico: a de transparência que
negocia CSV/XML.

Tudo o que é gerado leva a marca `x-nexus-generated`, e regenerar
(`UPDATE_OPENAPI=1`) refaz o que tem a marca. Onde havia texto escrito à mão,
vale uma regra única:

- **Handler devolve um struct:** o código é a fonte da verdade e a resposta é
  derivada dele. A `description` escrita à mão é mantida.
- **Handler devolve um `map` ou negocia CSV/XML:** o texto à mão prevalece,
  porque o código não tem tipo.
- **Componente à mão com a mesma forma do gerado:** é reaproveitado. A versão
  à mão pode acrescentar `enum`, `format` ou limites que o tipo Go não
  expressa.

Essa comparação mostrou que três entradas escritas à mão estavam erradas:

- `GET /me` documentava um usuário do banco, não a identidade da sessão;
- `GET /users` e `/users/{id}` estavam sem 6 a 7 campos e sem as lotações;
- `GET /lgpd/meus-dados` não tinha schema.

O teste de contrato também compara o arquivo com o resultado da geração:
mudar um DTO sem regenerar quebra o CI.

Duas regras de fidelidade ao `encoding/json`:

- **Ponteiro com `omitempty`** nunca sai como `null`, porque o nil é omitido.
  Só o ponteiro sem `omitempty` é `nullable`.
- **`required` depende do sentido.** Na resposta, marca o campo sempre
  presente no JSON (sem `omitempty`). Na requisição, marca o que o cliente
  precisa enviar (`validate:"required"`). Um struct usado nos dois sentidos
  ganha a variante `XInput` quando as duas regras dão resultados diferentes.

**Contrato frontend × backend.** O compilador também confere o frontend:

- `npm run gen:api` gera `src/types/openapi.gen.ts` a partir do OpenAPI, com `openapi-typescript`;
- `src/lib/nexus/contract.types.ts` faz o `tsc` provar que cada tipo escrito à
  mão em `types.ts` aceita o que o backend envia; a comparação é de forma, e
  um literal como `"noticia"` conta como string;
- no CI, um passo regenera os tipos e falha se o arquivo estiver defasado.

Na primeira verificação, o compilador acusou 36 divergências:

- a maioria era imprecisão do gerador (ponteiro com `omitempty` marcado como `nullable`) ou o `required` com um sentido só;
- três eram reais, e foram corrigidas:
  - `PaginationMeta` sem `required`;
  - os dois structs de duplo uso;
  - `AuditRecord.entity_context` e `metadata` tipados como objeto, embora
    sejam JSON livre e `entity_context` possa vir `null`. A tela agora
    confere o tipo antes de ler as chaves. De passagem, três respostas escritas
à mão estavam sem `description`, o que tornava o documento inválido. Foram
corrigidas, e o teste passou a exigir o campo.

**Grafo visual dos módulos.** A tela Configurações > Módulos ganhou a visão
"Grafo de dependências", ao lado dos cartões. O desenho funciona assim:

- é feito em camadas (`src/lib/modules/graph.ts`): cada módulo fica uma coluna à direita da sua dependência mais profunda;
- a ordem dentro de cada coluna segue o baricentro das dependências, o que reduz os cruzamentos;
- as setas vão da dependência ao dependente, e a ligação é tracejada quando uma das pontas está inativa;
- os módulos sem ligação nenhuma ficam numa grade à parte, para não esconder as cadeias.

Cada nó é um botão acessível (clique, Enter ou espaço; Esc limpa a seleção)
com o estado e as ligações no rótulo. Selecionar um nó faz duas coisas:

- destaca as dependências e os dependentes transitivos;
- abre um painel com o interruptor do módulo.

O interruptor usa o mesmo plano de cascata e a mesma confirmação da visão em
cartões.

## Consequências

- Uma regressão no núcleo agora quebra o CI com a linha descoberta, como já acontecia com os plug-ins.
- As costuras de teste são campos ou variáveis de pacote documentados: intervalos, `begin` e `subscribe` injetáveis, `newExporter`, `lookupNetIP`. Nenhuma muda o comportamento em produção.
- Os testes de infraestrutura dependem de serviços que o CI provê. Localmente eles são pulados, a menos que as variáveis estejam definidas (ver `GUIDE_DESENVOLVEDOR.md`).
