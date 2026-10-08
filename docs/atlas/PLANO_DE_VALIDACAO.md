# Plano de validação dos procedimentos do Atlas

Para: equipe de gestão documental e quem conduzirá as entrevistas nos
departamentos.

> O que ficou para depois (siglas a confirmar, cruzamento da TTDD com o
> cadastro de unidades, lacunas, débitos técnicos) está em
> [Alterações planejadas](PENDENCIAS.md).

## 1. Objetivo

Validar com cada departamento **como os processos tramitam de verdade** e
**padronizar os documentos** que eles produzem, antes de publicar os
procedimentos no Atlas (consulta interna, site público e assistente).

O ponto de partida são **92 rascunhos** gerados a partir da Tabela de
Temporalidade (TTDD): 28 dos processos mais comuns de cada secretaria
(primeira leva) e 64 das subfunções que ainda não tinham fluxo (segunda
leva). Eles são
**hipóteses**: etapas, setores, prazos e documentos têm de ser confirmados
ou corrigidos nas entrevistas.

## 2. Como funciona no sistema

Cada procedimento passa por três situações:

| Situação | O que significa | Visível para |
|---|---|---|
| **Rascunho** | Gerado, ainda não conversado com o departamento | Só a gestão do Atlas |
| **Em validação** | Entrevistas em andamento | Só a gestão do Atlas |
| **Homologado** | Validado e publicado | Todos (consulta, site público, assistente, busca) |

Regras:

- Rascunho e em validação **não aparecem** na consulta, no site público, na
  busca nem no assistente.
- O rascunho de uma **nova versão** de um procedimento já publicado não tira
  a versão atual do ar, e não avisa ninguém.
- Só a **homologação** publica. Ela substitui a versão anterior e avisa
  quem segue o fluxo e as unidades que participam dele.

Passo a passo:

1. **Importar os rascunhos** (já feito em produção). Em Atlas → Importar,
   envie `deploy/atlas/rascunhos-procedimentos.json` e
   `rascunhos-procedimentos-2.json` com **"Importar como rascunho"**
   marcado (já vem marcado), clique em **Simular**, confira e **Aplique**.
2. **Preparar a entrevista pela secretaria.** No início do Atlas, em
   **Procedimentos por secretaria**, abra a secretaria. A página mostra:
   - os procedimentos dela agrupados pela função da TTDD (em geral, o
     departamento);
   - a situação de cada um, com filtro;
   - os processos da TTDD que ainda não têm fluxo.

   O filtro **Função (departamento)** divide a secretaria em entrevistas
   menores. Exemplo: o RH ocupa as funções 2.0.05 a 2.0.08 (recursos
   humanos, folha, vida funcional, investidura); cada uma pode ser uma
   entrevista, com os seus procedimentos e as suas lacunas. Uma função sem nenhum fluxo também
   aparece, para conversar sobre as lacunas dela.

   O botão **Caderno da entrevista** imprime de uma vez a capa, o sumário e
   a ficha de cada procedimento, uma por página. O caderno respeita os
   filtros: com a função e "Rascunho", leva só o que falta validar naquele
   departamento. Assim o mapeamento pode ser feito por partes, em vários
   encontros.
3. **Registrar cada entrevista.** Na página do procedimento, cartão
   **Validação** → **Registrar entrevista**. Informe a data, o
   departamento, os participantes (pela **função**, não pelo nome), o que
   foi validado ou corrigido e as pendências. O primeiro registro muda o
   procedimento para **Em validação**.
4. **Corrigir o fluxo.** Corrija pelo botão **Nova versão**. Ele já vem
   marcado como rascunho, e a página da secretaria mostra sempre a versão
   mais recente. Para corrigir
   vários de uma vez, baixe o arquivo em Atlas → Importar → "Baixar os
   procedimentos em vigor", edite e importe de novo como rascunho.
5. **Ligar os modelos.** Ligue a cada documento o modelo padronizado: em
   **Detalhes** da peça, ou na biblioteca (Atlas → Modelos de documento).
6. **Homologar.** Com o fluxo e os documentos validados, use **Homologar**
   (pede confirmação).
7. **Acompanhar.** Em **Atlas → Cobertura** (rascunhos, em validação, peças
   sem modelo, séries sem fluxo).

As entrevistas ficam registradas pelo **código** do procedimento, então
continuam valendo para as versões seguintes. Elas não se apagam: uma
correção é um novo registro.

