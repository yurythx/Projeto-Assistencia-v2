# 030 — SEMPRAS: atendimento socioassistencial por unidade

- **Status:** Aceito e implementado (fases 0–7: o pedido do titular está no §8; e2e/e-MAG/RIPD seguem no [ROADMAP_SEMPRAS.md](../ROADMAP_SEMPRAS.md))
- **Data:** 2026-10-08
- **Relacionado:** [ADR 008](008-kernel-grafo-de-modulos-e-estrategia-de-testes.md) (Kernel e testes), [ADR 012](012-estrutura-desativada-nao-concede.md) (estrutura desativada), [ADR 013](013-permissao-com-escopo-e-heranca.md) (escopo e herança), [ADR 016](016-camada-de-aplicacao-e-transacoes.md) (transações), [ADR 027](027-notificacoes.md) (tempo real depois do commit)

## Contexto

O Projeto-Assistencia-v2 nasce da plataforma Nexus para atender a
Secretaria Municipal de Promoção e Assistência Social (**SEMPRAS**) de
Rondonópolis. As 16 unidades de atendimento (Casa Abrigo, Casa da Mulher,
Centro POP, CEU, os dois Conselhos Tutelares, os dez CRAS e o CREAS) têm a
mesma jornada:

1. buscar o munícipe;
2. cadastrá-lo, se ele ainda não existir;
3. incluí-lo na fila da unidade;
4. chamá-lo para o atendimento técnico ou psicológico.

Cada servidor só pode ver a própria unidade. O Superintendente e o
Administrador veem todas.

Os dados são **sensíveis** (LGPD art. 11): vulnerabilidade social,
violência doméstica, crianças e adolescentes. As anotações do psicólogo
têm sigilo profissional próprio (Código de Ética do CFP; Resolução CFP
01/2009).

## Decisão

### 1. Um plug-in; as unidades são dados

- Plug-in `sempras` (`internal/modules/sempras`), registrado no Kernel como
  os demais. Começa **desativado** (`DefaultEnabled: false`) e é ligado em
  Configurações → Módulos. Não depende de outro plug-in.
- As 16 unidades **não** são módulos. São as Unidades da entidade SEMPRAS
  que a estrutura real já traz (`deploy/estrutura`, `make
  estrutura-aplicar`). Todo registro do módulo é posicionado numa unidade
  (`unidade_id`). A autorização é a do ADR 013:
  `auth.Can(identidade, permissão, alvo)`.
- O alvo leva **entidade e unidade** (`auth.Target{EntidadeID, UnidadeID}`).
  Assim a concessão na entidade (o Superintendente) cobre a unidade mesmo
  onde a árvore expandida não viaja, como no ticket do WebSocket, cujo
  `Scope.Unidades` não é serializado.

### 2. Permissões (todas com escopo)

| Permissão | Para quem | O que libera |
|---|---|---|
| `sempras:recepcao` | Recepcionista | Buscar e cadastrar munícipe, incluir na fila, ver a fila, cancelar quem ainda aguarda |
| `sempras:atendimento` | Técnico | Além da recepção: chamar da fila, atender, concluir, marcar ausência, ver a ficha (prontuário social) e registrar evolução |
| `sempras:psicologia` | Psicólogo | Atendimentos do tipo psicológico e registros psicológicos (sigilosos) |
| `sempras:gestao` | Superintendente / coordenação | Fila e indicadores das unidades cobertas; cancelar entradas |

As quatro entram em `auth.ScopedPermissions` (e no relatório
`make iam-scope-report`) e valem **onde foram concedidas**.

Perfis semeados pela migração do módulo (editáveis; `sistema = false`):

| Perfil (slug) | Permissões | Concedido em |
|---|---|---|
| `sempras-recepcionista` | recepcao | unidade |
| `sempras-tecnico` | recepcao, atendimento | unidade |
| `sempras-psicologo` | recepcao, atendimento, psicologia | unidade |
| `sempras-superintendente` | recepcao, atendimento, gestao | **entidade SEMPRAS** |

O Superintendente recebe a concessão **na entidade SEMPRAS**, e não global:
vê as 16 unidades, mas não ganha nada nas outras secretarias. O
administrador da plataforma (`nexus-admin`) continua passando em
`auth.Can`, exceto no sigilo psicológico (item 5).

