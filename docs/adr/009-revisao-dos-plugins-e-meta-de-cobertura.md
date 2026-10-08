# 009 — Revisão das regras de negócio dos plug-ins e meta de 100% de cobertura por módulo

- **Status:** Aceito
- **Data:** 2026-09-26
- **Escopo:** todos os plug-ins em `internal/modules/*`, o núcleo de auditoria (`internal/platform/audit`), utilitários de teste (`database/dbtest`, `storage/storagetest`, `scripts/genfault.py`), CI

## Contexto

O ADR 008 levou a cobertura do backend a 70% e colocou os testes de
integração no CI. Os plug-ins ainda ficavam entre 67% e 89%, e o que ficava
de fora eram justamente os caminhos que definem se uma regra de negócio vale
ou não:

- as recusas (estado inválido, permissão insuficiente, recurso inexistente);
- as falhas de banco e de armazenamento no meio de uma transação;
- os handlers com entrada malformada.

Cada plug-in foi revisado separadamente, na ordem abaixo. Para cada um:

1. ler o fluxo de trabalho para o qual o módulo foi criado;
2. confrontá-lo com o código;
3. corrigir as regras que não eram aplicadas;
4. cobrir cada caminho com teste.

## Decisão

### 9.1. Correções de regra de negócio, por módulo

| Módulo | Principais correções |
|---|---|
| **Trâmite** | O fluxo agora é aplicado nos estados finais e na abertura:<br>• tipo de processo desativado não abre processo novo;<br>• processo concluído ou arquivado não recebe documento, anexo, edição nem pedido de assinatura;<br>• não se conclui processo com assinatura pendente.<br>Credenciais:<br>• credencial de acesso só em processo restrito ou sigiloso;<br>• a credencial agora pode ser revogada (migration 000122).<br>Correções pontuais:<br>• o filtro "minha unidade" trazia processos de quem já tinha saído dela;<br>• curingas do LIKE vazavam na busca. |
| **Signum** | Cerimônia de assinatura:<br>• envelope, vez e documento são validados **antes** da senha, e uma tentativa errada não consome o limite;<br>• falha de reautenticação é sempre auditada.<br>Signatários e Keycloak:<br>• signatário desativado é recusado na abertura;<br>• Keycloak fora do ar responde 503. |
| **IAM** | Arquitetura: a aplicação depende da porta `domain.Repository`.<br>Administração de contas (A01):<br>• só administra uma conta quem cobre as permissões efetivas dela. Antes, `users:manage` redefinia a senha de um administrador;<br>• conta federada não recebe senha local;<br>• retirar permissões também exige cobri-las.<br>Estrutura: unidade e departamento não trocam de pai, e ciclos e mães inválidas respondem 422. |
| **Arquivos** | Mover:<br>• o destino de um arquivo movido exige escrita, e o dono conseguia colocá-lo em pasta somente leitura;<br>• mover pasta para a raiz exige ser o dono dela.<br>Limpeza e ACL:<br>• a limpeza de uploads abandonados não deixa mais objeto órfão;<br>• ACL com o mesmo sujeito repetido é consolidada numa entrada só. |
| **Mercúrio** | Sala arquivada:<br>• a sala arquivada é somente leitura também para edição.<br>Perda de acesso:<br>• quem perde acesso à sala deixa de editar e de apagar o que escreveu nela;<br>• mudar o grupo do AD ou o departamento da sala derruba as inscrições em tempo real.<br>Arquitetura: porta `Publisher`. |
| **Egress** | Destino desativado: as entregas já enfileiradas **esperam** sem gastar tentativas.<br>Falhas visíveis:<br>• segredo indecifrável vira tentativa falha visível;<br>• falha ao registrar o resultado da entrega é logada.<br>Arquitetura: porta `Deliverer`, com o anti-SSRF na infraestrutura. |
| **Agenda** | Eventos e salas:<br>• evento cancelado é imutável;<br>• sala desativada recusa reserva nova, mas não trava eventos que já estão nela;<br>• sala com reserva futura não é excluída.<br>Ocupação: consultar a ocupação de sala inexistente responde 404. |
| **Catálogo** | Publicação:<br>• publicar exige resumo e canal de atendimento (Lei 13.460/2017, art. 7º);<br>• republicar não reemite o evento.<br>Exclusão e cadastro:<br>• serviço publicado não é excluído;<br>• slug vazio é recusado;<br>• unidade inexistente responde 422. |
| **Blog** | Publicação:<br>• publicar exige texto;<br>• editar uma publicação no ar sem enviar o texto era aceito e apagava o conteúdo publicado. Agora é recusado.<br>Capa: trocar a capa remove a antiga do armazenamento. |
| **Contato** | O responsável da triagem precisa ser uma conta existente e ativa.<br>Testes provam que:<br>• o evento não leva dados pessoais;<br>• a leitura da mensagem é auditada. |
| **Diretório** | Lotação:<br>• o autoatendimento definia a própria lotação no primeiro salvamento, e isso agora é só da gestão;<br>• o departamento precisa ser coerente com a unidade.<br>Busca: curingas do LIKE não vazam. |
| **Wiki** | Árvore de páginas:<br>• mover para uma página-mãe inexistente responde 404, e antes estourava como "possui subpáginas";<br>• erro de banco não é mascarado como ciclo.<br>Páginas: slug vazio é recusado, e o histórico de uma página inexistente responde 404. |
| **Busca** | Os testes passavam `modules=`, um parâmetro inexistente e ignorado em silêncio. O parâmetro correto é `module=`. |
| **Auditoria** | Registro:<br>• o User-Agent era cortado em bytes, podia partir um caractere UTF-8 e derrubava a transação auditada.<br>Consulta e exportação:<br>• curingas do LIKE vazavam no filtro de ação;<br>• a exportação LAI pulava linhas ilegíveis em silêncio;<br>• o IP saía com `/32`.<br>Cópia WORM:<br>• passa a retomar da marca d'água, sem revisitar todo o histórico a cada 6 h;<br>• um erro ao ler o SHA anterior quebrava a cadeia de hash sem aviso. |
| **Example** (blueprint) | É o que `make new-module` copia, então todo módulo gerado herdava as divergências. Alinhado às convenções:<br>• `database.DBTX` em todos os métodos;<br>• paginação padrão, sem `null` na lista vazia;<br>• invariantes no domínio e `MapError`.<br>Limpeza: código morto removido e `create-module.sh` corrigido. |

