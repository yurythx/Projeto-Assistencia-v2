# TTDD oficial → dados do Atlas

Converte `docs/ttdd.pdf` (Tabelas de Temporalidade e Destinação de Documentos
publicadas no Diário Oficial de Rondonópolis, aprovadas pela CCPAD) em
`deploy/ttdd/ttdd.json` (revisão) e `deploy/ttdd/ttdd.sql` (os dados, em tabelas
temporárias). Quem compara com o banco e grava é `deploy/ttdd/carga.sql`: séries
novas, alteradas (histórico dos prazos anteriores), revogadas (nunca apagadas) e
restabelecidas, com a lista de procedimentos afetados — ADR 019.

```bash
cd scripts/ttdd && npm ci && npm run tudo   # regera deploy/ttdd/
make ttdd-impacto                           # relatório do que mudaria (não grava)
make ttdd-aplicar                           # mesmo relatório + gravação (servidor)
```

## Como os dados são obtidos

1. **`extrair.mjs`** — lê o PDF com o pdf.js e grava cada fragmento de texto
   com a posição na página.
2. **`analisar.mjs`** — reconstrói a estrutura (órgão > função > subfunção >
   série documental) e lê as colunas da tabela pela posição horizontal do
   cabeçalho. Prazo, destinação e observação são centralizados na linha da
   tabela, então cada valor vai para o item cujo centro de linha está mais
   próximo. Os nomes de função e subfunção vêm do plano de classificação.
3. **`transcricao/`** — as páginas 51 a 73 do PDF (Meio Ambiente,
   Desenvolvimento Econômico e Promoção e Assistência Social) **não têm camada
   de texto**: o texto é vetorizado. A listagem foi obtida por OCR
   (`listagem-ocr.json`) e os prazos foram **transcritos e conferidos
   visualmente** contra as páginas renderizadas (`prazos-*.tsv/txt`).
4. **`consolidar.mjs`** — junta tudo, aplica as correções de divergências do
   próprio documento (cada uma registrada em `divergencias` no JSON) e gera
   o SQL dos dados.

## Divergências do documento oficial

Estão no campo `divergencias` de `deploy/ttdd/ttdd.json` (códigos repetidos,
numeração diferente entre o plano e a tabela, seção incompleta etc.). Devem
ser levadas à CCPAD para correção na próxima versão da TTDD.