### 3. Vínculo pelo Active Directory

No AD, as unidades são **Unidades Organizacionais**, e o Keycloak entrega
**grupos**. Por isso a TI cria um grupo de segurança por unidade e perfil.
Cada grupo vira um Mapeamento AD (perfil + escopo) sem código novo:

```
SEMPRAS_<UNIDADE>_<PERFIL>     ex.: SEMPRAS_CRAS_CENTRAL_TECNICO
SEMPRAS_SUPERINTENDENCIA       (perfil superintendente na entidade)
```

São 16 × 3 + 1 = **49 grupos**. A lista completa e o SQL idempotente dos
mapeamentos ficam em `deploy/sempras/`. Até os grupos existirem, as
lotações podem ser cadastradas na tela de Usuários: mesmo efeito, sem AD.

### 4. Munícipe, fila e atendimento

- **Cadastro do munícipe compartilhado** na SEMPRAS: uma pessoa, um
  cadastro, o que evita duplicidade entre CRAS. É identificado por **CPF**
  (dígitos verificadores conferidos), **NIS** ou, na falta dos dois, **nome +
  data de nascimento + nome da mãe**, porque a população de rua (Centro
  POP) e as crianças (Conselho Tutelar) muitas vezes não têm documento.
  CPF e NIS são únicos no banco.
- **Busca:** por CPF ou NIS (exata) ou por nome (prefixo, sem acento). Devolve
  identificação mínima (CPF e NIS mascarados na busca por nome) e o
  **histórico só da área de quem busca**, com o número de atendimentos nas
  unidades cobertas. Nunca mostra o que aconteceu em outra unidade.
- **Fila por unidade e por dia:**
  - a senha é sequencial por unidade e por dia, no fuso do processo
    (`TZ=America/Cuiaba` na imagem);
  - há prioridade legal (Lei 10.048/2000: idoso, gestante, pessoa com
    deficiência etc.);
  - a ordem é prioridade e depois chegada;
  - o munícipe tem no máximo uma entrada ativa por unidade.
- **Estados:** `aguardando → chamado → em_atendimento → concluido`. Saídas
  laterais: `aguardando|chamado → cancelado` e `chamado →
  nao_compareceu`. A transição é condicional no banco (`UPDATE … WHERE
  status = …`). Dois técnicos chamando ao mesmo tempo nunca pegam a mesma
  pessoa: "chamar o próximo" usa `FOR UPDATE SKIP LOCKED`.
- **Tipo do atendimento:** `social` (técnico) ou `psicologico`. Só quem
  tem `sempras:psicologia` chama e atende o psicológico.
- **Prontuário = registros de evolução**, sempre ligados a uma unidade:
  - **somente inclusão:** gatilhos bloqueiam UPDATE e DELETE, e a correção
    é feita num novo registro;
  - é um documento com valor legal.

### 5. Sigilo psicológico

Os registros do tipo `psicologico` só aparecem para quem tem
`sempras:psicologia` **por concessão** cobrindo a unidade. Aqui o papel
`nexus-admin` **não** passa automaticamente: administrar a plataforma não
dá acesso a sigilo profissional. Os demais veem apenas que existe um
registro sigiloso (data e autor), sem o texto.

### 6. Auditoria (LGPD e rastreabilidade)

Grava na **mesma transação** (ADR 016):

| Ação | Quando |
|---|---|
| `sempras.cidadao.criado`, `sempras.cidadao.atualizado` | cadastro, com antes e depois |
| `sempras.fila.incluido`, `.chamado`, `.iniciado`, `.concluido`, `.nao_compareceu`, `.cancelado` | cada mudança de estado |
| `sempras.registro.criado` | evolução registrada (sem o texto na auditoria) |
| `sempras.prontuario.visualizado` | **toda** abertura da ficha (leitura) |

A auditoria de leitura é um padrão novo na plataforma. Ela vale **só para
o prontuário**, que é dado sensível. Buscas e a fila não são auditadas
como leitura, para não afogar a trilha.

### 7. Tempo real

- O tópico `sempras:fila:<unidade_id>` é autorizado pelo `WSAuthorizer` do
  módulo: `auth.Can` com qualquer permissão do SEMPRAS na unidade.
