# Alterações planejadas do Atlas

Para: quem for continuar o Atlas (desenvolvimento e gestão documental).

Este documento registra o que **ficou combinado para depois**: decisões
provisórias, que dependem das entrevistas nos departamentos, e débitos
técnicos conhecidos. Cada item diz o que mudar, por quê, onde fica no código
e como saber que terminou. Ao concluir um item, risque-o (ou apague-o) no
mesmo commit da mudança.

Registrado em 2026-10-05. O estado do Atlas nessa data:
- **Procedimentos:** 3 publicados e 92 rascunhos (duas levas, ADR 028).
- **Siglas:** provisórias no cadastro de unidades.
- **Organograma da TTDD:** publicado (ADR 029).
- **Modelos de documento:** nenhum na biblioteca.

## Como usar

- **Depende das entrevistas (seção 1):** só mexa depois que o departamento
  confirmar. Anote a confirmação no registro da entrevista do procedimento
  (cartão **Validação**) e cite-a no commit.
- **Pode ser feito a qualquer momento:** seções 2, 3 e 4.

---

## 1. Depende das entrevistas

### 1.1 Confirmar as siglas provisórias das secretarias

- **O quê:** as siglas que pus nas sedes dos órgãos em 2026-10-04 são
  provisórias. São elas: SEMAD, SEMAGRI, SECULT, SEDEC, SEMEL, SEFIN, GAB,
  SEGEP, SEGOV, SEHAB, SEMMA, SEPPU, SEPLAN, PGM, SEREC, SESP, SETRAT e
  UCCI. Em uso antes disso: SECITI, SINFRA, IPPUR, PROCON, SEMED, SMS,
  SEMPRAS.
- **Por quê:** os avisos de nova versão chegam às unidades pela sigla da
  etapa (ADR 027). Uma sigla errada faz o aviso não chegar.
- **Onde:** `scripts/estrutura/ad-para-estrutura.mjs`, no mapa
  `SIGLA_ORGAO`. O cadastro em produção é gerado a partir dele.
- **Como:**
  1. Corrija o mapa.
  2. Rode `node scripts/estrutura/ad-para-estrutura.mjs docs/AD`
     (`docs/AD` fica fora do Git).
  3. Faça o commit do `deploy/estrutura/`.
  4. No servidor, rode `make estrutura-aplicar`.
  5. Se a sigla de um órgão mudar, atualize também as etapas: os geradores
     `deploy/atlas/gerar-rascunhos*.mjs` e, em produção, um script como
     `deploy/atlas/siglas-rascunhos.sql` (só para os não homologados). Os
     homologados mudam por nova versão.
- **Pronto quando:** a tabela da §7 do
  [Plano de validação](PLANO_DE_VALIDACAO.md) estiver sem "provisória" e
  nenhuma etapa de rascunho apontar para uma sigla que não exista no
  cadastro. Para conferir, use esta consulta:

  ```sql
  SELECT e.unidade_administrativa, count(*)
  FROM atlas_etapas e JOIN atlas_workflows w ON w.id = e.workflow_id
  WHERE w.situacao <> 'HOMOLOGADO' AND NOT EXISTS (
    SELECT 1 FROM unidades u WHERE u.ativo AND u.sigla <> '' AND upper(u.sigla) IN
      (upper(e.unidade_administrativa), upper(split_part(e.unidade_administrativa, '/', 1))))
  GROUP BY 1;
  ```

  Hoje só aparecem `SECRETARIA` ("a secretaria que pede") e `IMPRO`
  (item 1.10).

### 1.2 IPPUR aparece duas vezes

- **O quê:** o cadastro tem "Pesquisa e Planejamento Urbano" (SEPPU) e
  "IPPUR - SINFRA" (IPPUR). Tudo indica que são o mesmo instituto.
- **Como:** confirmado, junte as duas entradas.
  1. No gerador, faça um alias em `MESMO_QUE`.
  2. Antes de aplicar, verifique se alguém está lotado na entidade que vai
     sair (`user_scopes`, `ad_group_mappings`).

### 1.3 Compras: SEMAD ou SEFIN?

- **O quê:** os rascunhos põem as compras na Administração
  (`SEMAD/COMPRAS`). Os procedimentos já publicados (`ADM.LIC.001`,
  `ADM.DIR.002`) usam `SEFIN/COMPRAS`.
