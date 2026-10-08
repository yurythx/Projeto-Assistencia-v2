# Estrutura, funcionamento e regras de negócio

Referência funcional da plataforma. Serve para quem **implanta e administra**
(como modelar a organização, quem pode o quê) e para quem **desenvolve**
(quais regras cada módulo garante). Tudo aqui reflete o código em
`backend/internal`. Como programar um módulo está em
[GUIDE_DESENVOLVEDOR.md](GUIDE_DESENVOLVEDOR.md) e
[GUIDE_MODULOS.md](GUIDE_MODULOS.md). Como implantar está em
[DEPLOY.md](DEPLOY.md).

---

## 1. Como a plataforma funciona

### 1.1. O caminho de uma requisição

1. **Login.** O login principal é pelo Keycloak (OIDC), que pode federar o
   Active Directory. Se o Keycloak ficar indisponível, há o login local de
   contingência. O token traz quem é a pessoa e os **grupos** dela.
2. **Identidade efetiva.** A cada requisição, o IAM da API:
   - cria ou atualiza o usuário na primeira vez que ele entra
     (provisionamento automático, com nome, e-mail e grupos);
   - soma as **permissões** de todos os perfis que a pessoa recebeu, seja
     por **lotação manual** ou por **mapeamento de grupo**;
   - monta a lista de **escopos** (onde a pessoa está lotada).

   O resultado fica em cache por poucos segundos. Quando um administrador
   muda perfis, lotações ou mapeamentos, o cache é invalidado em todas as
   instâncias e a mudança vale **na hora**.
3. **Módulo.** A rota exige a permissão necessária. Depois, o próprio
   módulo aplica as regras dele: unidade, sigilo, dono, ACL.
4. **Efeitos.** Toda alteração grava, na mesma transação:
   - o **registro de auditoria**, imutável e encadeado por hash;
   - o **evento de domínio** no outbox.

   O worker publica esse evento no RabbitMQ, e daí ele segue para o tempo
   real (WebSocket), para os webhooks (Egress) e para a busca.

### 1.2. Módulos (Kernel)

| Tipo | Módulos | Pode desligar? |
|---|---|---|
| Núcleo | IAM & Usuários, Auditoria | Não |
| Plug-ins | Mercúrio, Egress, Blog, Catálogo, Contato, Diretório, Agenda, Arquivos, Wiki, Busca, Signum, Trâmite, Atlas, Módulo Modelo | Sim, em Configurações → Módulos (`modules:manage`) |

- Desligar um módulo tem efeito imediato, sem reiniciar:
  - as rotas dele respondem `404 MODULE_DISABLED`;
  - ele some do menu, do site público, do sitemap e da busca;
  - as assinaturas de tempo real dele caem;
  - os workers e consumidores de fila dele param.
- **Dependências:** o Trâmite depende do Signum. Não é possível desligar o
  Signum com o Trâmite ligado, nem ligar o Trâmite com o Signum desligado.
  A tela mostra esse grafo.
- **Módulos públicos** (Catálogo, Contato, Diretório, Agenda e Signum) têm
  páginas no site institucional, que funcionam sem login.

### 1.3. Garantias transversais