- Depois do commit, o serviço publica no Hub um frame **sem dado pessoal**
  (`{atendimento_id, status, senha}`). A tela recarrega a fila pela API,
  que reaplica a autorização.
- É o mesmo padrão do Mercúrio e das notificações (ADR 027): entrega de
  melhor esforço, e a verdade está no banco.
- O evento de domínio vai ao outbox **só com IDs e estado**. O Egress pode
  repassá-lo a webhooks, então nada sensível sai por ali.
- Desligar o módulo derruba as assinaturas (Kernel).

### 8. Direitos do titular

O `lgpd.PersonalData` da plataforma é indexado por **conta de usuário**, e
o munícipe não tem conta. O SEMPRAS **não** o implementa. O pedido do
munícipe (acesso, correção, anonimização) é registrado por quem o atende
presencialmente, numa tabela própria (`sempras_pedidos_titular`, migração
000144) — não pelo fluxo de `/api/v1/lgpd/*`.

- **Acesso** e **correção** se resolvem na hora: quem atende exporta o
  prontuário (mesma área e sigilo psicológico de `Prontuario`) ou edita o
  cadastro (`PUT /sempras/cidadaos/{id}` — os 4 perfis podem editar, não
  só a recepção) com o titular presente. O pedido nasce já `concluido`.
- **Anonimização** nasce `pendente` e só o Superintendente decide — com
  **cobertura total** (`sempras:gestao` sem recorte de unidade), nunca
  parcial: o cadastro do titular é intersecretaria, a decisão também
  precisa ser. Aprovar zera nome, nome social, CPF, NIS, nome da mãe,
  telefone e endereço (nunca `DELETE`); `data_nascimento` fica, porque a
  CHECK de identificação da tabela (`cpf IS NOT NULL OR nis IS NOT NULL
  OR (data_nascimento IS NOT NULL AND nome_mae <> '')`) não permite zerar
  os três ao mesmo tempo sem quebrar a integridade do próprio cadastro.
  `sempras_atendimentos`/`sempras_registros` não mudam: são o registro do
  atendimento prestado, não "dado do titular" neste sentido — mesma
  distinção que a plataforma já faz entre a conta do usuário e
  `audit_logs`/tramitações. A decisão é definitiva (trigger de
  imutabilidade): nem a gestão desfaz uma anonimização aprovada.

### 9. Paridade com o `protocolo_digital` (Fase 8.1)

Uma comparação file-a-file entre o legado de referência e o módulo achou,
além da diferença esperada de navegação (abas/modal do legado → rotas e
cards aqui, já decidida nas seções 1 e 4), dois módulos sem nenhuma tela
equivalente:

- **Ofícios do CRAS** (`sempras_cras_oficios`, migração 000158): o legado
  tem esse controle só pro CRAS; o módulo já tinha o equivalente pro
  CREAS (`sempras_creas_notificacoes`, Fase 8). É uma tabela própria, não uma
  extensão da notificação do CREAS — o legado guarda campos que a
  notificação não tem (nº do ofício, data de emissão, nº da resposta),
  mesmo padrão já usado em Casa Abrigo (uma tabela por conceito, não uma
  genérica).
- **Atendimentos Coletivos / SCFV** (`sempras_atendimentos_coletivos`,
  migração 000159): evento de grupo da unidade, não é por munícipe. O
  legado guarda a categoria como texto livre e soma por `ILIKE` no
  relatório — um erro de digitação some do número sem avisar. Aqui a
  categoria é um `CHECK` fechado (as 8 categorias do Bloco III do
  RMA/MDS), mesma filosofia já aplicada a `PaifVinculo`/`Beneficio`
  (campo real, não texto casado por substring). Alimenta 8 novos campos
  de `RmaUnidade` e a nova página `/sempras/coletivos`.

Também apareceram campos do legado sem equivalente em formulários já
existentes — adicionados sem mudar a tabela-conceito: `forma_acesso` e os
tipos `aluguel_social`/`outro` em `Beneficio` (migração 000156),
`tipo_atendimento` (particularizado/coletivo/ambos) em `PaifVinculo`
(migração 000157), e o campo `prazo_resposta` de `NotificacaoCreas` — que
já existia de ponta a ponta no backend desde a Fase 8, só faltava no
formulário do frontend.

Ficou fora desta fase: o **Centro POP** — do tamanho de toda a Fase 8
somada, tratado na seção 10 a seguir.

