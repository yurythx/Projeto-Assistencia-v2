# 012 — Estrutura desativada deixa de conceder perfis e de receber processos

- **Status:** Aceito
- **Data:** 2026-09-27
- **Escopo:** `internal/platform/iam` (resolvedor),
  `internal/modules/iam`, `internal/modules/tramite`, migration 000126,
  telas do Trâmite

## Contexto

Este ponto apareceu ao documentar as regras de negócio
([REGRAS_DE_NEGOCIO.md](../REGRAS_DE_NEGOCIO.md)). O campo "ativo" de
entidade, unidade e departamento só afetava o Diretório. As consequências
eram três:

- **Lotação e mapeamento seguiam concedendo o perfil** quando o escopo
  estava numa unidade desativada. Desativar um setor não tirava o acesso
  de ninguém.
- **O Trâmite abria processos em unidade desativada e tramitava para ela.**
  Um processo podia ir parar numa unidade sem ninguém lotado, e só um
  gestor o tiraria de lá.
- **Excluir uma unidade-mãe** deixava as subunidades sem mãe, sem aviso
  (`ON DELETE SET NULL`), ao contrário do resto da estrutura
  (`RESTRICT`, ADR 008).

## Decisão

1. **Escopo ativo como regra única.** A função
   `nexus_scope_active(entidade, unidade, departamento)`, da migration
   000126, vale só se cada nível informado existe e está ativo. Ela é usada
   pelo resolvedor do IAM (permissões por requisição) e pelo cálculo de
   permissões da tela de usuários, que antes repetia a regra em SQL
   próprio.
2. **Efeito imediato.** Salvar entidade, unidade ou departamento invalida o
   cache do IAM em todas as réplicas, como já acontecia com perfis,
   lotações e mapeamentos.
3. **Trâmite.** Abrir processo e tramitar exigem a unidade **e a entidade
   dela** ativas, inclusive para `tramite:manage`. A checagem trava as
   linhas (`FOR SHARE`) até o fim da transação: ninguém exclui nem desativa
   a unidade entre a checagem e a gravação. O mesmo passou a valer para o
   tipo de processo. Os tratamentos de violação de chave estrangeira ficaram
   inalcançáveis e foram removidos. As telas deixam de oferecer unidades de
   órgão desativado.
4. **Unidade-mãe.** `unidades.parent_id` passa a ser `ON DELETE RESTRICT`.
   Excluir uma unidade com subunidades responde `409`.

## Consequências

- **Desativar vira a forma de tirar de operação sem apagar.** Reativar
  devolve os acessos, sem refazer lotações.
- **Processos que já estão numa unidade desativada** continuam
  consultáveis. Para movimentá-los, é preciso reativar a unidade (ou
  movimentá-los antes de desativar).
- **Testes:**
  - `TestIAMEscopoDesativadoNaoConcede`: lotação e grupo; departamento,
    unidade e entidade;
  - `TestIAMUnidadeMaeComSubunidadesNaoEExcluida`;
  - `TestTramiteUnidadeDesativada`;
  - a matriz de falhas do Trâmite cobre `UnidadeAtiva`;
  - teste da tela do Trâmite com órgão desativado.

  Cobertura de 100% por pacote mantida, e a migration passa por up, down e
  up.
