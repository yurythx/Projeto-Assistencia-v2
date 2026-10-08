# 025 — Atlas em escala: importação em lote, cobertura e busca

- **Status:** Aceito e implementado
- **Data:** 2026-10-04
- **Relacionado:** [ADR 022](022-atualizar-ttdd-pela-tela.md) (o mesmo padrão simular → aplicar), [ADR 023](023-assistente-fluxos-e-ttdd.md) (assistente), [ADR 024](024-biblioteca-de-modelos.md) (modelos)

## Contexto

Em produção, a TTDD tem 1.686 séries vigentes, mas só 3 procedimentos
foram homologados, e nenhuma peça ou série tem modelo. O código do Atlas
estava pronto; faltava como **povoá-lo** em escala e como **acompanhar**
o que falta. Também havia achados menores:

- A Busca Global encontrava só procedimentos.
- O assistente não citava os modelos para baixar.
- A síntese de um fluxo misturava as diligências com o caminho normal.

## Decisão

1. **Importação de procedimentos em lote**
   (`/atlas/importar`, `atlas:manage`):
   - **Formato:** um arquivo JSON (`{"procedimentos": [...]}`) com o
     mesmo formato da **exportação**
     (`GET /atlas/admin/workflows/exportar`, `procedimentos.json`). O
     ciclo é exportar, editar e importar de volta. A peça aponta para o
     modelo da biblioteca pelo **nome**. Campo desconhecido é erro, para
     pegar nomes de campo digitados errado. Até 500 procedimentos por
     arquivo.
   - **Situação de cada item:**
     - **NOVO**: o código ainda não existe.
     - **NOVA_VERSAO**: o conteúdo é diferente da versão em vigor.
     - **INALTERADO**: a mesma *assinatura* de conteúdo, que ignora a
       ordem das peças e das transições.
     - **ERRO**: a regra que falhou, por linha. Exemplos: título
       ausente, série revogada, modelo inexistente, código repetido no
       arquivo.
   - **Simular** roda o cadastro de verdade numa transação desfeita, com
     um *savepoint* por item. Assim, um item com erro não derruba os
     outros, e as regras são exatamente as do cadastro pela tela.
   - **Aplicar** só aceita o arquivo simulado (hash) e **sem nenhum
     erro**. A operação é auditada (`atlas.procedimentos.importados`), e
     cada versão nova emite os eventos e a auditoria de sempre.
2. **Modelos em lote** (biblioteca → "Enviar vários"):
   - O nome do modelo vem do nome do arquivo.
   - Um código de série no início do nome
     (`2.0.02.00.07 - Edital.docx`) liga o modelo à série.
   - Cada arquivo é tratado sozinho, e o resultado aparece por arquivo.
3. **Painel de cobertura** (`/atlas/cobertura`,
   `GET /atlas/admin/cobertura`, `atlas:manage`):
   - **Totais e por secretaria:** séries com fluxo e com modelo.
   - **Lista de trabalho:** as peças de procedimentos em vigor sem modelo
     (nem próprio nem da série) e os modelos ativos sem uso.
4. **Busca Global:** passa a trazer as **séries vigentes da TTDD** e os
   **modelos ativos**, além dos procedimentos.
   - O código exato de uma série a coloca em primeiro.
   - O resultado de modelo abre a biblioteca já filtrada
     (`/atlas/modelos?q=`).
5. **Assistente:** cada fonte da resposta (série ou procedimento) leva os
   **modelos ativos da série** para baixar. A síntese do fluxo mostra o
   caminho normal antes das devoluções em diligência.

## Consequências

- Povoar o Atlas deixa de depender de cadastrar fluxo por fluxo na tela.
  Uma planilha de levantamento pode virar o JSON, e o arquivo exportado é
  o modelo.
- A importação **nunca apaga** nem desativa procedimentos fora do
  arquivo: retirar um fluxo continua sendo um ato explícito (desativar).
- O tempo da matriz de falhas cresceu com a importação (cerca de 4
  minutos no pacote `application`), dentro do limite do `go test`.