### 10. Centro POP (Fase 9)

Revendo a especificação do legado (`SocialAssistance.js`, 8 "entidades":
pessoa, recepção, abordagem de rua, benefício, documento, observação,
encaminhamento, rede de apoio) contra o módulo que já existe aqui: a
maior parte **já tem equivalente real e genérico**, porque Centro POP é
só mais uma `unidade` do SEMPRAS (já cadastrada em `deploy/estrutura`) —
fila (`sempras_atendimentos`), evolução/diário de bordo
(`sempras_registros`), rede de apoio (`sempras_rede_apoio`) e benefícios
(`sempras_beneficios`, só com 2 tipos a mais — auxílio-gás,
cobertor/agasalho) funcionam sem nenhuma mudança. O trabalho novo:

- **Cadastro sem identificação confiável** (`sempras_cidadaos.anonimo`,
  migração 000160): a tabela já previa gente sem CPF/NIS, identificada
  por nascimento + nome da mãe (migração 000143) — mas abordagem de rua
  às vezes não tem nem isso. Em vez de uma tabela de pessoa paralela (o
  que quebraria o reaproveitamento de fila/prontuário/benefícios), a
  identificação ganha uma 4ª via, explícita e auditável: `anonimo
  BOOLEAN`, aceito pelo `CHECK` de identificação só quando marcado. Vale
  pra qualquer unidade, não só Centro POP, mas na prática só a abordagem
  de rua usa.
- **Perfil de rua** (`sempras_centro_pop_perfis`, migração 000161):
  extensão 1:1 de `sempras_cidadaos`, não campo solto nela — situação
  habitacional, tempo de rua, motivo da vulnerabilidade, saúde/
  medicação/drogas. Sigiloso como o registro psicológico (mesma RN já
  estabelecida no módulo): só quem abre o prontuário com
  `sempras:atendimento` vê, nunca a recepção (que, de resto, já não
  acessa `Prontuario` — a permissão em si já é o gate, sem precisar de
  mascaramento de campo dentro da mesma resposta).
- **Encaminhamento externo** (`sempras_encaminhamentos_externos`,
  migração 000162): `Encaminhamento` hoje é entre unidades nossas, com
  aceite/recusa pela unidade de destino. Um encaminhamento pra CAPS AD,
  albergue ou a Polícia Civil não tem ninguém do nosso lado pra decidir
  — forçar isso dentro de `Encaminhamento` (destino nullable, texto
  alternativo) distorceria o fluxo de 2 pontas que CRAS/CREAS já usam.
  Ficou uma entidade nova, sem fluxo de aceite — status gerenciado por
  quem registrou, mesma lógica de log dos Atendimentos Coletivos (Fase
  8.1), não um fluxo de decisão.
- **Documentos** (`sempras_centro_pop_documentos`, migração 000163):
  mesmo padrão dos documentos da Casa Abrigo (ticket de upload direto ao
  MinIO, confirmação, download pré-assinado), só que chaveado por
  cidadão, não por acolhimento — por isso guarda `unidade_id` próprio
  (`Cidadao` não tem unidade: é cadastro da secretaria toda), pro escopo
  de área nas operações de confirmar/baixar/remover.

Ficou fora, documentado, não construído: **abordagem de rua**
(`SocialAbordagem`) — o próprio legado não tem tela de criar/editar
isso, só aparece em relatórios agregados por um caminho que a extração
não encontrou; e um **painel de indicadores dedicado** — `/sempras/
painel` e o RMA já cobrem o essencial por unidade/competência.

## Consequências

- **Reuso:** Kernel, `auth.Can` e `CoverageOf`, auditoria encadeada,
  outbox, Hub, idempotência, `ScopePicker` e `ModuleGate`. O módulo não
  cria utilitário de plataforma.
- **Cobertura:** o módulo entra no gate de 100% por pacote, com matriz de
  falhas do repositório, testes HTTP "unidade A não vê B; Superintendente
  vê todas" e o teste geral de módulo desligado (404).
- **Dependência externa:** os 49 grupos do AD. Sem eles, as lotações são
  manuais.
- **Fora do escopo agora:** painel de chamada (TV), encaminhamento entre
  unidades (CRAS → CREAS), RMA/MDS e integração com o CadÚnico. Todos
  estão no roadmap.