- **Como:** a entrevista da função 2.0.02 decide. Alinhe os rascunhos por
  SQL (como na 1.1) e os publicados por **nova versão**, homologada.

### 1.4 Receita × Finanças

- **O quê:** ao dividir a antiga SEFAZ, mandei tributos, cadastro
  imobiliário, dívida ativa e controle urbano para a **SEREC**, e
  contabilidade, tesouraria e patrimônio para a **SEFIN**. Patrimônio pode
  ser da Administração.
- **Como:** confirme nas entrevistas das funções 3.0.01 a 3.0.05 e corrija
  como na 1.1.

### 1.5 Setores como unidades do cadastro

- **O quê:** hoje o aviso de uma etapa `SEGEP/RH` vai para a **sede** da
  SEGEP, porque o setor "RH" não existe como unidade.
- **Por quê:** para avisar o setor que executa a etapa, ele precisa existir
  no cadastro com a mesma sigla.
- **Como:** crie os setores confirmados, pela tela (Configurações →
  Estrutura) ou no gerador. Use a sigla composta da etapa (`SEGEP/RH`) ou
  ajuste as etapas para a sigla do setor.

### 1.6 Cruzar o organograma da TTDD com o cadastro de unidades

- **O quê:** o organograma do Atlas (ADR 029) é **funcional** (TTDD). O
  cadastro de unidades é **administrativo** (AD, com as siglas). Os dois
  não batem: na TTDD, Administração, Gestão de Pessoas e Inovação são um só
  órgão (2.0); no cadastro, são entidades separadas (SEMAD, SEGEP, SECITI).
- **Proposta, a decidir depois das entrevistas:**
  - Uma tabela de correspondência, **função da TTDD → unidade do cadastro**
    (ex.: 2.0.05 a 2.0.08 → SEGEP; 2.0.04 → SECITI).
  - Com ela:
    - o organograma mostra "quem executa" cada função;
    - a página da secretaria passa a ser por unidade real;
    - os rascunhos ganham a sigla certa automaticamente.
  - **Implementação:** migração nova (`atlas_ttdd_unidades`), carga a
    partir de uma planilha conferida, uma coluna a mais no endpoint
    `/atlas/organograma` e um ADR.
- **Pronto quando:** cada função da TTDD tiver uma unidade responsável
  confirmada.

### 1.7 Corrigir os rascunhos com o resultado das entrevistas

- **O quê:** os 92 rascunhos são hipóteses (setores, prazos, documentos).
  É um processo de trabalho, sem código novo.
- **Como:** siga o [Plano de validação](PLANO_DE_VALIDACAO.md):
  1. Caderno da entrevista por função.
  2. Registro da entrevista.
  3. Nova versão (rascunho).
  4. Modelos ligados.
  5. Homologação.
- **Acompanhamento:** painel de Cobertura, página de cada secretaria e
  organograma.

### 1.8 Processos ainda sem procedimento

- **O quê:**
  - **Subfunções que ficaram de fora de propósito**, para as entrevistas
    decidirem:
    - 2.0.04.00 Pesquisa e Difusão Tecnológica (projetos PAPIRO, FECITI e
      WIFI-Social);
    - 7.0.01.00 Infraestrutura, série "Licenciamento" (não diz de quê);
    - 12.0.08.00 Gestão de Hospitais (internação, refeições);
    - 12.0.08.03 Nutrição (ticket de gás).
  - **Funções sem nenhum procedimento** (2026-10-05), que pedem entrevistas
    do zero:
    - 4.0.02 LGPD;
    - 7.0.01 Infraestrutura;
    - 10.0.04 Conselho Tutelar, 10.0.05 Casa Abrigo, 10.0.07 CREAS e
      10.0.08 Centro POP;
    - 12.0.03 SADT, 12.0.05 Saúde Mental, 12.0.09 SAMU e 12.0.12 Saúde do
      Trabalhador.
- **Como:** crie pela tela (o formulário já vem como rascunho) ou por uma
  nova leva no formato de `deploy/atlas/gerar-rascunhos-2.mjs` (simular
  antes de aplicar).
- **Pronto quando:** o organograma mostrar todas as funções com pelo menos
  um procedimento, ou a ausência estiver justificada no plano.

