# 029 — Organograma da TTDD no Atlas

- **Status:** Aceito e implementado
- **Data:** 2026-10-05
- **Relacionado:** [ADR 025](025-atlas-em-escala.md) (cobertura), [ADR 028](028-validacao-dos-procedimentos.md) (validação e procedimentos por secretaria)

## Contexto

A TTDD organiza os documentos da Prefeitura por secretaria (órgão), função
e subfunção. Hoje são 9 secretarias, 41 funções, 134 subfunções e 1.686
séries. Na prática, essa hierarquia é o organograma funcional da
Prefeitura. As entrevistas de validação (ADR 028) são feitas por função,
que em geral corresponde a um departamento.

A gestão documental precisa de uma visão didática dessa estrutura, que
também possa ser impressa. Ela deve mostrar onde estão as séries e onde
já existem procedimentos e onde não existem.

## Decisão

1. **API** (`GET /atlas/organograma` público e `GET /atlas/admin/organograma`
   para a gestão): devolve secretaria → função → subfunção.
   - **Contagens de cada nó:** séries vigentes, procedimentos (publicados
     e, para a gestão, em validação e rascunhos) e lacunas.
   - **Lacuna:** subfunção com série de processo e nenhum procedimento.
     Para o público, nenhum procedimento *publicado*.
   - **Consulta:** uma só, no nível da subfunção. O domínio
     (`MontarOrganograma`) soma as funções e os órgãos e esconde os
     rascunhos do público.
   - **Rascunhos:** contam só a versão mais recente de cada código, como
     no painel.
2. **Aba Organograma** (`/atlas/organograma`), aberta a todos, com quatro
   visualizações. A visualização e a secretaria escolhidas ficam na URL
   (`?visao=&orgao=`):
   - **Organograma em caixas:** a secretaria no topo e as funções em
     caixas ligadas por linhas (até 6 por linha), com as subfunções
     listadas. Imprime em paisagem, uma secretaria por página.
   - **Árvore em tópicos:** lista recuada que abre e fecha, com as séries
     carregadas ao abrir a subfunção. Imprime em retrato, como está na
     tela.
   - **Mapa de blocos:** treemap (*squarified*) com a área proporcional
     às séries e a cor pela fração de subfunções com procedimento. Com
     uma secretaria escolhida, mostra as funções divididas em
     subfunções. Imprime em paisagem, com as cores.
   - **Fichas por secretaria:** cartaz de uma página por secretaria, com
     o quadro-resumo, as funções em colunas e a lista das lacunas.
3. **Navegação:** a função abre os procedimentos do departamento
   (`/atlas/secretarias/{prefixo}?funcao=`). A subfunção abre as séries
   na TTDD.
4. **Impressão:** a orientação do papel muda conforme a visualização
   (`@page` na própria página) e cada secretaria começa numa página nova.

## Consequências

- O organograma é o da **TTDD** (funcional), não o do cadastro de unidades
  (o do AD, com as siglas). Os dois podem divergir: na TTDD, por exemplo, a
  Administração e a Gestão de Pessoas são um só órgão. Cruzar as duas
  estruturas é trabalho para depois das entrevistas: proposta no item 1.6
  de [Alterações planejadas](../atlas/PENDENCIAS.md).
- Os números se atualizam sozinhos com os procedimentos criados e
  homologados, sem nova carga.
