# 017 — TTDD do Atlas fiel ao documento oficial

- **Status:** Aceito e implementado — atualização da tabela complementada pelos ADRs [019](019-vigencia-da-ttdd-e-versoes.md) (vigência e histórico) e [022](022-atualizar-ttdd-pela-tela.md) (pela tela)
- **Data:** 2026-10-02
- **Relacionado:** [ADR 015](015-atlas-padronizado.md) (Atlas padronizado)

## Contexto

A TTDD (Tabela de Temporalidade e Destinação de Documentos) do município é
publicada no Diário Oficial de Rondonópolis e aprovada pela CCPAD
(`docs/ttdd.pdf`). A revisão do Atlas contra esse documento mostrou que o
modelo e os dados não o representavam:

- **Estrutura plana**, quando a TTDD é hierárquica: órgão (`2.0`) > função
  (`2.0.01`) > subfunção (`2.0.01.00`) > série documental (`2.0.01.00.00`).
- **Prazo só em anos (inteiro obrigatório).** O documento usa também
  condições — "Enquanto estiver vigorando" (663 séries), "30 dias após a
  data do evento", "03 meses" —, fase intermediária inexistente ("–") e
  destinação não definida ("X — entregue ao paciente").
- **Sem recomendações da subfunção** (ex.: Contabilidade da Fazenda:
  "transferência para o Arquivo Permanente após a aprovação do TCE-MT") e
  sem a fonte (edição do Diário Oficial, data, versão).
- **20 séries semeadas à mão**, com descritores reescritos e dois prazos
  divergentes do documento: `2.0.07.00.00` (semente 5 + 50 anos e guarda
  permanente; oficial 1 + 99 anos e eliminação) e `2.0.05.00.00`. O
  procedimento de Dispensa apontava para `2.0.02.00.04` (compra direta de
  serviços técnicos especializados); dispensa é `2.0.02.01.02`.
- **O assistente não respondia sobre temporalidade** — só sobre
  procedimentos.

## Decisão

1. **Modelo (migration `000134`):** tabelas `atlas_ttdd_orgaos` (com
   edição/data/versão da publicação), `atlas_ttdd_funcoes`,
   `atlas_ttdd_subfuncoes` (com recomendação); na série, cada fase tem
   `*_anos` **ou** `*_condicao` (CHECK), destinação anulável, descritor
   `TEXT`, código validado (`N.0.FF.SS.II`, com `-2` só para código
   repetido no documento) e busca full-text.
2. **Dados oficiais, não digitados:** `scripts/ttdd/` extrai do PDF por
   coordenadas (pdf.js) e reconstrói a tabela; as páginas 51–73, que não
   têm camada de texto, foram lidas por OCR (listagem) e conferidas à mão
   (prazos). Saída: `deploy/ttdd/ttdd.json` (revisão) e `ttdd.sql`
   (`make ttdd-aplicar`, idempotente) — **1.686 séries de 9 órgãos**. A
   migration corrige as 20 sementes e a Dispensa, para que toda instalação
   fique consistente mesmo sem a carga completa.
3. **Divergências do próprio documento** são registradas em
   `deploy/ttdd/ttdd.json` (`divergencias`) e tratadas sem inventar dado:
   numeração da tabela diferente do plano (vale o plano), prefixo errado
   (`5.0` em série da Administração), códigos repetidos (segundo vira
   `-2`), coluna corrente em branco (vira "não informado"), seção 13.0
   truncada no PDF (não importada).
4. **TTDD é somente leitura na plataforma:** é norma aprovada pela CCPAD;
   muda por nova publicação e nova carga, não pela tela.
5. **Consulta:** filtro por prefixo hierárquico (`?codigo=2.0.01`),
   árvore com contagens (`GET /atlas/ttdd/estrutura`), série com a
   hierarquia, a recomendação e a fonte.
6. **Assistente:** séries da TTDD também sustentam respostas (mesmo
   limiar 0,65, `domain.RelevanciaTTDD`); a síntese cita prazos exatamente
   como na TTDD (inclusive condições), prazo total quando ambas as fases
   são em anos, recomendação e fonte. O modelo de linguagem recebe o
   contexto já redigido a partir dos dados oficiais.

## Consequências

- Atualização da TTDD = nova versão do PDF em `docs/`, `npm run tudo` em
  `scripts/ttdd/` (revisar as divergências) e `make ttdd-aplicar`.
- As divergências listadas devem ir à CCPAD para correção na próxima
  versão do documento.
- A API das fontes do assistente mudou: `sources[]` traz `tipo`
  (`procedimento`|`ttdd`), `codigo`, `titulo`, `relevancia` e `id` (só
  procedimento).