## 3. Roteiro da entrevista

Leve a **ficha de validação** impressa. Para a secretaria inteira, use o
**caderno da entrevista**, na página da secretaria. Para um procedimento só,
use o cartão Validação → "Ficha de validação para imprimir". A ficha traz o fluxo proposto item a item, com "confere /
corrigir", a tabela de padronização de cada documento e o fechamento com as
assinaturas. Percorra-a com quem executa o processo e depois registre a
entrevista no sistema.

### 3.1 O processo

- Quem pede? Por qual canal (balcão, sistema, e-mail)?
- Qual é a base legal (lei, decreto, instrução normativa)?
- O processo é **público** ou tem informação **restrita** (dados pessoais,
  saúde, situação fiscal)? Qual hipótese legal?
- A série da TTDD sugerida está certa? O documento final é guardado por
  quanto tempo?

### 3.2 Cada etapa

- **Qual setor** executa? Qual é a **sigla oficial** da unidade? (ver o
  item 5)
- **O que** o setor faz? Quem decide e quem só confere?
- **Prazo** real em dias. Existe prazo legal?
- **Condição para seguir** à próxima etapa.
- O que faz o processo **voltar** (diligência)? Para quem volta?
- A unidade mantém o processo aberto depois de enviar?

### 3.3 Cada documento (padronização)

| Pergunta | Por quê |
|---|---|
| Qual é o **nome oficial** do documento? | O mesmo nome em todos os fluxos permite reaproveitar o modelo |
| É **obrigatório** ou opcional? | Entra no checklist do processo |
| Nasce **no sistema** (nato-digital) ou é **papel digitalizado**? | Define a conferência da cópia |
| Quem **assina**: uma pessoa, várias (conjunta) ou em bloco? | Configura a assinatura no Signum |
| Existe **modelo**? Quem é o dono do modelo? | Vai para a biblioteca de modelos |
| Quais **campos** o documento precisa ter (dados mínimos)? | Padroniza o modelo |
| Há documento **duplicado ou desnecessário** que pode sair? | Simplifica o fluxo |

Peça os **modelos em uso**, em Word ou LibreOffice: eles serão
padronizados e cadastrados na biblioteca.

### 3.4 Fechamento

- Leia o fluxo corrigido para o entrevistado confirmar.
- Registre as pendências (documento que falta, decisão de outra
  secretaria).
- Combine quem valida a versão final (chefia do departamento).

## 4. Depois das entrevistas: a versão consolidada

- Corrija o rascunho com o que foi validado (botão **Nova versão** — já vem
  marcado "Salvar como rascunho") e ligue os modelos padronizados.
- **Homologar** publica a versão consolidada: ela fica disponível na consulta
  interna, no site público (`/procedimentos`), na busca, no assistente e no
  Trâmite (abertura de processo, checklist de peças), com versão, histórico e
  impressão. A versão anterior, se houver, sai do ar e quem segue o fluxo é
  avisado.
- Revisões futuras seguem o mesmo caminho: nova versão como rascunho →
  entrevista → homologar. O homologado nunca volta a rascunho.

## 5. Procedimentos novos

Se a entrevista revelar um processo que não está na lista, cadastre-o em
**Atlas → Novo procedimento** (vem marcado "Salvar como rascunho") ou inclua-o
no arquivo de importação. Ele entra no mesmo ciclo de validação.

O painel **Atlas → Cobertura → Processos da TTDD sem procedimento** lista as
subfunções da TTDD com séries de processo (requerimento, licença, certidão…) e
nenhum procedimento, nem rascunho — é a lista de candidatos, atualizada sozinha
à medida que os procedimentos são criados.

## 6. Checklist antes de homologar

- [ ] Todas as unidades das etapas foram entrevistadas (ou consultadas).
- [ ] As siglas das etapas são as **cadastradas** no sistema.
- [ ] Os nomes dos documentos seguem o padrão da biblioteca.
- [ ] Os documentos obrigatórios têm modelo ligado (ou a pendência está
      registrada).
- [ ] O nível de acesso e a hipótese legal foram confirmados.
- [ ] A série da TTDD foi confirmada pela gestão documental.
- [ ] Nenhuma pendência aberta nas entrevistas.

## 7. Siglas das unidades

