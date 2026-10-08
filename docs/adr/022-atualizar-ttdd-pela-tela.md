# 022 — Atualizar a TTDD pela tela

- **Status:** Aceito e implementado
- **Data:** 2026-10-03
- **Relacionado:** [ADR 017](017-ttdd-oficial.md) (TTDD oficial), [ADR 019](019-vigencia-da-ttdd-e-versoes.md) (vigência, histórico, `make ttdd-impacto`)

## Contexto

Com o ADR 019, uma nova publicação da TTDD entra por comando no servidor
(`make ttdd-impacto` e `make ttdd-aplicar`). Quem cuida da tabela — a
gestão documental — não tem acesso ao servidor, e a tabela muda a cada
publicação da CCPAD.

## Decisão

1. **Tela Atlas → Tabela de Temporalidade → Atualizar TTDD**
   (`/atlas/ttdd/atualizar`, permissão `atlas:manage`, que só vale com
   concessão global).
2. **Dois formatos de entrada:**
   - **CSV da própria exportação** (`GET /atlas/ttdd/exportar`): baixar,
     revisar no Excel mantendo o cabeçalho e reenviar. Os prazos são lidos
     como a exportação os escreve ("1 ano", "Enquanto estiver vigorando",
     "não há", "não informado na TTDD"), a destinação por extenso, a
     publicação do órgão pela coluna Fonte; aceita BOM, `;` ou `,` e
     ignora linhas em branco;
   - **`ttdd.json`** gerado do PDF por `scripts/ttdd` (a extração com OCR
     e conferência continua fora do sistema).
3. **Simular antes de aplicar:** `POST /atlas/admin/ttdd/carga/simular`
   valida a carga inteira (até 20 problemas por vez, com a linha) e roda a
   comparação numa transação desfeita: totais por situação, antes e depois
   de cada série alterada, séries revogadas, restabelecidas e novas, e os
   **procedimentos afetados** (com link). Nada é gravado.
4. **Aplicar o que foi conferido:** `POST /atlas/admin/ttdd/carga/aplicar`
   exige o **hash** (SHA-256) devolvido pela simulação — outro arquivo é
   recusado — e a confirmação na tela. Grava na mesma transação: órgãos
   (edição do Diário Oficial), funções, subfunções, séries novas e
   alteradas (o gatilho do ADR 019 guarda o histórico), revogação das que
   saíram (só dos órgãos presentes na carga) e a auditoria
   `atlas.ttdd.carga.aplicada` (totais, formato, hash).
5. **Mesmas regras do comando:** o repositório reproduz
   `deploy/ttdd/carga.sql` (tabelas temporárias, comparação, gravação) — os
   dois devem andar juntos.

## Consequências

- A gestão atualiza a TTDD sem acesso ao servidor; o comando continua para
  quem opera pela linha de comando.
- Limite de 10 MB por arquivo (a tabela inteira tem ~600 KB).
- Funções e subfunções que saem da tabela continuam cadastradas (só as
  séries são revogadas).
