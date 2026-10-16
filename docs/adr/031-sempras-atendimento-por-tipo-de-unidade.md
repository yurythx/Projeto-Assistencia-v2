# 031 — SEMPRAS: cada tipo de unidade com o seu próprio atendimento

- **Status:** Aceito e implementado (fases A a D); a fase E (RMA do CREAS e do Centro POP) e os serviços próprios da Casa da Mulher ficam para depois
- **Data:** 2026-10-16
- **Revisa:** [ADR 030](030-sempras-atendimento-socioassistencial.md) §1 (a premissa de que "as 16 unidades têm a mesma jornada")
- **Referência:** `protocolo_digital/docs/RELATORIO_TECNICO_MODULOS_ASSISTENCIA_SOCIAL.md` e `ARQUITETURA_SEGURANCA_SUAS_RBAC.md` (legado)

## Contexto

O ADR 030 tratou as unidades da SEMPRAS como dados de um mesmo fluxo:
buscar o munícipe, pôr na fila e chamar para o atendimento. A separação
entre unidades ficou só no **escopo** (cada servidor vê a própria). Isso
funciona para o isolamento de dados, mas não para o atendimento. Pela
Tipificação Nacional (Resolução CNAS 109/2009), cada tipo de unidade
oferta serviços diferentes, a públicos diferentes e com regras de
identificação e sigilo diferentes. O legado (`protocolo_digital`) já
reflete isso: há um módulo de tela por tipo (`CrasAttendanceManagement`,
`CreasAttendanceManagement`, `SocialAssistance` para o Centro POP,
`CasaAbrigoManagement`, `ConselhoTutelarManagement`), e o middleware
`socialRbac.js` checa se a lotação do servidor coincide com o módulo
(camada 3).

A revisão do código atual encontrou o seguinte:

| # | Problema | Onde |
|---|---|---|
| 1 | **Não existe tipo de unidade.** O frontend adivinha o Centro POP por regex no nome (`/centro\s*pop/i`), e todo o resto cai como CRAS | `FichaModal.tsx:93`, `cidadaos/[id]/page.tsx:97` |
| 2 | **O backend aceita qualquer serviço em qualquer unidade.** PAEFI, MSE e notificação (CREAS) podem ser abertos num CRAS; acolhimento institucional num CREAS; medida do Conselho Tutelar num CRAS; PAIF no Centro POP. A única checagem é a permissão na unidade | `application/service.go` (`IncluirPaif`, `RegistrarPaefiCaso`, `RegistrarMseMedida`, `RegistrarConselhoAtendimento`, `RegistrarAcolhimento`...) |
| 3 | **Uma tela só para todas.** `/sempras` tem um seletor de unidade e a mesma recepção, fila e ficha para qualquer uma. A ficha (`cidadaos/[id]`, 1.594 linhas) carrega as seções de todos os tipos | `sempras/page.tsx`, `cidadaos/[id]/page.tsx` |
| 4 | **A recepção não registra a demanda.** A entrada na fila guarda só tipo (social/psicológico) e prioridade. No legado, cada unidade tem a sua lista (CRAS: CadÚnico, benefício eventual, PAIF; Centro POP: alimentação/higiene, banho/lavanderia, guarda de pertences, passagem) | `EntradaInput` |
| 5 | **O painel de chamada da TV exibe e fala o nome do munícipe em qualquer unidade**, inclusive na Casa da Mulher (Lei 11.340/2006, endereço e identidade sob sigilo), na Casa Abrigo (crianças acolhidas, ECA art. 17 e 143) e no CREAS (violação de direitos). É risco de exposição, não só de usabilidade | `PainelChamada.tsx:52,182` |
| 6 | **O menu é o mesmo para todos.** "Atendimentos coletivos" aparece para quem tem `sempras:atendimento`, inclusive no Conselho Tutelar e na Casa Abrigo, e o RMA só cobre o formulário do CRAS (o MDS tem um RMA para o CREAS e outro para o Centro POP) | `Sidebar.tsx:32-43` |
| 7 | **A estrutura tem unidades fora do atendimento.** Sede da SEMPRAS, SCFV Padre Lothar e Vila Olímpica não estão no ADR 030 e não têm regra | `deploy/estrutura` |

## Decisão

### 1. O tipo da unidade é um dado do módulo