### 1.9 Educação sem órgão na TTDD

- **O quê:** a Educação tem muitas unidades no cadastro, mas nenhum órgão
  na TTDD. Os documentos escolares não têm onde se enquadrar.
- **Como:** depende da CCPAD (tabela própria ou acréscimo à TTDD). Depois,
  carregue pela tela de atualização da TTDD (ADR 022) e só então crie os
  fluxos.

### 1.10 IMPRO fora do cadastro

- **O quê:** as etapas de aposentadoria usam a sigla `IMPRO`
  (previdência), que não está no cadastro de unidades e por isso não
  recebe avisos.
- **Como:** decidir se o IMPRO entra como entidade (é autarquia) e criar a
  unidade com essa sigla.

### 1.11 Padronização dos documentos e biblioteca de modelos

- **O quê:** a biblioteca tem **0 modelos**. As entrevistas devem definir,
  para cada documento, o nome oficial, os campos mínimos e o modelo (a
  ficha de validação já tem essa tabela).
- **Como:** suba os modelos em Atlas → Modelos de documento e ligue-os às
  peças ou às séries (ADR 024). O painel de Cobertura lista as peças sem
  modelo.

---

## 2. Melhorias do produto (quando houver demanda)

- **Caderno por mais de uma função:** hoje o caderno é por secretaria ou
  por **uma** função. Se uma entrevista cobrir duas ou três funções, aceitar
  vários `funcao=` na URL. A API já filtra por prefixo; bastaria repetir a
  consulta ou aceitar uma lista.
- **Organograma por unidade real:** depende da 1.6.
- **Exportar o organograma em planilha:** além de imprimir, gerar um CSV
  (secretaria; função; subfunção; séries; procedimentos; lacuna), como a
  exportação da TTDD.

---

## 3. Débitos técnicos conhecidos

### 3.1 Teste de falhas do Atlas cresce com o banco de teste

- **Sintoma:** o `TestEveryRepositoryFailureIsPropagated` (pacote
  `atlas/application`) estourou os 10 minutos sob `-race` quando o banco de
  teste do CI local acumulou procedimentos de várias execuções.
- **Causa:** o item `ExportarProcedimentos` da matriz percorre **todos** os
  procedimentos ativos, uma vez por ponto de falha.
- **Contorno atual:** recriar o banco antes da suíte (`ci-backend.sh down`
  e depois `up`). O GitHub Actions já começa com o banco vazio.
- **Correção:** fazer o caso de exportação da matriz rodar com poucos
  procedimentos. Por exemplo, um filtro de teste ou um limite no
  `faultRepo` para o `List` da exportação.

### 3.2 Deploy falha quando o Keycloak demora a responder

- **Sintoma:** `make deploy` termina com erro porque o `backend-api` não
  fica saudável a tempo. O log mostra "OIDC discovery failed … context
  deadline exceeded". O contêiner reinicia sozinho e fica saudável, mas o
  script já desistiu e o frontend não sobe.
- **Contorno atual:** rodar `docker compose up -d` depois.
- **Correção:** tentar a descoberta OIDC de novo, com espera crescente,
  antes de desistir (em `platform/auth`). Ou dar mais tempo ao
  `scripts/deploy.sh` na espera pelo backend.

### 3.3 Arquivos com CRLF no backend

- **Sintoma:** `gofmt -l` aponta dezenas de arquivos só por terem quebra de
  linha CRLF na cópia sincronizada do Windows.
- **Correção:** um `.gitattributes` com `*.go text eol=lf` e uma
  renormalização (`git add --renormalize .`) num commit separado, sem
  outras mudanças.

---

## 4. Operação e segurança

- **Trocar as senhas:** root do servidor e admin local do Nexus. Ambas
  circularam em conversa. O admin local se redefine com
  `make prod-seed-admin`.
- **Backup automatizado:** agendar o backup do Postgres e do MinIO de
  produção, com retenção e teste de restauração. Hoje não há rotina.
- **Volumes antigos no servidor:** backups `projeto-nexus_*` e o volume da
  IA local (2,3 GB). Só remover com a confirmação do responsável. Não mexer
  nos volumes dos projetos Aurora e protocolo, nem no Keycloak.