Os avisos de nova versão chegam às unidades **pela sigla**: a sigla da
etapa (`SEGEP/RH`) ou o primeiro segmento dela (`SEGEP`) tem de ser a sigla
de uma unidade do cadastro.

O cadastro trazia "SEDE" como sigla da sede de 25 órgãos, e 21 órgãos não
tinham sigla. Agora cada órgão tem sigla e a sede leva a sigla do órgão. As
etapas dos rascunhos foram alinhadas às mesmas siglas
(`deploy/atlas/siglas-rascunhos.sql`).

**As siglas abaixo são provisórias.** Confirme-as nas entrevistas e
corrija-as em `scripts/estrutura/ad-para-estrutura.mjs` (`SIGLA_ORGAO`),
para que a próxima carga da estrutura não as desfaça.

| Órgão | Sigla | Órgão | Sigla |
|---|---|---|---|
| Administração | SEMAD | Meio Ambiente | SEMMA |
| Agricultura e Pecuária | SEMAGRI | Pesquisa e Planejamento Urbano | SEPPU |
| Ciência, Tecnologia e Inovação | SECITI | Planejamento | SEPLAN |
| Cultura | SECULT | PROCON | PROCON |
| Desenvolvimento Econômico | SEDEC | Procuradoria Geral | PGM |
| Esporte e Lazer | SEMEL | Receita | SEREC |
| Finanças | SEFIN | Segurança Pública | SESP |
| Gabinete Comunicação | GAB | SINFRA | SINFRA |
| Gestão de Pessoas | SEGEP | IPPUR - SINFRA | IPPUR |
| Governo | SEGOV | Transportes e Trânsito | SETRAT |
| Habitação | SEHAB | Controle Interno | UCCI |
| Educação | SEMED | Saúde | SMS |
| Assistência Social | SEMPRAS | | |

Pontos a esclarecer:

- "Pesquisa e Planejamento Urbano" e "IPPUR - SINFRA" parecem o mesmo órgão
  (o IPPUR). Se forem, uma das entradas sai do cadastro.
- Os rascunhos põem as **compras** na Administração (`SEMAD/COMPRAS`). Os
  procedimentos já publicados, porém, usam `SEFIN/COMPRAS`. A entrevista
  decide qual vale.
- Com a separação entre Receita e Finanças, tributos, cadastro imobiliário e
  dívida ativa ficaram em `SEREC`. Contabilidade, tesouraria e patrimônio
  ficaram em `SEFIN`.
- Os setores (a parte depois da barra) são descritivos. Para avisar um setor
  específico, ele precisa existir no cadastro como unidade com essa sigla.
  Até lá, o aviso vai para a sede do órgão.
- `IMPRO` (previdência) não está no cadastro de unidades. `SECRETARIA` quer
  dizer "a secretaria que pede" e não avisa ninguém.

## 8. Procedimentos prioritários (rascunhos)

### 8.1 Primeira leva

**28 procedimentos, 93 etapas e 154 documentos.** A coluna "Entrevistar"
indica quem conversar primeiro.

