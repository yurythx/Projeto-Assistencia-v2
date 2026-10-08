# 030 — SEMPRAS: atendimento socioassistencial por unidade

- **Status:** Aceito e implementado (fases 0–6; a 7 — titular e e2e — está no [ROADMAP_SEMPRAS.md](../ROADMAP_SEMPRAS.md))
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
munícipe (acesso, correção, portabilidade) é atendido pela gestão em fluxo
próprio (fase 7 do roadmap). A eliminação dá lugar à **anonimização** quando
há obrigação legal de guarda.

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
  unidades (CRAS → CREAS), RMA/MDS, integração com o CadÚnico e direitos
  do titular do munícipe. Todos estão no roadmap.