| Garantia | Como |
|---|---|
| Auditoria de toda alteração | Tabela só de inclusão (o banco impede alterar ou apagar), cadeia SHA-256 verificável, exportação LAI (CSV/JSON/XML) e cópia diária imutável (WORM) |
| Consentimento LGPD | Aceite registrado com versão do termo e IP (anônimo ou da conta); direitos do titular: acesso, portabilidade e pedido de exclusão |
| Limites de abuso | Por identidade na API (600/min), por IP nas rotas públicas e no login, e bloqueio progressivo de login local (ver [DEPLOY.md](DEPLOY.md#segurança-do-login-local)) |
| Idempotência | Repetir uma alteração com a mesma chave devolve a resposta original, sem duplicar |
| Dados pessoais nos logs | Mascarados (CPF, e-mail, telefone) |

---

## 2. Estrutura organizacional

```
Entidade (ex.: Prefeitura, Secretaria de Saúde, Empresa A)
└── Unidade (ex.: UPA 24h, Escola, Filial) — pode ter unidade-mãe
    └── Departamento (ex.: Administração, RH, TI)
```

### 2.1. Entidade

É o órgão ou a organização de nível mais alto: prefeitura, secretaria,
autarquia, empresa.

- Campos: nome, sigla, slug (endereço), documento (CNPJ ou outro) e ativo.
- O slug é gerado a partir do nome quando não é informado, e é único.

### 2.2. Unidade

É o local ou a subdivisão onde as pessoas trabalham: escola, posto de
saúde, filial, sede.

- Pertence a **uma** entidade e **não pode mudar de entidade** depois de
  criada, porque as lotações gravam a entidade da unidade.
- Pode ter uma **unidade-mãe**, o que forma uma hierarquia. A mãe precisa
  ser da **mesma entidade**, e ciclos são recusados.
- Campos: nome, sigla, slug (único dentro da entidade), grupo do AD de
  referência, e-mail, telefone, endereço e ativo.
- **É o nível que o Trâmite usa.** Processos são abertos, ficam e são
  tramitados entre **unidades**.

### 2.3. Departamento

É o setor dentro de uma unidade: Administração, Atendimento, RH.

- Pertence a **uma** unidade e **não pode mudar de unidade**.
- Campos: nome, sigla, slug (único dentro da unidade), grupo do AD de
  referência, e-mail, telefone e ativo.
- **Todo departamento pertence a uma unidade.** Uma organização que só tem
  departamentos (como a Prefeitura no ambiente de teste) usa uma unidade
  "Sede" para agrupá-los.

### 2.4. Regras comuns

- **Exclusão protegida:** excluir entidade, unidade ou departamento **em
  uso** responde `409`. Nada é apagado em cascata; primeiro se remove o que
  depende, depois o item.

  **Bloqueiam a exclusão:**
  - unidades de uma entidade, departamentos de uma unidade e subunidades
    de uma unidade-mãe;
  - lotações e mapeamentos de grupo que apontam para o item;
  - processos do Trâmite (origem, unidade atual e histórico).

  **Não bloqueiam; a referência é apenas limpa:**
  - unidade responsável de um serviço do Catálogo;
  - lotação exibida no Diretório;
  - departamento de uma sala do Mercúrio.
- **"Ativo" — desativar é tirar de operação sem apagar:**
  - lotações e mapeamentos de grupo cujo escopo tenha entidade, unidade ou
    departamento desativado **deixam de conceder** o perfil, na hora
    (salvar a estrutura invalida o cache do IAM); reativar devolve;
  - desativar a entidade vale para tudo que está abaixo dela;
  - o Trâmite recusa (`422`) abrir processo em unidade desativada, ou de
    entidade desativada, e tramitar para ela — inclusive para quem tem
    `tramite:manage`. Processos que já estão lá continuam consultáveis;
  - o Diretório e as telas do Trâmite deixam de oferecer o que está
    desativado.

  A regra do escopo ativo é única, `nexus_scope_active` (migration 000126),
  usada tanto pelo cálculo de permissões por requisição quanto pela tela de
  usuários.
- Toda alteração é auditada com o antes e o depois.
- Quem administra: permissão `iam:manage`.

---

## 3. Perfis e permissões

### 3.1. Permissões

- Formato `recurso:ação` (ex.: `tramite:create`). Há dois curingas:
  `recurso:*` (todas as ações do recurso) e `*` (tudo).
- Cada módulo declara as próprias permissões. A lista completa, com
  descrições, sai em `GET /api/v1/iam/permissions` e aparece na tela de
  perfis.

| Módulo | Permissões |
|---|---|
| IAM | `users:read`, `users:manage`, `iam:manage`, `modules:manage`, `keycloak:manage`, `ia:manage`, `branding:manage`, `monitoring:read`, `monitoring:manage` |
| Auditoria | `audit:read`, `audit:verify` |
| Trâmite | `tramite:create`, `tramite:route`, `tramite:manage` |
| Blog / Catálogo / Wiki | `blog:manage`, `catalog:manage`, `wiki:manage` |
| Contato | `contact:read`, `contact:manage` |
| Agenda / Arquivos / Diretório | `calendar:manage`, `files:manage`, `directory:manage` |
| Mercúrio / Signum / Egress | `mercurio:manage`, `signum:manage`, `egress:manage` |
| Atlas | `atlas:read`, `atlas:manage` |
| Módulo Modelo | `example:manage` |

O que **não** exige permissão, só estar autenticado:
- ler Blog e Wiki, criar e editar páginas da Wiki;
- usar o Mercúrio, a Agenda (eventos próprios), os Arquivos (pastas
  próprias e compartilhadas) e o Diretório;
- criar envelopes no Signum e usar a Busca.

Sem login nenhum: consultar os procedimentos ativos e a Tabela de
Temporalidade do Atlas (o assistente do Atlas exige `atlas:read`).

### 3.2. Perfis

Um perfil é um **conjunto nomeado de permissões**. Perfis de sistema (vêm
com a plataforma):

| Perfil | Permissões | Para quem |
|---|---|---|
| `administrador` | `*` | Administração da plataforma |
| `auditor` | `audit:read`, `audit:verify`, `users:read`, `monitoring:read` | Controle interno |
| `gestor-iam` | `users:read`, `users:manage`, `iam:manage` | Quem cadastra estrutura, usuários e lotações |
| `gestor-conteudo` | `blog:manage`, `catalog:manage`, `wiki:manage`, `contact:read`, `contact:manage` | Comunicação e ouvidoria |
| `protocolo` | `tramite:create`, `tramite:route` | Quem abre e tramita processos |
| `servidor` | *(nenhuma)* | Colaborador comum, só o que não exige permissão |

Regras:
- É possível criar **perfis customizados** com qualquer combinação de
  permissões válidas. Permissão malformada é recusada.
- Perfis de sistema não são excluídos e mantêm o slug. O `administrador`
  **sempre** mantém `*` e fica ativo, para que um erro de edição não tranque
  a plataforma.
- **Desativar um perfil** retira na hora as permissões dele de todos que o
  têm. Reativar devolve.
- A role de realm `nexus-admin` no Keycloak equivale a `*`.

### 3.3. Ninguém concede o que não tem (anti-escalada)

Criar ou editar um perfil, lotar alguém ou mapear um grupo exige que
**quem concede** tenha todas as permissões envolvidas. Retirar também exige.
Na administração delegada (seção 5), vale **no escopo da lotação**: o
delegado só concede uma permissão que ele próprio tem ali.
Exemplo: o `gestor-iam` (que não tem `audit:read`) não cria um perfil com
`audit:read` nem lota alguém num perfil com essa permissão; recebe `403`.
Só quem tem acesso total concede ou retira `nexus-admin`.

---

## 4. Lotações e mapeamentos de grupo

Há dois jeitos de alguém receber um perfil. Os dois resultam na mesma coisa:
**perfil + escopo**.

| | Lotação manual | Mapeamento de grupo |
|---|---|---|
| O que é | "Usuário X tem o perfil P no escopo E" | "Quem está no grupo G do Keycloak/AD tem o perfil P no escopo E" |
| Onde se cadastra | Configurações → Usuários → Lotações | Configurações → Mapeamento AD |
| Quando vale | Enquanto existir | Enquanto a pessoa estiver no grupo (lido do token a cada login ou renovação) |
| Uso típico | Exceções e acúmulos | Regra geral: departamento do AD → perfil + departamento |

- **Escopo:** entidade, unidade e/ou departamento. Tudo vazio significa a
  plataforma inteira. O escopo é **completado e validado**:
  - informar só o departamento preenche a unidade e a entidade;
  - informar unidade e departamento de unidades diferentes é recusado.
- Uma pessoa pode ter **várias** lotações, por exemplo trabalhar em duas
  unidades.
- O grupo é comparado sem diferenciar maiúsculas e aceita o caminho do
  Keycloak (`/SEMSA/SEMSA-UPA/SEMSA-UPA-ADM` ou `SEMSA-UPA-ADM`).
- Cada lotação é uma **concessão** separada: as permissões do perfil
  valem no escopo daquela lotação, não somadas para a plataforma toda
  (seção 5). O menu mostra o que a pessoa pode fazer em **algum** lugar.
- Mudanças valem na próxima requisição da pessoa, sem novo login.

---

## 5. Onde o escopo vale (e onde não vale)

> **Regra central (ADR 013):** cada lotação é uma **concessão**: um perfil
> num lugar. As permissões do perfil valem **só onde a concessão foi dada**
> e **para baixo** — a unidade cobre as subunidades e os departamentos
> dela; a entidade cobre todas as unidades. Concessão sem escopo é
> **global**.

**Permissões com escopo** (valem onde foram concedidas):

| Permissão | Onde o recurso "está" |
|---|---|
| `tramite:create`, `tramite:route`, `tramite:manage` | unidade de origem ou atual do processo |
| `catalog:manage` | unidade responsável do serviço |
| `blog:manage` | unidade dona do post |
| `wiki:manage` | unidade dona da página (herdada da página-mãe) |
| `calendar:manage` | unidade dona da sala (e dos eventos reservados nela) |
| `mercurio:manage` | departamento da sala |
| `directory:manage` | lotação exibida da pessoa |
| `contact:read`, `contact:manage` | setor para onde a triagem encaminhou a mensagem |
| `audit:read` | lotação do autor da ação, gravada no registro |
| `files:manage` | unidade dona da pasta (ou de uma pasta acima) |
| `iam:manage`, `users:read`, `users:manage` | estrutura, lotações e contas da área (ver IAM, seção 7) |

**Permissões de plataforma** (`modules:manage`, `keycloak:manage`, `ia:manage`,
`branding:manage`, `monitoring:*`, `egress:manage`, `audit:verify`,
`signum:manage`): **só valem com concessão global**. Dadas com escopo, não
valem em lugar nenhum.

**Institucional:** recurso sem unidade dona (tudo o que existia antes do
ADR 013, e o que a gestão global criar sem dona). Só a gestão global o
gerencia.

**A leitura:** o escopo restringe a **gestão**. Para restringir a
**leitura**, Blog, Wiki e Agenda têm o **público-alvo** (ADR 014): o
conteúdo pode ser só para uma ou mais secretarias e/ou unidades. Sem
público-alvo, tudo é visível a todos como antes; o Catálogo publicado
segue sempre aberto (é do cidadão).

**Público-alvo (ADR 014):**
- **Quem pertence:** quem tem lotação (manual ou por grupo do AD, de
  qualquer perfil) numa secretaria do público, ou numa unidade dele **ou
  abaixo dela** (a lotação num departamento conta na unidade dele).
  Concessão só global não é pertencimento.
- **Quem mais vê:** o autor (organizador, no evento) e a gestão que cobre
  o conteúdo — a do módulo na unidade dona — e a gestão global.
- Conteúdo com público-alvo **nunca** vai ao site público; não é
  difundido a todos em tempo real; a Busca Global respeita.
- Excluir uma secretaria ou unidade que é público de algum conteúdo é
  recusado (senão o conteúdo ficaria aberto a todos em silêncio).
- Público-alvo direciona a leitura; **não é sigilo** (o sigilo de
  processos é regra do Trâmite).

Além disso, **Arquivos** usa as lotações nas ACLs das pastas (`unidade`,
`departamento`, `perfil`) e o **Trâmite** usa a lotação para abrir e
tramitar.

Consequência prática: lotar alguém como `gestor-conteudo` **na UPA** dá
`blog:manage` e `catalog:manage` **só na UPA** (e nas subunidades dela).
Para gerir a plataforma inteira, a concessão precisa ser global. O
relatório `make iam-scope-report` mostra, por lotação, o que vale e o que
não vale.

---

## 6. Como modelar uma implantação

1. **Entidades:** uma por órgão (ou empresa).
2. **Unidades:** os locais de trabalho, que no Trâmite são quem "recebe"
   processos. Use unidade-mãe se houver hierarquia.
3. **Departamentos:** os setores de cada unidade.
4. **Grupos no Keycloak/AD:** idealmente um por departamento, com nome
   único (ex.: `SEMSA-UPA-ADM`), e grupos de papel (`NEXUS-ADMINS`,
   `NEXUS-AUDITORES`…).
5. **Mapeamentos:**
   - cada grupo de departamento → `servidor` no departamento;
   - quem protocola (ex.: a Administração da unidade) → `protocolo` na
     unidade;
   - grupos de papel → perfis de gestão, sem escopo.
6. **Exceções:** lotações manuais.

O ambiente de teste segue exatamente essa receita
(`scripts/demo-data/generate.mjs`), com 4 entidades, 15 unidades e 76
departamentos.

**Várias organizações independentes** (ex.: duas empresas): a estrutura se
modela sem código, com uma entidade por empresa. Mas a plataforma **não
isola** as empresas entre si. Blog, Wiki, Catálogo, Agenda pública, a sala
"Geral" do Mercúrio, o Diretório, a identidade visual e o site são
compartilhados, e o administrador vê tudo. Para empresas independentes, use
**uma implantação por empresa**.

---

## 7. Regras por módulo

### Trâmite (processos administrativos)
- **Tipos:** Contratação, Memorando, Ofício, Requerimento, Outros. Tipo
  desativado não abre processo novo.
- **Numeração:** `NNNNNN/AAAA`, sequencial por ano.
- **Estados:** `aberto` → `em_tramitacao` → `concluido` → `arquivado`.
  Reabrir (`tramite:manage` na unidade atual) volta para `em_tramitacao`. Só se arquiva o
  que está **concluído**.
- **Abrir:** exige `tramite:create` e lotação na unidade de origem, que
  precisa estar ativa (e a entidade dela também).
- **Tramitar:** exige `tramite:route`, o processo na sua unidade e um
  destino ativo, diferente da unidade atual.
- **Sigilo:**

  | Sigilo | Quem vê |
  |---|---|
  | público | qualquer autenticado |
  | restrito | autor, credenciados, lotados na unidade de origem ou na atual, e `tramite:manage` que cubra uma delas |
  | sigiloso | **somente** o autor e os credenciados nominalmente (nem a unidade nem o gestor) |

  Credenciar ou revogar acesso nominal só existe para processo não público
  e dá **leitura**.
- **Documentos:**
  - nascem em rascunho;
  - são editáveis só pela unidade que está com o processo;
  - anexos vão para o MinIO por URL pré-assinada;
  - **pedido de assinatura** abre um envelope no Signum, e o documento
    assinado não pode mais ser editado;
  - não se conclui processo com assinatura pendente.
- Processo concluído ou arquivado não recebe alterações. Cada movimento
  (abertura, tramitação, conclusão, arquivamento, reabertura) fica no
  histórico.

### Signum (assinatura eletrônica)
- Qualquer autenticado abre um envelope com o **hash SHA-256** do
  documento e de 1 a 50 signatários ativos, em ordem sequencial ou livre.
- **Cerimônia:**
  1. o signatário pede um **desafio** (uso único, expira em
     `SIGNUM_CHALLENGE_TTL`, padrão 5 min);
  2. confirma o hash do documento;
  3. **reautentica com a senha**: conta local contra o banco, conta do
     Keycloak via Keycloak/AD, pelo client `nexus-backend`.
- **Sequencial:** ninguém assina antes dos anteriores.
- **Estados:** `pending` → `completed` (todos assinaram) | `refused`
  (alguém recusou com motivo) | `cancelled` (dono ou `signum:manage`
  cancelou). Envelope não pendente não aceita ação.
- Tentativas de senha erradas são limitadas por usuário.
- **Verificação pública** pelo id do envelope, sem login.
- Veem o envelope: o dono, os signatários e `signum:manage`.

### Arquivos
- Pastas em árvore e arquivos no MinIO. Upload e download são feitos
  **direto** pelo navegador, por URL pré-assinada (15 min); o limite é
  `UPLOAD_MAX_FILE_MB` (100 MB).
- **Acesso a uma pasta:**
  - `files:manage` global tem acesso total;
  - `files:manage` com escopo tem acesso total às pastas cuja **unidade
    dona** (da própria pasta ou de uma acima) ele cobre;
  - o **dono** da pasta, ou de qualquer pasta acima dela, tem acesso total;
  - a **ACL** de qualquer nível acima (herança) dá leitura, e escrita se
    `can_write`.
- Sujeitos da ACL: `everyone`, `user`, `perfil`, `ad_group`, `unidade`,
  `departamento`.
- Alterar a ACL, renomear ou excluir a pasta exige acesso total.
- **Unidade dona** (opcional): marca quem está lotado na unidade ou tem
  `files:manage` cobrindo-a; trocar exige poder marcar a antiga e a nova.
  Levar para a raiz uma pasta cuja dona era herdada exige ser o dono dela
  ou marcá-la antes.
- Nomes são únicos dentro da pasta. Pasta com conteúdo só é excluída com
  confirmação recursiva. Não se move uma pasta para dentro de si mesma.

### Mercúrio (chat)
- **Salas:**
  - **global** (todos);
  - **de departamento**: acesso por grupo do AD ou lotação no
    departamento, e `mercurio:manage`;
  - **direta** (duas pessoas; não se abre conversa consigo mesmo).
- Criar ou arquivar sala e moderar mensagens: `mercurio:manage` cobrindo
  o departamento da sala (sala global e direta: só a gestão global). Sala
  arquivada é só leitura.
- Mensagens de 1 a 4.000 caracteres. O autor **edita nos primeiros 15
  minutos** e pode excluir. Tudo chega em **tempo real**.
- Não lidas por sala; "marcar como lida" zera o contador.

### Agenda
- Eventos com visibilidade:
  - **público:** aparece no site;
  - **interno:** todos os autenticados;
  - **privado:** só o organizador.
- Duração de até 31 dias.
- Salas com capacidade, recursos e **unidade dona**: criar, editar e
  excluir exigem `calendar:manage` cobrindo a dona (sala sem dona é
  institucional). Salas inativas aparecem só para a gestão que as cobre. **Sem conflito:**
  uma sala não aceita duas reservas no mesmo horário. Sala inativa não
  aceita reservas, e sala com reservas futuras não é excluída (desative).
- Só o organizador ou `calendar:manage` na unidade dona da **sala
  reservada** altera, cancela ou vê um evento privado de terceiros. Evento
  sem sala é institucional. Evento cancelado não se
  altera, cria-se outro. Cancelar libera a sala.
- **Público-alvo** (seção 5): o evento interno só aparece para o público
  escolhido; com público, o evento não pode ser "público" (site), e na
  ocupação da sala aparece como "Reservado".

### Blog
- `blog:manage` cria, edita, publica, arquiva e exclui os posts da
  unidade dona que ele cobre. Quem tem só escopo escolhe a dona entre as
  suas unidades; post institucional (sem dona) é da gestão global.
- **Estados:** `draft` → `published` → `archived`, com volta a rascunho.
  Não se publica sem texto.
- Leitores veem só o publicado. Rascunho e arquivado ficam só para
  a gestão que cobre a dona. Slug único. Tipos: notícia ou comunicado.
- **Público-alvo** (seção 5): o post pode ser só para secretarias e/ou
  unidades; fora do público some da lista, do detalhe e da busca.

### Wiki
- Qualquer autenticado cria e edita. Excluir exige `wiki:manage` cobrindo a
  **unidade dona** da página (sem dona: só a gestão global).
- **Unidade dona:** vazia na criação, herda a da página-mãe. Marca uma
  unidade como dona quem está lotado nela ou tem `wiki:manage` cobrindo-a;
  trocar a dona exige poder marcar a antiga e a nova.
- **Público-alvo** (seção 5), com herança: a página é lida por quem
  satisfaz o público de **cada** página da cadeia até a raiz que tem um —
  a subpágina restringe mais, nunca abre o que a mãe fechou. Trocar o
  público de uma página: quem a criou ou a gestão da dona (qualquer um
  edita, mas não esconde dos colegas o que é de todos).
- **Controle de concorrência:** a edição informa a versão aberta. Se outra
  pessoa salvou antes, a edição é recusada (`409`) em vez de sobrescrever.
- Histórico de revisões com restauração. Árvore de páginas: não se exclui
  página com subpáginas, e uma página não fica dentro de si mesma.

### Catálogo de Serviços (Carta de Serviços)
- `catalog:manage` cria, publica e arquiva os serviços da unidade
  responsável que ele cobre (sem unidade: gestão global).
- **Publicar exige** resumo e ao menos um **canal de atendimento** (Lei
  13.460/2017).
- Publicado aparece no site sem login. Publicado **não é excluído**
  (arquive antes). A unidade responsável precisa existir.

### Contato (ouvidoria)
- Formulário público sem login.
- Exige **consentimento LGPD**. Há um campo-armadilha contra robôs e um
  limite de **5 mensagens por hora por IP**.
- Ler: `contact:read` (contém dados pessoais). Triar: `contact:manage`.
- **Setor:** toda mensagem chega na **caixa geral** (só a gestão global).
  A triagem encaminha a um setor (unidade ativa); `contact:*` com escopo
  vê e trata só o que chegou à área dele e pode devolver à caixa geral,
  mas não encaminhar para fora da área.
- Estados: `new`, `in_progress`, `answered`, `archived`. Pode haver um
  responsável, que precisa ser um usuário ativo.

### Diretório
- Pessoas e setores. Os dados de identidade (nome, e-mail, grupos) vêm do
  Keycloak/AD.
- Cada pessoa edita o próprio cargo, telefone, ramal e bio. A **lotação
  exibida** e o perfil de terceiros exigem `directory:manage` cobrindo a
  lotação exibida atual e a nova. Perfis ocultos aparecem só para essa
  gestão.
- **Setores públicos** no site listam só entidades, unidades e
  departamentos **ativos**.

### Busca Global
- Busca em Blog, Wiki, Arquivos, Catálogo, Diretório e Trâmite, nos
  módulos ativos, **respeitando as regras de cada um**: rascunho não
  aparece, a ACL das pastas vale, o sigilo do processo vale.
- Mínimo de 2 caracteres. Acha por **prefixo** ("vacina" acha
  "vacinação") e aceita aspas, `-termo` e `or`.

### Egress (webhooks de saída)
- `egress:manage` cadastra destinos:
  - tipos webhook, n8n, Zabbix, Grafana;
  - padrões de evento (ex.: `tramite.*`);
  - segredo HMAC opcional, guardado cifrado e nunca devolvido.
- **Anti-SSRF:** destinos em rede interna, loopback, endereço de metadados
  de nuvem ou nome de container são recusados. Em produção não dá para
  desligar a proteção.
- Entregas assíncronas, com **retentativa e espera crescente** até
  `EGRESS_MAX_ATTEMPTS` (8). Há reenvio manual e teste de conexão.

### Auditoria
- `audit:read` consulta (com filtros) e exporta (LAI: CSV, JSON, XML).
  `audit:verify` confere a cadeia de hash (só global). A própria
  exportação é auditada.
- **Com escopo**, `audit:read` vê as ações de quem estava **lotado na
  área quando agiu** (a lotação gravada no registro, protegida pelo hash).
  Ações sem lotação registrada (sistema, contas só globais) ficam com a
  gestão global.

### IAM (administração delegada)
- **Só a administração global:** entidades, perfis e mapeamentos do AD.
- **Estrutura:** com `iam:manage` numa unidade, cria e exclui as
  subunidades dela, edita a própria unidade e as de baixo (mudar de mãe
  exige cobrir o destino) e cuida dos departamentos. Numa entidade, também
  cria e exclui as unidades de 1º nível.
- **Lotações:** só com escopo dentro da área e com perfil cujas permissões
  o delegado tem naquele escopo. Ele vê só as lotações que cobre.
- **Contas:** `users:read` com escopo vê as contas com alguma lotação na
  área (manual ou por grupo do AD) e as sem lotação, para lotá-las.
  `users:manage` com escopo edita, redefine a senha e desbloqueia só
  conta **inteiramente** na área (todas as lotações cobertas) e sem o papel
  de administrador.

### LGPD
- **Aceite do termo:** anônimo (identificador do navegador) ou da conta,
  com versão e IP.
- **Direitos do titular:** "meus dados" (acesso e portabilidade, incluindo
  os dados pessoais dos módulos) e pedido de exclusão, processado com
  retentativas.

### Transparência
- Endpoints públicos, sem login: dados da plataforma, módulos ativos,
  conjuntos de dados e estatística de ações da auditoria.

### Atlas (procedimentos SEI, TTDD e assistente)
- **Fonte de consulta, não de execução:** o Atlas descreve como cada tipo
  de processo deve tramitar; **não bloqueia nem intercepta** o Trâmite.
- **Consulta pública** (API sem login, limite por IP): Tabela de
  Temporalidade **vigente** e procedimentos **ativos**. Desativado some da
  consulta e da Busca Global, mas continua na gestão. As telas do Atlas
  ficam na área autenticada.
- **Gestão (`atlas:manage`, concessão global):** cadastra, ativa,
  desativa e publica **nova versão** de procedimentos, e **atualiza a
  TTDD** (ver Vigência). Cada procedimento tem código
  (`ADM.LIC.001` — segmentos alfanuméricos com ponto, gravado em caixa
  alta) e versão; código + versão é único (repetir → 409).
- **Regras de cadastro:**
  - título, objetivo, público-alvo e classificação TTDD **existente**;
  - nível de acesso sugerido para o processo (`PUBLICO`, `RESTRITO`,
    `SIGILOSO`); restrito ou sigiloso **exige a hipótese legal**;
  - de 1 a 50 etapas com ordem única (≥ 1), sigla da unidade, setor,
    atribuições e prazo de 0 a 3650 dias; regra SEI
    `manter_aberto_apos_remessa`;
  - peças: `NATO_DIGITAL` ou `EXTERNO_DIGITALIZADO`; assinatura
    `INDIVIDUAL`, `CONJUNTA_MULTINIVEL` ou `EM_BLOCO`; o modelo da peça é
    um modelo **ativo** da biblioteca (ver Modelos de documento) — o link
    externo de minuta, se usado, só como endereço `http(s)` completo;
  - transições apontam para outra etapa existente; devolução em diligência
    exige a descrição da diligência.
- **TTDD (norma da CCPAD, ADR 017):** a tabela oficial (`docs/ttdd.pdf`)
  com 1.686 séries de 9 órgãos, organizada como no documento. Não se edita
  série a série: muda por **nova publicação** (ver Vigência):
  - **hierarquia:** órgão (`2.0`) > função (`2.0.01`) > subfunção
    (`2.0.01.00`) > série (`2.0.01.00.00`); a consulta filtra por qualquer
    prefixo;
  - **prazo de cada fase:** em anos **ou** por condição ("Enquanto estiver
    vigorando", "30 dias após a data do evento"); fase intermediária pode
    não existir ("não há"); fase corrente em branco no documento aparece
    como "não informado na TTDD";
  - **prazo total** só quando as duas fases são em anos (sem fase
    intermediária, conta só a corrente);
  - **destinação final:** guarda permanente, eliminação ou **não definida**
    (quando a TTDD marca "X");
  - observações da série, **recomendação da subfunção** (vale para todas
    as séries dela) e **fonte** — versão e edição/data do Diário Oficial;
  - cada procedimento é enquadrado numa série existente **e vigente**;
  - **exportação CSV** (`GET /atlas/ttdd/exportar`, pública, mesmos
    filtros `codigo` e `q` da consulta, só séries vigentes): separador
    `;` e BOM UTF-8 (abre direto no Excel), com órgão, função, subfunção,
    prazos, destinação, observações, recomendação e fonte; até 20.000
    séries. É também o modelo da planilha de atualização.
- **Vigência da TTDD (ADR 019):**
  - nova publicação, **pela tela** (Atlas → Tabela de Temporalidade →
    Atualizar TTDD, `atlas:manage` — ADR 022): enviar o CSV da exportação
    revisado no Excel (ou o `ttdd.json` do PDF), **simular** — séries
    novas, alteradas (antes → depois), revogadas e restabelecidas e os
    procedimentos afetados, sem gravar — e **aplicar** só o arquivo
    simulado (hash), depois de confirmar; auditado. Pelo servidor:
    `make ttdd-impacto` / `make ttdd-aplicar` (mesmas regras);
  - série que sai da tabela fica **revogada** (data e edição do Diário
    Oficial), nunca apagada: some da consulta, da árvore, da exportação e
    do assistente, mas abre pelo código com o aviso; só são revogadas
    séries dos órgãos presentes na carga;
  - toda mudança de série guarda os **valores anteriores** e a edição que
    a trouxe (`GET /atlas/ttdd/{codigo}/historico`, na página da série);
  - série revogada não aceita procedimento novo nem nova versão (422); o
    procedimento que já apontava para ela mostra o aviso.
- **Nova versão de procedimento (`atlas:manage`):** a partir da página do
  procedimento; o formulário vem preenchido com a versão atual (peças com
  formato e assinatura, transições). Ao publicar, a versão seguinte do
  mesmo código fica ativa e as demais são desativadas na mesma transação
  (eventos e auditoria com a versão que substituiu); a gestão vê todas as
  versões na página.
- **Modelos de documento (biblioteca, ADR 024):**
  - catálogo reutilizável: nome único (sem diferenciar maiúsculas, até
    150 caracteres), descrição e situação; as peças **apontam** para o
    modelo — publicar uma nova versão vale para todos os fluxos que o usam;
  - arquivo .docx, .odt, .pdf, .doc, .rtf, .xlsx ou .ods, até 10 MB,
    conferido pelo conteúdo (a assinatura dos bytes tem de corresponder à
    extensão); cada versão guarda nome, tamanho, SHA-256, nota (até 500
    caracteres), quem publicou e quando; as versões anteriores continuam
    baixáveis;
  - nada é apagado: **desativado**, o modelo sai da escolha de novas peças
    e da lista pública, mas as peças já ligadas continuam baixando e a nova
    versão de um procedimento que já o usava é aceita; ligar uma peça nova
    a modelo desativado ou inexistente → 422;
  - consulta e download públicos, sem a autoria; cadastro, versão e
    alteração com `atlas:manage`, auditados; o arquivo passa pela API
    (mesma origem), nunca por URL do MinIO;
  - no cadastro do procedimento, a peça com o mesmo nome de um modelo
    ativo (sem diferenciar acento e caixa) já vem ligada a ele;
  - no **detalhe de cada peça** (página do procedimento), a gestão liga,
    troca ou retira o modelo e envia o arquivo ali mesmo (nova versão do
    modelo ligado, que vale para todos os fluxos que o usam, ou um modelo
    novo com o nome da peça), **sem nova versão do procedimento** — o
    modelo não muda o fluxo; a troca é auditada com o modelo anterior;
  - na **página de cada série da TTDD**, a seção Modelos de documento lista
    os modelos daquele tipo de documento; a gestão liga um modelo ativo ou
    envia um arquivo (vira modelo novo já ligado); retirar só desfaz a
    ligação; série revogada não recebe modelos novos (422);
  - a peça **sem modelo próprio** mostra, no checklist e nos Detalhes, os
    modelos **ativos** da série da TTDD do procedimento; o modelo próprio
    da peça tem precedência.
- **Importação e exportação em lote (ADR 025, `atlas:manage`):**
  - o arquivo JSON da exportação (`procedimentos.json`) é o mesmo da
    importação; a peça aponta para o modelo pelo nome; até 500
    procedimentos; campo desconhecido é erro;
  - cada item é NOVO, NOVA_VERSAO (conteúdo diferente da versão em vigor),
    INALTERADO (mesmo conteúdo) ou ERRO (a regra que falhou); a simulação
    não grava; aplicar exige o arquivo simulado (hash) e nenhum erro;
  - procedimentos fora do arquivo não mudam (nada é desativado);
  - modelos em lote: o nome vem do arquivo; um código de série no início
    do nome liga o modelo à série.
- **Painel de cobertura (`atlas:manage`):** séries com fluxo e com modelo
  por secretaria, peças sem modelo (nem próprio nem da série) e modelos sem
  uso.
- **Busca Global:** procedimentos, séries vigentes da TTDD (código exato
  primeiro) e modelos ativos.
- **Ligação com o Trâmite (ADR 026):** o processo pode ser classificado
  com o procedimento do Atlas e a série da TTDD (na abertura ou depois,
  também encerrado; exige poder movimentar o processo; auditado); o
  processo mostra o checklist das peças (juntada = documento com tipo ou
  título igual ao nome da peça, sem acento e caixa; cancelado não conta),
  o modelo de cada peça e a guarda; a página Guarda documental lista os
  encerrados por série com a situação (fase corrente, intermediário,
  eliminação com aprovação da CCPAD, recolhimento ao permanente), contando
  do encerramento.
- **Atlas público (sem login):** `/procedimentos` e `/temporalidade` no
  site institucional, com os modelos para baixar e a planilha da TTDD; o
  proxy anônimo libera só a leitura do Atlas.
- **Visualizar modelo:** só PDF abre no navegador (`?inline=1`).
- **Validação (ADR 028):** o procedimento é RASCUNHO, EM_VALIDACAO ou
  HOMOLOGADO; só o homologado fica ativo (regra no banco). A importação
  como rascunho (padrão) não publica, não substitui a versão em vigor e não
  avisa; cada entrevista (data não futura, departamento, participantes pela
  função, registro, pendências) fica registrada pelo código e é imutável; a
  primeira leva o rascunho a em validação; homologar exige a série vigente,
  publica, substitui as demais versões e avisa; homologado não volta a
  rascunho (revisão = nova versão). Roteiro: `docs/atlas/PLANO_DE_VALIDACAO.md`.
- **Avisos de nova versão (ADR 027):** quem segue o fluxo (opt-in, pelo
  código) e quem está lotado (manual ou grupo do AD) nas unidades das etapas
  recebe o aviso da nova versão do procedimento; quem segue um procedimento
  que usa um modelo recebe o da nova versão do modelo; quem abriu um processo
  em andamento que segue a versão substituída/desativada é avisado pelo
  Trâmite. O autor da mudança não recebe; cada aviso chega uma vez; cada
  pessoa pode desligar os avisos de um módulo em `/notificacoes`; avisos
  lidos há mais de 180 dias são apagados.
- **Páginas (URL própria, compartilhável e imprimível):**
  - `/atlas` — busca unificada (procedimentos e séries em paralelo, ou a
    pergunta ao assistente), procedimentos e atalhos por secretaria;
  - `/atlas/procedimentos/{id}` — percurso em linha do tempo (prazo
    previsto = soma dos prazos das etapas), peças exigidas e temporalidade;
  - `/atlas/ttdd` — plano de classificação em árvore e séries, com os
    filtros na URL (`?codigo=`, `?q=`, `?page=`);
  - `/atlas/ttdd/{codigo}` — a série, a fonte oficial, os procedimentos
    que a produzem e o histórico de prazos (série revogada abre com aviso);
  - `/atlas/ttdd/atualizar` — atualização da TTDD (gestão);
  - `/atlas/importar` e `/atlas/cobertura` — importação em lote e painel
    de cobertura (gestão);
  - `/atlas/modelos` — biblioteca de modelos de documento (consulta,
    download e histórico; a gestão cadastra, publica versões e desativa);
  - os links antigos `/atlas?procedimento=` e `/atlas?ttdd=` redirecionam.
- **Calculadora de temporalidade** (nas páginas de série e de
  procedimento): é uma **estimativa** a partir da data informada — fim da
  fase corrente e da intermediária pela soma dos anos (29/02 vira 28/02);
  com fase corrente por condição, a data informada é a do fim da condição.
  Fase intermediária por condição ou fase corrente não informada **não têm
  data** (a tela explica o motivo). Não autoriza eliminação: ela continua
  dependendo da CCPAD.
- **Assistente (`atlas:read`, limite por pessoa — padrão 10 por minuto):**
  - **responde só sobre o acervo do Atlas (ADRs 021 e 023):** os **fluxos**
    homologados — do início ao fim: etapas na ordem, setor, prazo, o que
    fazer, peças exigidas (obrigatória, formato, assinatura, modelo),
    condição para seguir, devoluções em diligência, prazo total e a guarda
    dos documentos — e a **TTDD** (prazos de guarda, destinação final,
    classificação). Qualquer outro assunto recebe: *"Esse assunto foge do
    objetivo da IA: este assistente responde apenas sobre os fluxos
    documentais do Atlas … e a Tabela de Temporalidade…"*;
  - pergunta de fluxo responde com o procedimento; de temporalidade, com a
    série; que fala dos dois, com os dois;
  - a decisão é **determinística**, antes do modelo: pedido de tarefa ("me
    ajuda a escrever", "resuma", "traduza") que não fala de fluxo nem de
    temporalidade é recusado direto — salvo quando a palavra faz parte do
    nome da fonte encontrada ("Relatório Resumido…"); a resposta exige uma
    série com relevância ≥ **0,65** (fração dos termos da pergunta
    presentes no descritor — no início de uma palavra, aceitando plural e
    gênero: "licitação" não casa com "solicitação" —, sem acentos e sem
    palavras vazias, com bônus
    para o código exato e o descritor; palavras de temporalidade — "prazo",
    "guarda", "destinação", "por quanto tempo" — que a série não contém não
    entram na conta); usa até 3 séries, só as que ficam até 0,15 abaixo da
    melhor;
  - pergunta de fluxo ou de temporalidade sem fonte correspondente: *"Não
    localizei um fluxo homologado nem uma série da TTDD que corresponda à
    sua consulta…"*;
  - o modelo de IA recebe a mesma regra no prompt (inclusive ignorar
    pedidos para mudar de papel ou revelar as instruções); se ele devolver
    a recusa por assunto, a resposta é marcada como recusada e sai sem
    fontes;
  - repete os prazos exatamente como na TTDD (inclusive condições), a
    destinação, a recomendação e a fonte;
  - com IA configurada: o modelo redige a resposta só com as fontes
    encontradas (temperatura 0,05); sem IA, ou se ela falhar, a resposta é
    a **síntese canônica** dos fluxos e das séries;
  - **qual IA (ADR 020):** Configurações → Inteligência artificial
    (`ia:manage`) — conexão principal e reserva (IA local ou fornecedor
    compatível com a API da OpenAI); a chave é cifrada e nunca volta pela
    API; toda conexão é testada antes de ser salva; trocar o endereço exige
    a chave de novo; fornecedor externo exige autorização registrada e, por
    padrão, CPF, CNPJ, e-mail e telefone são mascarados na pergunta. Sem
    nada salvo na tela, valem as variáveis `ATLAS_AI_*`;
  - a resposta informa o modo (`ia`, `sintese`, `recusada`), a relevância
    e as fontes usadas — fluxos e séries, com link para a página de cada uma;
  - fica numa gaveta lateral disponível em todas as páginas do Atlas; o
    "Perguntar sobre…" de cada página só preenche a pergunta (nada é
    enviado sem a pessoa confirmar);
  - cada consulta é auditada **sem o texto da pergunta** (modo,
    relevância, fontes e tamanho), por minimização (LGPD).
- **Outbox e auditoria:** `atlas.workflow.created`, `.activated` e
  `.deactivated` (também quando uma nova versão substitui a anterior), na
  mesma transação da mudança; repetir o estado atual não gera evento.
  Auditados também: `atlas.ttdd.carga.aplicada` (totais e hash do
  arquivo), `atlas.assistente.consulta` (sem o texto) e as alterações de
  IA (`ia.conexao.*`, `ia.uso.alterado` — nunca a chave).


---

## 8. Pontos de atenção (comportamento atual)

Comportamentos do código hoje que merecem decisão de quem administra:

1. **Permissões de plataforma exigem concessão global** (seção 5): um
   `monitoring:read` ou `modules:manage` dado numa unidade não vale.
2. **Processo sigiloso não é visto nem por `tramite:manage`.** É
   intencional (só autor e credenciados), mas quem administra precisa
   saber.
3. **Reabrir processo exige `tramite:manage`** (na unidade atual),
   que nenhum perfil de sistema tem além do administrador. O Protocolo conclui e arquiva, mas
   não reabre.
4. **Sem isolamento entre entidades** (seção 6): uma implantação serve a
   uma organização (que pode ter várias entidades), não a várias
   organizações independentes.

Corrigidos (antes eram pontos de atenção): perfis de gestão com escopo
valiam para a plataforma toda (ADR 013); estrutura desativada seguia
dando acesso e recebendo processos, e excluir uma unidade-mãe deixava as
subunidades sem mãe em silêncio — ver a seção 2.4.