| Secretaria (TTDD) | Código | Procedimento | Série TTDD | Entrevistar |
|---|---|---|---|---|
| 2.0 Administração | ADM.CMP.004 | Adesão a Ata de Registro de Preços | 2.0.02.00.10 | Compras, Procuradoria |
| 2.0 Administração | ADM.CON.005 | Formalização de contrato e aditivos | 2.0.02.02.12 | Contratos, Procuradoria |
| 2.0 Administração | ADM.CON.006 | Fiscalização de contrato | 2.0.02.02.15 | Fiscais, Contabilidade |
| 2.0 Administração | ADM.CMP.007 | Inexigibilidade de licitação | 2.0.02.01.03 | Compras, Procuradoria |
| 2.0 Administração | ADM.FRO.008 | Infração de trânsito de veículo oficial | 2.0.01.02.15 | Frotas, Folha |
| 2.0 Administração | ADM.SIC.009 | Pedido de acesso à informação (LAI) | 2.0.03.02.00 | SIC |
| 2.0 Gestão de Pessoas | RH.FER.001 | Férias | 2.0.06.03.01 | RH, Folha |
| 2.0 Gestão de Pessoas | RH.LIC.002 | Licença-prêmio | 2.0.06.03.05 | RH, Procuradoria |
| 2.0 Gestão de Pessoas | RH.ADI.003 | Adicional de insalubridade | 2.0.06.02.09 | RH, DESOPEM |
| 2.0 Gestão de Pessoas | RH.APO.004 | Aposentadoria voluntária | 2.0.07.01.01 | RH, IMPRO |
| 2.0 Gestão de Pessoas | RH.PAD.005 | Processo Administrativo Disciplinar *(restrito)* | 2.0.05.02.04 | Comissão de PAD, Procuradoria |
| 2.0 Gestão de Pessoas | RH.EST.006 | Avaliação de estágio probatório | 2.0.08.01.02 | RH, comissão de avaliação |
| 2.0 Gestão de Pessoas | RH.CON.007 | Posse de aprovado em concurso | 2.0.08.00.14 | RH, DESOPEM |
| 3.0 Fazenda | FAZ.DIA.001 | Diárias (concessão e prestação de contas) | 3.0.01.00.05 | Contabilidade |
| 3.0 Fazenda | FAZ.DES.002 | Liquidação e pagamento de despesa | 3.0.01.00.02 | Contabilidade, Tesouraria |
| 3.0 Fazenda | FAZ.PAR.003 | Parcelamento de débitos | 3.0.02.00.03 | Dívida ativa, Atendimento |
| 3.0 Fazenda | FAZ.IPT.004 | Isenção de IPTU | 3.0.02.01.01 | Cadastro imobiliário |
| 3.0 Fazenda | FAZ.ALV.005 | Alvará de funcionamento | 3.0.03.00.04 | Licenciamento, Fiscalização |
| 3.0 Fazenda | FAZ.CND.006 | Certidão negativa de débitos | 3.0.02.00.00 | Atendimento, Dívida ativa |
| 4.0 Controle Interno | CTR.OUV.001 | Manifestação de ouvidoria *(restrito)* | 4.0.01.02.00 | Ouvidoria |
| 6.0 Habitação | HAB.ALV.001 | Alvará de licença para construção | 6.0.02.00.00 | Aprovação de projetos |
| 6.0 Habitação | HAB.HAB.002 | Habite-se | 6.0.02.00.08 | Fiscalização de obras |
| 6.0 Habitação | HAB.REU.003 | REURB-S *(restrito)* | 6.0.01.01.00 | Regularização fundiária |
| 8.0 Meio Ambiente | MAM.LIC.001 | Licenciamento ambiental | 8.0.01.01.00 | Licenciamento ambiental |
| 9.0 Desenvolvimento Econômico | DEC.USO.001 | Uso do solo para profissional liberal | 9.0.01.01.04 | Sala do Empreendedor |
| 10.0 Assistência Social | SAS.OSC.001 | Inscrição de OSC no CMAS | 10.0.02.00.00 | CMAS, Vigilância socioassistencial |
| 12.0 Saúde | SAU.VSA.001 | Licença sanitária | 12.0.10.02.03 | Vigilância sanitária |
| 12.0 Saúde | SAU.TFD.001 | Tratamento Fora do Domicílio *(restrito)* | 12.0.06.01.08 | TFD, Regulação |

### 8.2 Segunda leva

Cobre as subfunções da TTDD que tinham séries de processo e nenhum
procedimento. Como a primeira, vale como hipótese: em vários casos o
"setor" é um palpite a partir do nome da série e deve ser o primeiro ponto
da entrevista.

**64 procedimentos, 165 etapas e 212 documentos.**

