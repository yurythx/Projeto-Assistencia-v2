# 018 — Atlas em páginas próprias, busca unificada e assistente em gaveta

- **Status:** Aceito e implementado — o assistente passou a tratar só da TTDD ([ADR 021](021-assistente-restrito-a-ttdd.md))
- **Data:** 2026-10-03
- **Relacionado:** [ADR 015](015-atlas-padronizado.md), [ADR 017](017-ttdd-oficial.md)

## Contexto

O Atlas era uma única tela com três abas (procedimentos, TTDD e
assistente) e o detalhe do procedimento num painel lateral selecionado por
`?procedimento=`. Com a TTDD oficial (1.686 séries, ADR 017) e o assistente
respondendo sobre temporalidade, o formato limitava o uso:

- **sem URL por série**: não dava para enviar a alguém "a série 3.0.03.00.04"
  nem imprimir uma ficha;
- **duas buscas separadas** (procedimentos e TTDD), e quem não sabe se a
  dúvida é de tramitação ou de guarda precisava tentar nas duas;
- **o assistente ficava numa aba**: perguntar sobre o que se está lendo
  exigia sair da página e redigitar o contexto;
- a TTDD era uma tabela de 50 linhas por página, com a recomendação da
  subfunção repetida em cada série e o plano de classificação só em listas
  de seleção.

Portais de conhecimento e de normas (bases de suporte, catálogos de
serviços públicos, consultas de legislação) resolvem isso com páginas
canônicas por item, uma busca única com sugestões por tipo e a ajuda
contextual numa gaveta.

## Decisão

1. **Páginas com URL própria**: `/atlas` (início), `/atlas/procedimentos/{id}`,
   `/atlas/ttdd` (consulta, filtros na URL) e `/atlas/ttdd/{codigo}`
   (série). Trilha de navegação em todas e estilo de impressão (só o
   conteúdo). Os links antigos (`?procedimento=`, `?ttdd=`) redirecionam;
   a Busca Global e as fontes do assistente já apontam para as páginas
   novas.
2. **Busca unificada** (combobox WAI-ARIA, com teclado): consulta
   procedimentos e séries em paralelo nas rotas públicas existentes e
   oferece "ver todas as séries" e "perguntar ao assistente". Sem endpoint
   novo de busca.
3. **Assistente em gaveta lateral** em todas as páginas do Atlas (layout do
   segmento). "Perguntar sobre este procedimento/esta série" **só preenche**
   a pergunta; nada é enviado sem a pessoa confirmar.
4. **TTDD navegável**: plano de classificação em árvore (abre só o ramo do
   código selecionado) e séries agrupadas por subfunção, com a recomendação
   uma vez por grupo.
5. **Calculadora de temporalidade** no cliente (`lib/atlas/temporalidade.ts`),
   rotulada como estimativa; não calcula o que a TTDD deixa por condição.
6. **Exportação CSV** da TTDD (`GET /atlas/ttdd/exportar`), pública como a
   consulta, com os mesmos filtros.

## Consequências

- Cada série e cada procedimento passam a ser referenciáveis (e-mail,
  processo, impressão).
- A busca faz duas requisições por termo (com espera de 250 ms entre
  teclas); as rotas são paginadas a 5 itens e já têm limite por IP.
- O cálculo de datas fica no navegador e não é auditado: é orientação, não
  ato administrativo. A eliminação continua dependendo da CCPAD.
- Ficaram fora desta rodada a avaliação "útil/não útil" das respostas e o
  painel de lacunas do assistente: exigem guardar o texto das perguntas,
  o que hoje é evitado por minimização (LGPD).
