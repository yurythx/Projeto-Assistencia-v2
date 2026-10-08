# 028 — Validação dos procedimentos (rascunho → em validação → homologado)

- **Status:** Aceito e implementado
- **Data:** 2026-10-04
- **Relacionado:** [ADR 025](025-atlas-em-escala.md) (importação em lote), [ADR 027](027-notificacoes.md) (avisos)

## Contexto

Os fluxos do Atlas serão levantados por **entrevistas** em cada
departamento, que também vão padronizar os documentos. O ponto de partida
são rascunhos gerados a partir da TTDD.

Até aqui, todo procedimento cadastrado ou importado era publicado na hora:
entrava na consulta, no site público, na busca e no assistente. Um rascunho
ainda não validado apareceria como fluxo oficial.

## Decisão

1. **Situação do procedimento** (migração 000141): `RASCUNHO`,
   `EM_VALIDACAO` ou `HOMOLOGADO`.
   - O banco garante a regra central: um procedimento só fica **ativo**
     (publicado) se estiver **homologado** (`CHECK (NOT ativo OR situacao =
     'HOMOLOGADO')`).
   - Os procedimentos que já existiam ficam como homologados.
   - O cadastro e a nova versão pela tela continuam publicando direto, como
     ato da gestão.
2. **Importação como rascunho** (padrão na tela de importação):
   - Os procedimentos entram inativos, como `RASCUNHO`.
   - O rascunho de nova versão de um código já publicado **não substitui** a
     versão em vigor e **não avisa** ninguém.
   - Rascunhos não aparecem na consulta, no site público, na busca nem no
     assistente.
3. **Registro das entrevistas** (`atlas_validacoes`):
   - **Conteúdo:** data (não futura), departamento, participantes (pela
     função), o que foi validado ou corrigido, e pendências.
   - **Escopo:** é pelo **código** do procedimento, então vale para as
     versões seguintes.
   - **Imutável:** uma correção vira um novo registro.
   - O primeiro registro passa o rascunho para **em validação**.
   - A auditoria guarda o departamento e a data, sem os participantes
     (minimização).
4. **Homologar** (`POST /atlas/admin/workflows/{id}/homologar`): publica a
   versão, ativando-a e desativando as demais do mesmo código.
   - Emite os eventos de substituição; o Trâmite avisa os processos
     abertos.
   - Avisa quem segue o fluxo e as unidades das etapas (ADR 027).
   - Exige que a série da TTDD esteja **vigente** no momento da publicação.
   - Um procedimento homologado **não volta** a rascunho: a revisão é uma
     nova versão.
   - **Alternar a situação** (`POST …/situacao`) só vale entre rascunho e em
     validação.
5. **Acompanhamento:** o painel de cobertura mostra quantos procedimentos
   estão em rascunho e em validação. Os cartões mostram a situação, e a
   página do procedimento tem o cartão **Validação**, com a situação, as
   entrevistas e a homologação com confirmação.
6. **Conteúdo inicial e método:**
   - `deploy/atlas/gerar-rascunhos.mjs` gera 28 rascunhos dos processos
     prioritários das secretarias, já conferidos contra a TTDD de produção
     por simulação.
   - `deploy/atlas/gerar-rascunhos-2.mjs` gera a segunda leva, com 64
     rascunhos para as subfunções que ainda não tinham fluxo. Depois dela,
     restam 4 lacunas, deixadas para as entrevistas.
   - `docs/atlas/PLANO_DE_VALIDACAO.md` traz o roteiro de entrevista, o
     checklist de padronização dos documentos e o checklist antes de
     homologar.

7. **Também pela tela:** o cadastro e a nova versão de procedimento
   vêm marcados "Salvar como rascunho" (desmarcado, publica direto). Assim, um
   procedimento revelado numa entrevista entra no mesmo ciclo.
8. **Ficha de validação** (`/atlas/procedimentos/{id}/ficha`): versão para
   imprimir e levar à entrevista, com o fluxo proposto item a item
   ("confere / corrigir"), a tabela de padronização de cada documento e o
   fechamento com assinaturas.
9. **Lacunas no painel de cobertura:** subfunções da TTDD com séries de
   processo e nenhum procedimento, nem rascunho. É uma heurística pelo
   início do descritor ("Processo", "Requerimento", "Solicitação",
   "Licença", "Certidão"…). A lista é atualizada sozinha conforme os
   procedimentos são criados.
10. **Procedimentos por secretaria:** as entrevistas são feitas por
    secretaria, então o Atlas organiza os procedimentos pelo órgão da TTDD,
    como já fazia com a temporalidade.
    - **API:** `GET /atlas/workflows/secretarias` (público, só publicados)
      e `/atlas/admin/workflows/secretarias` (com rascunhos e em
      validação).
    - **Filtros da lista:** `prefixo_ttdd` e, para a gestão, `situacao`
      e `ultima=true` (a versão mais recente de cada código, onde fica o
      rascunho corrigido).
    - **Página da secretaria:** `/atlas/secretarias/{prefixo}`, com os
      procedimentos agrupados pela função, filtros de **função**
      (`?funcao=2.0.06`, que a API recebe como `prefixo_ttdd`) e de
      situação, as lacunas da secretaria (ou da função) e o
      **caderno da entrevista**, que junta as fichas de todos os
      procedimentos para imprimir de uma vez.
    - **Contagem:** os rascunhos e os procedimentos em validação contam só
      a versão mais recente de cada código. Assim, um rascunho corrigido não
      conta duas vezes.

## Consequências

- O que ficou provisório ou para depois está em
  [Alterações planejadas](../atlas/PENDENCIAS.md).
- Os fluxos podem ser levantados, corrigidos e padronizados sem nunca
  aparecer como oficiais antes da homologação.
- **Siglas das unidades:** a sede de cada órgão levava a sigla "SEDE", e
  os avisos às unidades dependem da sigla. Agora a sede leva a sigla do
  órgão, e as etapas dos rascunhos usam as mesmas siglas. As siglas são
  provisórias até a confirmação nas entrevistas (plano de validação, §7).
- **Educação:** não tem órgão na TTDD, e o enquadramento dos documentos
  escolares depende da CCPAD.