| Secretaria (TTDD) | Código | Procedimento | Série TTDD | Entrevistar |
|---|---|---|---|---|
| 2.0 Administração | ADM.CEM.010 | Concessão de jazigo em cemitério municipal | 2.0.01.03.04 | Administração dos cemitérios, Arrecadação |
| 2.0 Administração | ADM.PRO.011 | Requerimento geral ao protocolo | 2.0.03.00.46 | Protocolo geral, Unidade competente |
| 2.0 Administração | ADM.DOC.012 | Eliminação de documentos | 2.0.03.01.05 | Unidade produtora, Comissão Permanente de Avaliação de Documentos |
| 2.0 Administração | TI.ACE.001 | Acesso a sistemas, rede e e-mail corporativo | 2.0.04.01.03 | Tecnologia da Informação |
| 2.0 Administração | TI.SUP.002 | Suporte técnico e manutenção de equipamento | 2.0.04.01.09 | Suporte técnico |
| 2.0 Gestão de Pessoas | RH.CAP.008 | Inscrição em curso de capacitação | 2.0.05.01.03 | Capacitação |
| 2.0 Gestão de Pessoas | RH.FOL.009 | Folha de pagamento complementar | 2.0.06.00.06 | Recursos Humanos, Contabilidade |
| 2.0 Gestão de Pessoas | RH.RES.010 | Rescisão e verbas rescisórias *(restrito)* | 2.0.06.00.04 | Recursos Humanos, Contabilidade |
| 2.0 Gestão de Pessoas | RH.VTR.011 | Vale-transporte (adesão e desistência) | 2.0.06.01.00 | Recursos Humanos, Folha de pagamento |
| 2.0 Gestão de Pessoas | RH.CNS.012 | Empréstimo consignado *(restrito)* | 2.0.06.01.16 | Recursos Humanos, Folha de pagamento |
| 2.0 Gestão de Pessoas | RH.PAL.013 | Desconto de pensão alimentícia *(restrito)* | 2.0.06.01.11 | Procuradoria, Folha de pagamento |
| 2.0 Gestão de Pessoas | RH.REE.014 | Reembolso de despesas e indenização ao servidor | 2.0.06.01.14 | Recursos Humanos, Folha de pagamento |
| 2.0 Gestão de Pessoas | RH.INS.015 | Declaração de tempo de contribuição ao INSS | 2.0.06.04.05 | Recursos Humanos |
| 2.0 Gestão de Pessoas | RH.LMA.016 | Licença-maternidade *(restrito)* | 2.0.07.00.16 | Recursos Humanos, Saúde ocupacional (DESOPEM) |
| 2.0 Gestão de Pessoas | RH.LPA.017 | Licença-paternidade | 2.0.07.00.17 | Recursos Humanos |
| 2.0 Gestão de Pessoas | RH.LNO.018 | Licença por falecimento (nojo) | 2.0.07.00.18 | Recursos Humanos |
| 2.0 Gestão de Pessoas | RH.PER.019 | Perícia médica para licença de saúde *(restrito)* | 2.0.07.02.30 | Saúde ocupacional (DESOPEM), Junta médica |
| 2.0 Gestão de Pessoas | RH.CAT.020 | Comunicação de Acidente de Trabalho (CAT) *(restrito)* | 2.0.07.02.10 | Saúde ocupacional (DESOPEM) |
| 2.0 Gestão de Pessoas | RH.REA.021 | Readaptação de função *(restrito)* | 2.0.07.02.28 | Junta médica, Recursos Humanos |
| 2.0 Gestão de Pessoas | RH.RCH.022 | Redução de carga horária para cuidar de pessoa com deficiência *(restrito)* | 2.0.07.02.32 | Recursos Humanos, Junta médica |
| 2.0 Gestão de Pessoas | RH.COM.023 | Nomeação e exoneração de cargo em comissão | 2.0.08.02.00 | Recursos Humanos |
| 2.0 Gestão de Pessoas | RH.TMP.024 | Contratação temporária por excepcional interesse público | 2.0.08.02.03 | Secretaria solicitante, Recursos Humanos |
| 2.0 Gestão de Pessoas | RH.ETG.025 | Contratação de estagiário | 2.0.08.02.05 | Unidade concedente, Gestão de estágios |
| 2.0 Gestão de Pessoas | RH.SEL.026 | Processo seletivo simplificado | 2.0.08.03.00 | Comissão do processo seletivo |
| 3.0 Fazenda | FAZ.FIS.007 | Ação fiscal tributária | 3.0.03.01.23 | Fiscalização, Auditor fiscal |
| 3.0 Fazenda | FAZ.ISS.008 | Recurso contra lançamento de ISSQN | 3.0.03.01.13 | Atendimento ao contribuinte, Auditor autuante |
| 3.0 Fazenda | FAZ.JUL.009 | Consulta tributária | 3.0.03.02.05 | Atendimento ao contribuinte, Julgamento tributário |
| 3.0 Fazenda | FAZ.FPM.010 | Recurso sobre o índice do FPM | 3.0.03.03.01 | Receita, Procuradoria |
| 3.0 Fazenda | FAZ.CAD.011 | Atualização do cadastro imobiliário (proprietário e endereço) | 3.0.04.00.05 | Atendimento ao contribuinte, Cadastro imobiliário |
| 3.0 Fazenda | FAZ.VVE.012 | Certidão de valor venal | 3.0.04.00.07 | Atendimento ao contribuinte, Cadastro imobiliário |
| 3.0 Fazenda | FAZ.CUS.013 | Certidão de uso do solo (controle urbano) | 3.0.04.01.06 | Atendimento, Controle urbano |
| 3.0 Fazenda | FAZ.ORC.014 | Suplementação orçamentária | 3.0.05.00.08 | Secretaria solicitante, Orçamento |
| 3.0 Fazenda | PAT.BAI.001 | Baixa de bem patrimonial | 3.0.05.01.01 | Unidade detentora, Comissão de patrimônio |
| 3.0 Fazenda | PAT.TRA.002 | Transferência de bem patrimonial | 3.0.05.01.04 | Unidade cedente, Patrimônio |
| 3.0 Fazenda | PAT.INC.003 | Incorporação de bem permanente | 3.0.05.01.15 | Patrimônio, Unidade recebedora |
| 4.0 Controle Interno | CTR.AUD.002 | Auditoria interna *(restrito)* | 4.0.01.01.00 | Controladoria, Equipe de auditoria |
| 6.0 Habitação | HAB.ITB.004 | Isenção de ITBI em programa habitacional *(restrito)* | 6.0.01.00.00 | Programas habitacionais, Julgamento tributário |
| 6.0 Habitação | HAB.MCM.005 | Titulação de beneficiário de programa habitacional *(restrito)* | 6.0.01.00.04 | Programas habitacionais, Procuradoria |
| 6.0 Habitação | HAB.RRU.006 | Regularização fundiária rural *(restrito)* | 6.0.01.02.00 | Regularização fundiária, Topografia e cartografia |
| 6.0 Habitação | HAB.DES.007 | Desmembramento e remembramento de imóvel | 6.0.02.01.10 | Atendimento, Averbação e cartografia |
| 6.0 Habitação | HAB.ALI.008 | Alinhamento de lote | 6.0.02.01.00 | Atendimento, Topografia |
| 8.0 Meio Ambiente e Agricultura | MAM.FIS.002 | Fiscalização ambiental | 8.0.01.00.00 | Fiscalização ambiental, Julgamento ambiental |
| 8.0 Meio Ambiente e Agricultura | MAM.ARV.003 | Remoção ou supressão de árvores | 8.0.01.02.03 | Atendimento, Análise técnica |
| 8.0 Meio Ambiente e Agricultura | MAM.REC.004 | Recurso ao Conselho de Meio Ambiente (CONSEMMA) | 8.0.01.03.02 | Secretaria executiva do CONSEMMA, Plenária do CONSEMMA |
| 8.0 Meio Ambiente e Agricultura | AGR.SIM.001 | Registro no Serviço de Inspeção Municipal (SIM) | 8.0.02.01.02 | Serviço de Inspeção Municipal, Médico veterinário |
| 8.0 Meio Ambiente e Agricultura | AGR.TRA.002 | Serviço de trator e implementos ao produtor rural | 8.0.02.02.00 | Atendimento ao produtor, Patrulha mecanizada |
| 8.0 Meio Ambiente e Agricultura | AGR.POC.003 | Perfuração de poço artesiano em área rural | 8.0.02.03.02 | Atendimento ao produtor, Extensão rural |
| 8.0 Meio Ambiente e Agricultura | AGR.EST.004 | Manutenção de estrada rural | 8.0.02.04.00 | Atendimento, Manutenção de estradas |
| 9.0 Desenvolvimento Econômico | DEC.INC.002 | Incentivo fiscal a empresa | 9.0.01.00.04 | Sala do Empreendedor, Conselho de desenvolvimento |
| 9.0 Desenvolvimento Econômico | DEC.IND.003 | Concessão de área em distrito industrial | 9.0.01.02.00 | Indústria, Conselho Diretor (CODIP) |
| 10.0 Assistência Social | SAS.CNV.002 | Convênio ou parceria na assistência social | 10.0.01.00.03 | Gestão do SUAS, Procuradoria |
| 10.0 Assistência Social | SAS.CHA.003 | Chamamento público de OSC | 10.0.03.00.06 | Gestão do SUAS, Comissão de seleção |
| 10.0 Assistência Social | SAS.BIL.004 | Bilhete único especial para pessoa com deficiência *(restrito)* | 10.0.06.00.40 | Atendimento, Avaliação social |
| 12.0 Saúde | SAU.EMD.002 | Emenda parlamentar para a saúde | 12.0.01.00.13 | Planejamento em saúde, Fundo Municipal de Saúde |
| 12.0 Saúde | SAU.CNE.003 | Cadastro de estabelecimento no SCNES | 12.0.01.01.09 | Cadastro CNES |
| 12.0 Saúde | SAU.EST.004 | Convênio de estágio nas unidades de saúde | 12.0.01.02.00 | Educação permanente, Procuradoria |
| 12.0 Saúde | SAU.AUD.005 | Auditoria de serviço de saúde *(restrito)* | 12.0.01.05.01 | Auditoria (DIACAUD), Equipe de auditoria |
| 12.0 Saúde | SAU.PCF.006 | Prestação de contas do Fundo Municipal de Saúde | 12.0.01.08.03 | Fundo Municipal de Saúde, Conselho Municipal de Saúde |
| 12.0 Saúde | SAU.DIU.007 | Inserção de DIU *(restrito)* | 12.0.02.00.81 | Unidade Básica de Saúde, Saúde da mulher |
| 12.0 Saúde | SAU.CNS.008 | Cartão Nacional de Saúde *(restrito)* | 12.0.04.00.01 | Recepção da unidade |
| 12.0 Saúde | SAU.PRO.009 | Cópia de prontuário *(restrito)* | 12.0.07.02.04 | Arquivo médico |
| 12.0 Saúde | SAU.OPM.010 | Órteses, próteses e meios auxiliares (OPM) *(restrito)* | 12.0.07.04.16 | Programa de OPM, Complexo regulador |
| 12.0 Saúde | SAU.PLA.011 | Inclusão ou exclusão de plantão médico | 12.0.08.05.08 | Coordenação da unidade, RH dos médicos |
| 12.0 Saúde | SAU.AGU.012 | Análise microbiológica de água | 12.0.11.03.01 | Laboratório de análise de água |

