# 023 — Assistente sobre o acervo do Atlas: fluxos do início ao fim e TTDD

- **Status:** Aceito e implementado
- **Data:** 2026-10-04
- **Relacionado:** [ADR 021](021-assistente-restrito-a-ttdd.md) (que este amplia), [ADR 020](020-conexoes-de-ia.md) (conexões de IA)

## Contexto

O ADR 021 restringiu o assistente à TTDD: perguntas sobre como um
processo tramita eram recusadas ("Esse assunto foge do objetivo da IA"). Na
prática, o Atlas é o acervo documental inteiro — os **fluxos** homologados
(procedimentos SEI: etapas, setores, prazos, peças) e a **TTDD** (por quanto
tempo guardar o que esses fluxos produzem). Recusar "como funciona o fluxo
de licitação?" tirava do servidor justamente a orientação que o Atlas tem
de melhor.

## Decisão

1. **Escopo = acervo do Atlas:** fluxos homologados **e** TTDD. Qualquer
   outro assunto continua recebendo *"Esse assunto foge do objetivo da IA:
   este assistente responde apenas sobre os fluxos documentais do Atlas
   (como cada processo tramita, do início ao fim) e a Tabela de
   Temporalidade…"*.
2. **Resposta de fluxo do início ao fim** (`domain.SinteseFluxo`): código,
   título e objetivo; prazo previsto total; em cada etapa, o setor e a
   sigla, o prazo, **o que fazer**, as **peças exigidas** (obrigatória ou
   opcional, formato, assinatura, conferência da cópia, modelo), a
   **condição para seguir** à próxima etapa e as **devoluções em
   diligência**; "Fim do fluxo"; e a **guarda dos documentos** (série da
   TTDD do procedimento). O modelo de IA, quando configurado, recebe a mesma
   estrutura e a instrução de responder nessa ordem.
3. **Relevância por procedimento** (`domain.RelevanciaProcedimento`), com a
   mesma regra das séries: palavras no início de palavra, plural e gênero;
   palavras que descrevem a pergunta ("fluxo", "funciona", "etapa",
   "tramitar", "processo", "prazo"…) não diluem quando o procedimento não as
   contém; bônus para o código e o título.
4. **Foco da pergunta:** pergunta de fluxo ("como funciona", "etapas",
   "tramitar", "o que preciso para dar continuidade") responde com o
   procedimento (que já traz a temporalidade); pergunta de temporalidade
   ("prazo de guarda", "destinação"), com a série; pergunta que fala dos
   dois, ou de nenhum, mantém os dois tipos. Se o tipo pedido não tem fonte
   acima do limiar, vale o outro.
5. **Recusas:** pedido de tarefa ("me ajuda a escrever", "resuma",
   "traduza") continua recusado direto; fluxo ou temporalidade sem fonte →
   *"Não localizei um fluxo homologado nem uma série da TTDD…"*.
6. **Interface:** boas-vindas e sugestões falam dos dois; fontes de fluxo
   levam à página do procedimento; na página do procedimento, "Perguntar
   sobre este fluxo" pergunta como ele funciona do início ao fim.

## Consequências

- O assistente responde sobre o que o Atlas tem cadastrado: um fluxo que
  ainda não foi homologado no Atlas não tem resposta ("Não localizei…"); a
  gestão cadastra os fluxos pela tela.
- A auditoria continua sem o texto da pergunta; as fontes registradas
  passam a incluir códigos de procedimento.