Nova tabela `sempras_unidades` com uma linha por unidade do IAM:
`unidade_id` (PK, FK), `tipo` (enum) e opções de comportamento. O IAM
continua sem saber nada de assistência social. A migração preenche a
tabela pelo slug da estrutura real (igual ao `deploy/sempras/mapeamentos-ad.sql`).
Uma unidade sem linha **não atende**: a fila e os serviços respondem 404,
como um módulo desligado.

| Tipo | Unidades | Proteção (CNAS 109) |
|---|---|---|
| `cras` | os 9 CRAS | Básica |
| `scfv` | SCFV Padre Lothar, CEU, Vila Olímpica | Básica |
| `creas` | CREAS | Especial de média complexidade |
| `centro_pop` | Centro POP | Especial de média complexidade |
| `casa_mulher` | Casa da Mulher | Especial (violência contra a mulher) |
| `casa_abrigo` | Casa Abrigo | Especial de alta complexidade |
| `conselho_tutelar` | os 2 Conselhos | Garantia de direitos (ECA) |
| — | Sede da SEMPRAS | gestão; não atende |

### 2. Um catálogo de serviços por tipo, checado no backend

No domínio, um mapa `tipo → serviços ofertados`:

| Serviço | cras | scfv | creas | centro_pop | casa_mulher | casa_abrigo | conselho |
|---|:-:|:-:|:-:|:-:|:-:|:-:|:-:|
| Fila e recepção | ✓ | ✓ | ✓ | ✓ | ✓ | | ✓ |
| Evolução social/psicológica, rede de apoio | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Benefício eventual | ✓ | | | ✓ | | | |
| PAIF, ofícios | ✓ | | | | | | |
| Atendimento coletivo | ✓ | ✓ | | | | | |
| PAEFI, MSE, notificação externa | | | ✓ | | | | |
| Perfil de rua, documentos, encaminhamento externo, cadastro anônimo | | | | ✓ | | | |
| Acolhimento institucional (PIA, termos, visitantes) | | | | | | ✓ | |
| Medida protetiva (art. 101 ECA) | | | | | | | ✓ |
| Encaminhamento entre unidades | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

Toda operação de serviço passa por um único guard na aplicação,
`s.unidadeComServico(ctx, tx, identity, unidade, perm, domain.ServicoPaefi)`,
que soma o tipo à checagem que `s.unidade` já faz. Se a unidade não
oferta o serviço, o resultado é `ErrServicoIndisponivel`, que vira 404,
o mesmo contrato de módulo desligado. É a camada 3 do `socialRbac.js` do
legado, só que com o tipo da unidade em vez do nome do setor.

### 3. A recepção segue o tipo

- **Demanda da entrada:** nova coluna `demanda` no atendimento, com uma
  lista fechada por tipo (vinda do legado), e "Outra" com texto. Isso
  alimenta o RMA e o painel de gestão sem `ILIKE`.
- **Identificação:**
  - CRAS: exige CPF ou NIS, porque é referenciado ao CadÚnico.
  - Centro POP: permite cadastro anônimo (Decreto 7.053/2009).
  - Conselho Tutelar: a criança e o responsável.
  - Nos demais tipos vale a regra geral de hoje.
- **Painel de chamada:** é uma opção por unidade (`painel_exibe_nome`).
  - Fica **desligada por padrão** em `casa_mulher`, `casa_abrigo`, `creas`
    e `conselho_tutelar`. Nessas unidades a TV mostra e fala só a senha.
  - Na Casa Abrigo não há painel, porque não há fila.

### 4. Um espaço de trabalho por unidade no frontend

- A rota passa a ser `/sempras/u/[unidade]`. Quem é lotado em uma única
  unidade entra direto nela; o Superintendente escolhe a unidade numa
  lista em `/sempras`.
- As abas são montadas pelo tipo, a partir de `GET /v1/sempras/unidades`,
  que passa a devolver `tipo` e `servicos`.
  - CRAS: Recepção · Fila · PAIF · Benefícios · Coletivos · Ofícios.
  - CREAS: Recepção · Fila · PAEFI · MSE · Notificações.
  - E assim por diante para os outros tipos.