### 8.3 Já publicados

Devem ser revisados nas mesmas entrevistas:

- `ADM.LIC.001` (Pregão eletrônico);
- `ADM.DIR.002` (Dispensa por valor);
- `ADM.MAT.003` (Carga patrimonial).

## 9. Lacunas a tratar com a gestão documental

- **7.0 Infraestrutura** não tem rascunho: a TTDD só traz a subfunção
  "Gestão Administrativa". Definir com a secretaria quais processos
  tramitam (obras, manutenção, medições).
- **Educação** tem muitas unidades no cadastro, mas **não tem órgão na
  TTDD**. Os documentos escolares precisam de tabela ou enquadramento
  definido pela CCPAD antes de qualquer fluxo.
- Depois da segunda leva, o painel de Cobertura mostra **4 subfunções**
  sem procedimento, deixadas de propósito para as entrevistas:
  - **2.0.04.00 Pesquisa e Difusão Tecnológica** (projetos PAPIRO,
    FECITI, WIFI-Social): são projetos e eventos, não pedidos. Confirmar
    se tramitam como processo.
  - **7.0.01.00 Infraestrutura — "Licenciamento"**: a série não diz de
    quê (obras, veículos, equipamentos).
  - **12.0.08.00 Gestão de Hospitais** (cadastro de internação, refeições)
    e **12.0.08.03 Coordenação da Nutrição** (ticket de gás): rotinas
    internas do hospital. Confirmar se viram fluxo no Atlas.
- Uma subfunção coberta não quer dizer todas as séries cobertas: um fluxo
  pode atender várias séries parecidas (ex.: as licenças do servidor). A
  entrevista decide se uma série precisa de fluxo próprio.
- **Saúde (745 séries)** é a maior área: depois dos dois rascunhos,
  priorizar com a secretaria (regulação, farmácia, vigilância
  epidemiológica).

## 10. Proteção de dados nas entrevistas

- Registre os participantes pela **função** ("chefe do RH"), não pelo nome.
  O registro fica guardado e pode ser consultado pela gestão.
- Não anote dados de cidadãos ou servidores citados como exemplo.
- Para regerar os rascunhos (ex.: depois de ajustar o gerador), use
  `node deploy/atlas/gerar-rascunhos.mjs` (primeira leva) ou
  `node deploy/atlas/gerar-rascunhos-2.mjs` (segunda leva).
