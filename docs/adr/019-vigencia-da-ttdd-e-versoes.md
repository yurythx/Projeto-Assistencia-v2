# 019 — Vigência e histórico da TTDD; novas versões de procedimento

- **Status:** Aceito e implementado
- **Data:** 2026-10-03
- **Relacionado:** [ADR 017](017-ttdd-oficial.md) (TTDD oficial), [ADR 018](018-atlas-paginas-e-busca-unificada.md)

## Contexto

A TTDD muda por nova publicação da CCPAD no Diário Oficial (ADR 017: é
somente leitura na plataforma). A carga (`make ttdd-aplicar`) só inseria e
atualizava:

- **série retirada** da nova publicação continuava como vigente;
- **prazo alterado** sobrescrevia o anterior sem rastro — documentos
  produzidos antes seguem a regra da época, e ninguém sabia qual era;
- **nada mostrava o impacto** antes de gravar, nem quais procedimentos
  apontavam para séries alteradas ou retiradas.

Do lado dos procedimentos, a gestão só cadastrava e ativava/desativava:
atualizar um roteiro era recadastrar tudo do zero, com a versão seguinte, e
desativar a antiga à mão — com uma janela em que a consulta pública via
duas versões vigentes ou nenhuma.

## Decisão

1. **Revogação em vez de exclusão** (migration `000135`): a série ganha
   `revogada_em` e `revogada_edicao`. Revogada, ela sai da consulta, da
   árvore, da exportação e do assistente, mas continua acessível pelo
   código, porque documentos já produzidos (e procedimentos) continuam
   classificados nela. Série revogada **não aceita** procedimento novo nem
   nova versão (422).
2. **Histórico por gatilho** (`atlas_ttdd_historico`): toda mudança de
   descritor, prazos, destinação, observações ou vigência guarda os valores
   **anteriores**, o evento (`ALTERADA`, `REVOGADA`, `RESTABELECIDA`) e a
   edição do Diário Oficial que trouxe a mudança. Por gatilho, vale para a
   carga oficial e para qualquer outra escrita; regravar os mesmos valores
   não gera histórico. Exposto em `GET /atlas/ttdd/{codigo}/historico`.
3. **Carga em duas etapas**: `deploy/ttdd/ttdd.sql` (gerado) só traz os
   dados em tabelas temporárias; `deploy/ttdd/carga.sql` compara com o
   banco, imprime o relatório — séries novas, alteradas (antes → depois),
   revogadas, restabelecidas e **procedimentos afetados** — e grava.
   `make ttdd-impacto` faz tudo e desfaz (ROLLBACK); `make ttdd-aplicar`
   confirma. Só são revogadas séries de órgãos presentes na carga (carregar
   uma secretaria não revoga as outras).
4. **Nova versão de procedimento** (`POST /atlas/admin/workflows/{id}/versoes`,
   `atlas:manage`): cria a versão seguinte do mesmo código com o conteúdo
   enviado e desativa as demais na **mesma transação** (eventos
   `created`/`deactivated` e auditoria com `substituido_por`). Na tela, o
   formulário vem preenchido com a versão atual e **preserva** os atributos
   das peças (formato, assinatura, conferência, modelo) e as transições
   (que acompanham a etapa de destino se a ordem mudar e somem se ela for
   removida). A gestão lista as versões por `?codigo_processual=`.
5. **Na interface**: aviso de série revogada na série e no procedimento
   (com "publique uma nova versão" para a gestão), selo "Série revogada"
   nos cartões e o histórico de prazos na página da série. A escolha da
   série no formulário passou a ser por busca: a seleção listava só as 200
   primeiras das 1.686.

## Consequências

- Ao publicar nova TTDD: `make ttdd-impacto`, revisar, `make ttdd-aplicar`
  e, para cada procedimento afetado, publicar uma nova versão.
- O histórico começa na migration 000135: mudanças anteriores não foram
  registradas.
- O link de uma versão desativada deixa de abrir na consulta pública
  (continua na gestão); a Busca Global e a lista mostram só a vigente.