Regras transversais que surgiram em vários módulos e agora são convenção:

- **Erro de banco nunca vira regra de negócio.** A falha numa checagem de
  ciclo, de acesso ou de existência é propagada como erro, e não mascarada
  como "ciclo", "sem acesso" ou "não encontrado".
- **Curingas digitados são texto.** As buscas usam `strpos`/`starts_with`
  ou escapam `%`, `_` e `\`.
- **Estado final é final.** Registros cancelados, arquivados ou concluídos
  recusam mutação, e repetir a transição para o estado atual não gera nova
  auditoria nem novo evento.
- **Dependências externas atrás de portas** (Publisher, Deliverer,
  SignaturePort). É o que permite testar a aplicação sem Hub, sem HTTP real
  e sem outro módulo.

### 9.2. Estratégia de testes dos plug-ins

| Técnica | Onde | O que prova |
|---|---|---|
| Sistema pela API | `internal/app/module_<m>_http_test.go` | O fluxo de trabalho completo com router real, IAM, Guard, auditoria e outbox. `h.deliver` entrega o outbox aos consumidores reais, por exemplo Signum → Trâmite. |
| Matriz de falhas | `<m>/application/faults_test.go` + `faultrepo_test.go` | Para cada caso de uso, cada chamada ao repositório falha (`failAt`) ou envenena a transação logo depois (`poisonAt`, que faz a instrução seguinte falhar, seja do repositório, do outbox ou da auditoria). Nenhum erro é engolido e nada é gravado pela metade. |
| Repositório | `<m>/infrastructure/*_test.go` | `dbtest.Fail`, `ScanFail`, `RowsErr` e `Seq` (respostas roteirizadas) cobrem cada `if err != nil`: falha na contagem, na página, na leitura de linha e em `rows.Err`. |
| Handlers com o banco fora | `faults_test.go` | Respostas 500 sem vazar a mensagem interna. |
| Transação revertida | `platform/audit/worm_test.go` | Tabelas append-only (`audit_worm_exports`) são testadas numa transação que sempre sofre rollback. |

Utilitários:

- `database/dbtest`: fakes de `DBTX`, mais `Pool`/`User`/`Unidade` para as
  sementes;
- `storage/storagetest`: armazenamento em memória com falha por operação;
- `scripts/genfault.py`: gera o `faultRepo` a partir da interface
  `Repository` do domínio. É preciso regenerá-lo quando a porta muda.

### 9.3. Meta de cobertura no CI

`scripts/coverage-gate.sh` lê o perfil de
`go test -coverpkg=./internal/...`. O perfil soma vários binários de teste,
então vale a maior contagem de cada bloco. O script exige **100%** de
cobertura de instruções em cada `internal/modules/<plugin>` e em
`internal/platform/audit`, e quando falha lista as linhas descobertas.

| Módulo | Antes | Depois |
|---|---:|---:|
| tramite | 75,2% | 100% |
| signum | 77,8% | 100% |
| iam | 75,4% | 100% |
| files | 67,5% | 100% |
| mercurio | 78,1% | 100% |
| egress | 76,1% | 100% |
| calendar | 78,0% | 100% |
| catalog | 81,5% | 100% |
| blog | 89,0% | 100% |
| contact | 84,0% | 100% |
| directory | 80,0% | 100% |
| wiki | 84,8% | 100% |
| busca | 98,0% | 100% |
| example | 72,8% | 100% |
| auditoria (platform/audit) | 63,0% | 100% |

A cobertura total do backend (`internal/...`) passou de 70% para 84,7%.

## Consequências

- **Positivas.**
  - Cada regra de negócio descrita aqui tem um teste que falha se ela deixar
    de valer.
  - Uma regressão de cobertura num plug-in quebra o CI e aponta a linha.
  - Módulos gerados pelo scaffolding já nascem no padrão e com testes.
- **Custos.**
  - Toda linha nova em um plug-in precisa de teste, inclusive os
    `if err != nil`. A matriz de falhas e o `dbtest` tornam isso barato,
    mas não gratuito.
  - A suíte completa com `-race` leva cerca de 6 minutos.
- **Não coberto pela meta.** A meta vale para os plug-ins e para a
  auditoria. O restante de `internal/platform` e `internal/app` continua
  medido pelo total, sem meta por pacote.
