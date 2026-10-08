# 013 — Permissão com escopo e herança

- **Status:** Aceito e implementado (fases 1, 2 e 3)
- **Data:** 2026-09-27
- **Escopo:** `internal/platform/auth`, `internal/platform/iam`, os módulos
  com permissões de gestão, o frontend (hooks de permissão e formulários de
  dono)

## Contexto

Hoje a identidade carrega as permissões **somadas** de todos os perfis e,
separada, a lista de escopos. `auth.HasPermission(perm)` ignora o escopo.
Por isso só o Trâmite, os Arquivos e o Mercúrio restringem por lotação
([REGRAS_DE_NEGOCIO.md §5](../REGRAS_DE_NEGOCIO.md#5-onde-o-escopo-vale-e-onde-não-vale)).
Lotar alguém como gestor de conteúdo "na UPA" libera Blog e Catálogo para a
prefeitura inteira.

As plataformas de referência avaliam acesso como **(quem, ação,
recurso)**, com o recurso posicionado na organização e a concessão valendo
do escopo para baixo: Google Cloud, Azure, SAP e Salesforce.

## Proposta

### 1. Concessões em vez de soma

A identidade passa a carregar **concessões**: cada uma tem perfil,
permissões e escopo. O resolvedor já lê isso linha a linha e hoje descarta
a associação. `Permissions`, a soma, continua existindo só para exibição e
menus.

### 2. Cobertura (herança para baixo)

| Concessão | Cobre recursos… |
|---|---|
| sem escopo (global) | de toda a plataforma, inclusive os institucionais (sem dono) |
| entidade E | de E e de todas as unidades e departamentos de E |
| unidade U | de U, das **subunidades** de U (árvore `parent_id`) e dos departamentos delas |
| departamento D | de D |

A árvore de subunidades é expandida no resolvedor e fica no cache junto
com a identidade, então checar não custa consulta.

### 3. API de autorização

- `auth.Can(identidade, perm, alvo)`: o alvo é a posição do recurso
  (entidade, unidade, departamento). Vale se alguma concessão com `perm`
  cobre o alvo.
- `auth.CanGlobal(identidade, perm)`: só concessão sem escopo. Vale para
  recursos institucionais e para as permissões de plataforma.
- `auth.HasPermission` passa a ser "tem em algum escopo". Serve para
  mostrar o menu e para criar recursos no próprio escopo, **nunca** para
  decidir sobre um recurso existente.

### 4. Duas classes de permissão

**Plataforma (só valem com concessão global):** `modules:manage`,
`keycloak:manage`, `branding:manage`, `monitoring:*`, `egress:manage`,
`audit:verify`, `signum:manage` e `example:manage`. Na fase 3, `iam:manage`,
`users:*`, `audit:read`, `contact:*` e `files:manage` passaram a valer com
escopo (ver "Como ficou na implementação (fase 3)").

**Com escopo (valem onde foram concedidas):**

| Permissão | Posição do recurso | O que muda no banco |
|---|---|---|
| `tramite:manage` | unidade de origem ou atual do processo | nada |
| `catalog:manage` | unidade responsável do serviço | nada (coluna existe) |
| `blog:manage` | dono do post | nova coluna de dono |
| `wiki:manage` | dono da página, herdado da página-mãe | nova coluna de dono |
| `calendar:manage` | dono da sala (moderação dos eventos nela) | nova coluna de dono |
| `mercurio:manage` | departamento da sala | nada |
| `directory:manage` | lotação exibida da pessoa | nada |
| `contact:read`, `contact:manage` | setor para onde a triagem encaminhou | nova coluna (fase 3) |
| `audit:read` | lotação do autor gravada no registro | nada (fase 3) |
| `files:manage` | unidade dona da pasta ou de uma acima | nova coluna (fase 3) |
| `iam:manage`, `users:*` | estrutura, lotação e contas da área | nada (fase 3) |

**Dono na criação:**
- quem tem concessão global escolhe qualquer dono, ou nenhum, o que torna
  o recurso **institucional**;
- quem tem concessão com escopo só escolhe dentro dele, e o padrão é a
  própria unidade.

**Leitura não muda:** Blog, Wiki, Catálogo e Agenda continuam visíveis
como hoje. Segmentar a leitura por público-alvo é outra funcionalidade —
depois implementada no [ADR 014](014-publico-alvo.md).

### 5. Compatibilidade

- **Conteúdo existente** fica **institucional** (sem dono): só quem tem
  concessão global o gerencia.
- **Concessões com escopo que hoje "vazam"** (gestão ou plataforma
  concedidas com escopo) passam a valer só no escopo, ou a não valer, no
  caso das de plataforma. Antes de publicar, um **relatório de impacto**
  (`make iam-scope-report`) lista cada lotação e mapeamento afetado,
  indicando quem perde o quê.
- **Ambiente de teste:** os grupos `NEXUS-*` são globais e não mudam. O
  Protocolo já é restrito por unidade.

## Fases

1. **Núcleo:** concessões na identidade (resolvedor, cache, `/me`), `Can`
   e `CanGlobal`, expansão das subunidades, permissões de plataforma só
   globais, relatório de impacto e testes da matriz de cobertura.
2. **Módulos:**
   - Trâmite (`tramite:manage` por unidade, reunindo o `InUnidade` atual
     na mesma regra);
   - Catálogo, Blog, Wiki, Agenda, Mercúrio e Diretório, com a coluna de
     dono, os formulários e testes HTTP de "gestor da unidade A não
     gerencia B, o global gerencia tudo".
3. **Depois:**
   - administração delegada do IAM (gestor que cadastra usuários e
     lotações só da sua entidade);
   - auditoria filtrada por escopo;
   - contato encaminhado por setor;
   - Arquivos com dono organizacional.

## Como ficou na implementação (fase 2)

- **Wiki:** criar e editar continuam abertos a qualquer autenticado. Marca
  uma unidade como dona quem está **lotado** nela (`auth.LotadoEm`:
  concessão de qualquer perfil na unidade, num departamento dela ou numa
  unidade acima) ou tem `wiki:manage` cobrindo-a. Vazio na criação herda
  a dona da página-mãe. Trocar a dona exige poder marcar a antiga e a nova.
- **Agenda:** a dona é da **sala**; o evento é moderado pela gestão da sala
  reservada. Evento sem sala é institucional; o organizador continua
  gerindo os próprios. Salas inativas aparecem só para a gestão que as
  cobre.
- **Blog:** o post exige dona para quem tem só escopo; a gestão global pode
  deixá-lo institucional. Rascunhos e arquivados aparecem só para a gestão
  que cobre a dona.
- **Mercúrio e Diretório:** sem coluna nova — valem o departamento da sala
  e a lotação exibida da pessoa (a atual e a nova, ao editar).
- Fora do escopo, a API responde **403**; na leitura de algo que a pessoa
  não deve ver, **404**, como antes.

## Como ficou na implementação (fase 3)

- **Contato:** a mensagem chega na caixa geral (institucional, só a
  gestão global); a triagem encaminha a um setor (unidade ativa).
  `contact:*` com escopo vê e trata só o que foi encaminhado à área e
  pode devolver à caixa geral. Migração 129.
- **Auditoria:** a área de um registro é a lotação do autor **gravada no
  próprio registro** (`entity_context.scopes`), que entra na cadeia de
  hash — sem coluna nova, sem reescrever o passado quando a pessoa muda de
  lotação, e valendo para os registros antigos que já traziam o campo.
  Vale na consulta, no detalhe e na exportação LAI. `audit:verify` segue
  global.
- **Arquivos:** a pasta pode ter unidade dona, herdada pela cadeia até a
  raiz; `files:manage` com escopo tem acesso total às pastas da área.
  Marca a dona quem é lotado nela ou a gerencia. Dono pessoal e ACLs não
  mudam. Migração 130.
- **IAM delegado:** entidades, perfis e mapeamentos do AD são da
  administração global. A delegada cria e exclui unidades onde cobre a
  posição (a mãe, ou a entidade no 1º nível), edita as que cobre e os
  departamentos delas; lota só dentro da área e com perfil cujas
  permissões ela tem **naquele escopo**; vê as contas da área e as sem
  lotação; administra (senha, desbloqueio, edição) só conta inteiramente na
  área e sem o papel de administrador.
- O relatório de impacto (`make iam-scope-report`) acompanha a lista, e
  um teste garante que ela espelhe `auth.ScopedPermissions`.

## Decisões tomadas (2026-09-27)

1. Conteúdo existente sem dono é **institucional**: só a gestão global o
   gerencia.
2. Concessão na unidade **cobre as subunidades**.
3. Administração delegada do IAM fica na **fase 3** — implementada
   com "lotações + estrutura"; auditoria pelas ações de quem é da área;
   contato encaminhado pela triagem; pasta da unidade nos Arquivos.
4. A **leitura continua aberta**. O escopo restringe só a gestão.