- **A ficha mostra só as seções do tipo da unidade em uso.** O munícipe e
  o prontuário continuam únicos (prontuário SUAS). Das outras unidades, a
  ficha mostra só "também acompanhado por: CREAS (PAEFI ativo)", sem o
  conteúdo, que segue a regra de necessidade de saber do ADR 030. A página
  de 1.594 linhas é quebrada em seções por tipo
  (`components/sempras/secoes/<tipo>/`).
- O painel de chamada e os atendimentos coletivos saem do menu lateral
  (que é da plataforma e não conhece as unidades) e viram atalhos no
  espaço da unidade, só quando ela oferta o serviço. As páginas deles
  listam só as unidades que ofertam o serviço e aceitam `?unidade=`.
- O perfil de rua do Centro POP (saúde, uso de drogas) e os documentos
  dele são do munícipe, não de uma unidade. Por isso só saem no
  prontuário para quem atua num Centro POP. Antes saíam para qualquer
  técnico.

### 5. O que não muda

- Um único plug-in `sempras` no Kernel. Os tipos **não** viram módulos.
- O cadastro do munícipe continua único e compartilhado.
- Os perfis (recepcionista, técnico, psicólogo, superintendente), os
  grupos do AD, o escopo por unidade e o `CoverageOf` continuam como
  estão.
- O encaminhamento entre unidades continua sendo a ponte entre os tipos
  (CRAS → CREAS etc.).

## Consequências

- Fecha uma brecha de integridade: hoje um CRAS consegue abrir um caso de
  PAEFI ou uma medida socioeducativa. Os registros que já existem fora do
  tipo precisam de um relatório antes da migração; nada é apagado.
- Fecha o risco de exposição do nome no painel da TV.
- O frontend fica menor por tela: cada unidade carrega só o que oferta.
- Mais um dado de configuração a manter. Uma unidade nova precisa ganhar
  tipo, e até ganhar, não atende. É o comportamento seguro.

## Fases

| Fase | Entrega |
|---|---|
| **A. Tipo e guard** (backend) | Tabela `sempras_unidades` com seed por slug, o catálogo no domínio, o guard em todas as operações de serviço, `tipo`/`servicos` em `GET /unidades`, testes "serviço de CREAS em CRAS → 404" e relatório dos registros fora do tipo |
| **B. Painel de chamada** | Opção `painel_exibe_nome` com os padrões do §3 (sai junto da A por ser risco de exposição) |
| **C. Espaço por unidade** (frontend) | `/sempras/u/[unidade]`, abas e menu pelo catálogo, a ficha em seções por tipo, o fim da regex do nome |
| **D. Recepção por tipo** | Coluna `demanda`, listas por tipo e regra de identificação por tipo |
| **E. RMA por tipo** | Formulários do MDS para CREAS e Centro POP |

## Decisões das questões em aberto (2026-10-16)

1. **CEU, SCFV Padre Lothar e Vila Olímpica** fazem atendimento individual
   com fila e também atividades coletivas: tipo `scfv`.
2. **Sede da SEMPRAS** não atende público: fica sem tipo.
3. **Casa da Mulher** recebe agora só o básico (fila sem nome no painel,
   prontuário, encaminhamento). Os serviços próprios do legado
   (tipificação da Lei Maria da Penha, medida protetiva, abrigamento
   sigiloso) entram numa fase própria.
4. **Visão entre unidades:** além do resumo, o técnico lê a **evolução
   social** das unidades que encaminharam o munícipe para a dele
   (encaminhamento pendente ou aceito). A evolução psicológica e os
   registros especializados (PAEFI, acolhimento etc.) nunca cruzam.

## Implementação

| Peça | Onde |
|---|---|
| Tabela do tipo e seed pelo slug | migração `000165`, `deploy/sempras/tipos-unidades.sql` (`make sempras-mapeamentos` e o playbook do Ansible) |
| Catálogo de serviços e demandas, `Oferta`, `ValidarEntrada` | `domain/sempras.go` |
| Guard único: `s.unidade(..., servico, perms...)` | `application/service.go` (o serviço é parâmetro obrigatório; esquecê-lo não compila) |
| Demanda da entrada | migração `000166`; `EntradaRequest.demanda` é obrigatória |
| Visão entre unidades | `Prontuario.outras_unidades` e `Prontuario.registros_origem` |
| Espaço por unidade | `/sempras` (escolha) e `/sempras/u/[unidade]` |
