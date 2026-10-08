// Segunda leva de RASCUNHOS (ADR 028): as subfunções da TTDD com séries de
// processo que ficaram sem procedimento na primeira leva (painel de
// Cobertura → "Processos da TTDD sem procedimento"). Mesmo formato e mesmas
// ressalvas de gerar-rascunhos.mjs: são hipóteses para as entrevistas.
//
// Uso: node deploy/atlas/gerar-rascunhos-2.mjs
//      → deploy/atlas/rascunhos-procedimentos-2.json

import { writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const FORMATO = { D: "NATO_DIGITAL", X: "EXTERNO_DIGITALIZADO" };
const ASSINATURA = { I: "INDIVIDUAL", C: "CONJUNTA_MULTINIVEL", B: "EM_BLOCO" };
const P = (codigo, titulo, objetivo, publico, ttdd, etapas, extra = {}) => ({ codigo, titulo, objetivo, publico, ttdd, etapas, ...extra });
const restrito = (hipotese) => ({ nivel: "RESTRITO", hipotese });
const SAUDE = restrito("LGPD, art. 11 (dado de saúde) e LAI, art. 31");
const PESSOAL = restrito("LAI, art. 31 (informação pessoal)");

// Atalhos de etapas comuns.
const rhRecebe = (a, docs, segue = "Requerimento instruído") => ({ u: "SEGEP/RH", s: "Recursos Humanos", a, p: 5, d: docs, segue });
const chefia = (a, docs, segue = "Chefia de acordo") => ({ u: "SECRETARIA", s: "Chefia imediata", a, p: 3, d: docs, segue });
const folha = (a = "Lançar na folha de pagamento.") => ({ u: "SEGEP/FOLHA", s: "Folha de pagamento", a, p: 30, d: [] });
const portaria = (doc = "Portaria") => ({ u: "SEGEP/RH", s: "Recursos Humanos", a: "Publicar o ato e registrar na pasta funcional.", p: 5, d: [[doc, 1, "D", "I"]] });
const pgm = (segue = "Parecer favorável", dil) => ({ u: "PGM", s: "Procuradoria", a: "Emitir parecer jurídico.", p: 10, d: [["Parecer jurídico", 1, "D", "I"]], segue, ...(dil ? { dil } : {}) });

const procedimentos = [
  // ------------------------------------------------------------- Administração (2.0)
  P("ADM.CEM.010", "Concessão de jazigo em cemitério municipal", "Conceder o direito de uso de jazigo em cemitério municipal, com o pagamento das taxas.", "Cidadãos", "2.0.01.03.04", [
    { u: "SEMAD/CEMITERIOS", s: "Administração dos cemitérios", a: "Receber o pedido e verificar a disponibilidade de jazigo.", p: 3, d: [["Requerimento de concessão de jazigo", 1, "X", "I"], ["Certidão de óbito (quando houver)", 0, "X"]], segue: "Jazigo disponível" },
    { u: "SEREC/ARRECADACAO", s: "Arrecadação", a: "Emitir a guia das taxas.", p: 2, d: [["Guia de pagamento", 1]], segue: "Taxa paga" },
    { u: "SEMAD/CEMITERIOS", s: "Administração dos cemitérios", a: "Lavrar o termo de concessão e registrar no livro do cemitério.", p: 5, d: [["Termo de concessão de uso", 1, "D", "C"]] },
  ]),
  P("ADM.PRO.011", "Requerimento geral ao protocolo", "Receber e encaminhar requerimento que não tem procedimento próprio, até a unidade competente responder.", "Cidadãos e servidores", "2.0.03.00.46", [
    { u: "SEMAD/PROTOCOLO", s: "Protocolo geral", a: "Registrar o requerimento, autuar e encaminhar à unidade competente.", p: 1, d: [["Requerimento", 1, "X", "I"], ["Documentos anexos", 0, "X"]], segue: "Processo autuado" },
    { u: "SECRETARIA", s: "Unidade competente", a: "Analisar e decidir o pedido.", p: 15, d: [["Despacho decisório", 1, "D", "I"]], segue: "Pedido decidido", dil: ["Unidade não competente", "Devolver ao protocolo para novo encaminhamento", 1] },
    { u: "SEMAD/PROTOCOLO", s: "Protocolo geral", a: "Dar ciência ao requerente e arquivar.", p: 3, d: [["Comunicação ao requerente", 1, "D", "I"]] },
  ]),
  P("ADM.DOC.012", "Eliminação de documentos", "Eliminar documentos com prazo de guarda cumprido e destinação \"eliminação\" na TTDD, com aprovação da CCPAD e publicação do edital de ciência.", "Unidades administrativas", "2.0.03.01.05", [
    { u: "SECRETARIA", s: "Unidade produtora", a: "Levantar os documentos com prazo cumprido e preencher a listagem de eliminação.", p: 30, d: [["Listagem de eliminação de documentos", 1, "D", "I"]], segue: "Listagem pronta" },
    { u: "SEMAD/CCPAD", s: "Comissão Permanente de Avaliação de Documentos", a: "Conferir a listagem contra a TTDD e aprovar.", p: 30, d: [["Parecer da CCPAD", 1, "D", "C"]], segue: "Listagem aprovada", dil: ["Divergência com a TTDD", "Corrigir a listagem", 1] },
    { u: "SEMAD/CCPAD", s: "Comissão Permanente de Avaliação de Documentos", a: "Publicar o edital de ciência de eliminação e aguardar o prazo.", p: 45, d: [["Edital de ciência de eliminação", 1, "X"]], segue: "Prazo do edital cumprido" },
    { u: "SECRETARIA", s: "Unidade produtora", a: "Eliminar os documentos e lavrar o termo de eliminação.", p: 15, d: [["Termo de eliminação de documentos", 1, "D", "C"]] },
  ]),

  // ------------------------------------------------------------- Tecnologia da informação (2.0.04.01)
  P("TI.ACE.001", "Acesso a sistemas, rede e e-mail corporativo", "Criar, alterar ou revogar o acesso do servidor aos sistemas municipais, à rede e ao e-mail corporativo.", "Servidores municipais", "2.0.04.01.03", [
    chefia("Solicitar o acesso e indicar o perfil necessário à função.", [["Requerimento de acesso", 1, "D", "I"], ["Termo de responsabilidade de uso", 1, "D", "I"]]),
    { u: "SEMAD/TI", s: "Tecnologia da Informação", a: "Criar a conta com o perfil mínimo necessário e registrar.", p: 3, d: [["Registro de concessão de acesso", 1, "D", "I"]], segue: "Acesso criado", dil: ["Perfil incompatível com a função", "Rever o perfil solicitado", 1] },
  ]),
  P("TI.SUP.002", "Suporte técnico e manutenção de equipamento", "Atender chamado de suporte ou manutenção de equipamento de informática.", "Servidores municipais", "2.0.04.01.09", [
    { u: "SECRETARIA", s: "Unidade solicitante", a: "Abrir o chamado descrevendo o problema e o patrimônio do equipamento.", p: 1, d: [["Ordem de serviço / chamado", 1, "D", "I"]], segue: "Chamado aberto" },
    { u: "SEMAD/TI", s: "Suporte técnico", a: "Diagnosticar e resolver ou encaminhar à manutenção externa.", p: 5, d: [["Laudo técnico", 0, "D", "I"]], segue: "Atendimento concluído" },
    { u: "SECRETARIA", s: "Unidade solicitante", a: "Confirmar a solução e encerrar o chamado.", p: 2, d: [] },
  ]),

  // ------------------------------------------------------------- Gestão de pessoas
  P("RH.CAP.008", "Inscrição em curso de capacitação", "Inscrever o servidor em curso promovido pela Prefeitura ou por outro órgão, com autorização da chefia.", "Servidores municipais", "2.0.05.01.03", [
    chefia("Autorizar a participação e a dispensa do expediente.", [["Ficha de inscrição", 1, "D", "I"]]),
    { u: "SEGEP/CAPACITACAO", s: "Capacitação", a: "Confirmar a vaga, registrar a frequência e emitir o certificado.", p: 10, d: [["Lista de presença", 1, "X"], ["Certificado", 0, "D", "I"]] },
  ]),
  P("RH.FOL.009", "Folha de pagamento complementar", "Pagar valores devidos fora da folha normal (diferenças, verbas atrasadas).", "Servidores municipais", "2.0.06.00.06", [
    rhRecebe("Apurar os valores devidos e montar o demonstrativo.", [["Demonstrativo de cálculo", 1, "D", "I"]], "Valores apurados"),
    { u: "SEFIN/CONTABILIDADE", s: "Contabilidade", a: "Verificar a dotação e empenhar.", p: 5, d: [["Nota de empenho", 1]], segue: "Despesa empenhada" },
    folha("Processar a folha complementar e o pagamento."),
  ]),
  P("RH.RES.010", "Rescisão e verbas rescisórias", "Calcular e pagar as verbas devidas na saída do servidor (exoneração, término de contrato, falecimento).", "Ex-servidores e dependentes", "2.0.06.00.04", [
    rhRecebe("Receber o ato de desligamento e apurar férias, 13º e demais verbas.", [["Ato de desligamento", 1, "X"], ["Termo de rescisão", 1, "D", "C"]], "Verbas apuradas"),
    { u: "SEFIN/CONTABILIDADE", s: "Contabilidade", a: "Empenhar e pagar as verbas rescisórias.", p: 10, d: [["Nota de empenho", 1], ["Ordem de pagamento", 1]] },
  ], PESSOAL),
  P("RH.VTR.011", "Vale-transporte (adesão e desistência)", "Conceder ou cancelar o vale-transporte, com desconto na folha conforme a lei.", "Servidores municipais", "2.0.06.01.00", [
    rhRecebe("Receber a declaração com o itinerário e os meios de transporte.", [["Requerimento de vale-transporte", 1, "D", "I"], ["Declaração de itinerário", 1, "D", "I"]]),
    folha("Lançar o benefício e o desconto correspondente."),
  ]),
  P("RH.CNS.012", "Empréstimo consignado", "Averbar o empréstimo consignado do servidor dentro da margem legal, com banco credenciado.", "Servidores municipais", "2.0.06.01.16", [
    rhRecebe("Emitir a declaração de margem consignável.", [["Requerimento de adesão ao consignado", 1, "D", "I"], ["Declaração de margem consignável", 1, "D", "I"]], "Margem disponível"),
    folha("Averbar o contrato e lançar o desconto em folha."),
  ], PESSOAL),
  P("RH.PAL.013", "Desconto de pensão alimentícia", "Cumprir a determinação judicial de desconto de pensão alimentícia na folha do servidor.", "Servidores municipais", "2.0.06.01.11", [
    { u: "PGM", s: "Procuradoria", a: "Receber o ofício judicial e orientar o cumprimento.", p: 3, d: [["Ofício judicial", 1, "X"]], segue: "Ordem conferida" },
    folha("Lançar o desconto e repassar ao beneficiário conforme a decisão."),
  ], restrito("LAI, art. 31 e segredo de justiça (CPC, art. 189)")),
  P("RH.REE.014", "Reembolso de despesas e indenização ao servidor", "Ressarcir o servidor por despesa feita a serviço ou por indenização prevista no estatuto.", "Servidores municipais", "2.0.06.01.14", [
    chefia("Confirmar que a despesa foi feita a serviço.", [["Requerimento de reembolso", 1, "D", "I"], ["Comprovantes de despesa", 1, "X"]]),
    rhRecebe("Conferir a previsão legal e o valor.", [["Informação do RH", 1, "D", "I"]], "Reembolso devido"),
    folha("Pagar o reembolso na folha ou por ordem de pagamento."),
  ]),
  P("RH.INS.015", "Declaração de tempo de contribuição ao INSS", "Emitir a declaração do tempo de contribuição ao Regime Geral (servidor celetista, contratado ou comissionado).", "Servidores e ex-servidores", "2.0.06.04.05", [
    rhRecebe("Receber o pedido e levantar as contribuições do período.", [["Requerimento de tempo de contribuição", 1, "X", "I"]], "Período levantado"),
    { u: "SEGEP/RH", s: "Recursos Humanos", a: "Emitir a declaração.", p: 10, d: [["Declaração de tempo de contribuição", 1, "D", "I"]] },
  ]),
  P("RH.LMA.016", "Licença-maternidade", "Conceder a licença à gestante ou adotante, com a prorrogação prevista na legislação municipal.", "Servidoras municipais", "2.0.07.00.16", [
    rhRecebe("Receber o atestado ou a certidão de nascimento ou o termo de adoção.", [["Requerimento de licença-maternidade", 1, "D", "I"], ["Certidão de nascimento ou termo de guarda", 1, "X"]]),
    { u: "SEGEP/DESOPEM", s: "Saúde ocupacional (DESOPEM)", a: "Homologar a licença e, se pedida, a prorrogação.", p: 5, d: [["Requerimento de prorrogação", 0, "D", "I"]], segue: "Licença homologada" },
    portaria("Portaria de licença-maternidade"),
  ], PESSOAL),
  P("RH.LPA.017", "Licença-paternidade", "Conceder a licença-paternidade ao servidor pelo nascimento ou adoção.", "Servidores municipais", "2.0.07.00.17", [
    rhRecebe("Receber a certidão de nascimento ou o termo de adoção.", [["Requerimento de licença-paternidade", 1, "D", "I"], ["Certidão de nascimento ou termo de guarda", 1, "X"]]),
    portaria("Registro da licença-paternidade"),
  ]),
  P("RH.LNO.018", "Licença por falecimento (nojo)", "Conceder a licença pelo falecimento de familiar previsto no estatuto.", "Servidores municipais", "2.0.07.00.18", [
    rhRecebe("Receber a certidão de óbito e o vínculo de parentesco.", [["Requerimento de licença", 1, "D", "I"], ["Certidão de óbito", 1, "X"]]),
    portaria("Registro da licença"),
  ]),
  P("RH.PER.019", "Perícia médica para licença de saúde", "Avaliar o servidor em perícia médica e conceder a licença para tratamento de saúde.", "Servidores municipais", "2.0.07.02.30", [
    { u: "SEGEP/DESOPEM", s: "Saúde ocupacional (DESOPEM)", a: "Agendar a perícia (no local, domiciliar ou hospitalar) a partir do atestado.", p: 3, d: [["Requerimento de perícia médica", 1, "D", "I"], ["Atestado médico", 1, "X"]], segue: "Perícia agendada" },
    { u: "SEGEP/DESOPEM", s: "Junta médica", a: "Realizar a perícia e emitir o laudo com o prazo da licença.", p: 5, d: [["Laudo pericial", 1, "D", "C"]], segue: "Laudo emitido" },
    portaria("Portaria de licença para tratamento de saúde"),
  ], SAUDE),
  P("RH.CAT.020", "Comunicação de Acidente de Trabalho (CAT)", "Registrar o acidente de trabalho do servidor e encaminhar a avaliação e os benefícios.", "Servidores municipais", "2.0.07.02.10", [
    chefia("Comunicar o acidente em até 1 dia útil, com testemunhas.", [["Relato do acidente", 1, "D", "I"]], "Acidente comunicado"),
    { u: "SEGEP/DESOPEM", s: "Saúde ocupacional (DESOPEM)", a: "Emitir a CAT e avaliar o afastamento.", p: 5, d: [["CAT – Comunicação de Acidente de Trabalho", 1, "D", "I"], ["Laudo médico", 1, "X"]] },
  ], SAUDE),
  P("RH.REA.021", "Readaptação de função", "Readaptar o servidor com limitação de saúde a atribuições compatíveis, após avaliação da junta médica.", "Servidores municipais", "2.0.07.02.28", [
    { u: "SEGEP/DESOPEM", s: "Junta médica", a: "Avaliar a limitação e indicar as restrições.", p: 15, d: [["Laudo de readaptação", 1, "D", "C"]], segue: "Restrições definidas" },
    rhRecebe("Indicar a função compatível com a secretaria de lotação.", [["Proposta de readaptação", 1, "D", "I"]], "Função definida"),
    portaria("Portaria de readaptação"),
  ], SAUDE),
  P("RH.RCH.022", "Redução de carga horária para cuidar de pessoa com deficiência", "Reduzir a jornada do servidor responsável por pessoa com deficiência, sem redução de vencimento (Lei 8.563/2015).", "Servidores municipais", "2.0.07.02.32", [
    rhRecebe("Receber o pedido com o laudo da pessoa com deficiência e a comprovação da dependência.", [["Requerimento de redução de carga horária", 1, "D", "I"], ["Laudo médico da pessoa com deficiência", 1, "X"]]),
    { u: "SEGEP/DESOPEM", s: "Junta médica", a: "Avaliar o laudo e opinar.", p: 15, d: [["Parecer da junta médica", 1, "D", "C"]], segue: "Parecer favorável" },
    portaria("Portaria de redução de carga horária"),
  ], SAUDE),
  P("RH.COM.023", "Nomeação e exoneração de cargo em comissão", "Nomear ou exonerar servidor de cargo em comissão, com verificação de vedações (nepotismo, ficha limpa).", "Secretarias municipais", "2.0.08.02.00", [
    { u: "SECRETARIA", s: "Secretário", a: "Indicar o nome para o cargo vago.", p: 3, d: [["Requerimento de nomeação/exoneração", 1, "D", "I"]], segue: "Indicação feita" },
    rhRecebe("Conferir a vaga, os documentos e as declarações de vedação.", [["Declaração de não parentesco", 1, "X", "I"], ["Certidões negativas", 1, "X"]], "Requisitos atendidos"),
    { u: "GAB", s: "Prefeito", a: "Assinar e publicar o decreto ou a portaria.", p: 5, d: [["Ato de nomeação/exoneração", 1, "D", "I"]] },
  ]),
  P("RH.TMP.024", "Contratação temporária por excepcional interesse público", "Contratar pessoal por tempo determinado, nos casos previstos em lei, a partir de processo seletivo ou justificativa.", "Secretarias municipais", "2.0.08.02.03", [
    { u: "SECRETARIA", s: "Secretaria solicitante", a: "Justificar a necessidade temporária e indicar o cadastro de reserva.", p: 5, d: [["Requerimento de contratação temporária", 1, "D", "I"], ["Justificativa de excepcional interesse público", 1, "D", "I"]], segue: "Necessidade justificada" },
    rhRecebe("Conferir a dotação, a ordem de classificação e os documentos do contratado.", [["Documentos do contratado", 1, "X"]], "Contratação viável"),
    { u: "SEGEP/RH", s: "Recursos Humanos", a: "Formalizar o contrato e cadastrar na folha.", p: 5, d: [["Contrato temporário", 1, "D", "C"]] },
  ]),
  P("RH.ETG.025", "Contratação de estagiário", "Contratar estagiário por meio de agente de integração e termo de compromisso com a instituição de ensino.", "Secretarias e estudantes", "2.0.08.02.05", [
    { u: "SECRETARIA", s: "Unidade concedente", a: "Solicitar a vaga e indicar o supervisor.", p: 3, d: [["Requerimento de contratação de estagiário", 1, "D", "I"]], segue: "Vaga solicitada" },
    { u: "SEGEP/CAPACITACAO", s: "Gestão de estágios", a: "Selecionar com o agente de integração e firmar o termo de compromisso.", p: 15, d: [["Termo de compromisso de estágio", 1, "D", "C"], ["Plano de atividades", 1, "D", "C"]] },
  ]),
  P("RH.SEL.026", "Processo seletivo simplificado", "Planejar e realizar processo seletivo para contratação temporária, com edital, recursos e homologação.", "Candidatos e secretarias", "2.0.08.03.00", [
    { u: "SEGEP/RH", s: "Comissão do processo seletivo", a: "Planejar, publicar o edital e receber as inscrições.", p: 30, d: [["Edital do processo seletivo", 1, "D", "C"], ["Lista de inscritos", 1]], segue: "Inscrições encerradas" },
    { u: "SEGEP/RH", s: "Comissão do processo seletivo", a: "Avaliar, publicar o resultado e julgar os recursos.", p: 30, d: [["Resultado parcial", 1, "D", "C"], ["Decisão dos recursos", 0, "D", "C"]], segue: "Recursos julgados" },
    { u: "GAB", s: "Prefeito", a: "Homologar o resultado final.", p: 5, d: [["Homologação do resultado", 1, "D", "I"]] },
  ]),

  // ------------------------------------------------------------- Fazenda (3.0)
  P("FAZ.FIS.007", "Ação fiscal tributária", "Fiscalizar o contribuinte, lavrar o auto de infração quando houver débito e encerrar a ação fiscal.", "Contribuintes", "3.0.03.01.23", [
    { u: "SEREC/FISCALIZACAO", s: "Fiscalização", a: "Emitir a ordem de serviço e o termo de início de fiscalização.", p: 5, d: [["Ordem de serviço", 1, "D", "I"], ["Termo de início de fiscalização", 1, "D", "I"]], segue: "Fiscalização iniciada" },
    { u: "SEREC/FISCALIZACAO", s: "Auditor fiscal", a: "Levantar os documentos e apurar o débito.", p: 60, d: [["Levantamento fiscal", 1, "D", "I"], ["Auto de infração e multa", 0, "D", "I"]], segue: "Apuração concluída" },
    { u: "SEREC/FISCALIZACAO", s: "Fiscalização", a: "Notificar o contribuinte e lavrar o termo de conclusão.", p: 5, d: [["Termo de conclusão de fiscalização", 1, "D", "I"]] },
  ]),
  P("FAZ.ISS.008", "Recurso contra lançamento de ISSQN", "Julgar a impugnação ou o recurso do contribuinte contra lançamento ou auto de infração de ISSQN.", "Contribuintes", "3.0.03.01.13", [
    { u: "SEREC/ATENDIMENTO", s: "Atendimento ao contribuinte", a: "Receber o recurso no prazo e juntar o lançamento.", p: 2, d: [["Recurso do contribuinte", 1, "X", "I"]], segue: "Recurso tempestivo" },
    { u: "SEREC/FISCALIZACAO", s: "Auditor autuante", a: "Contestar ou rever o lançamento.", p: 15, d: [["Informação fiscal", 1, "D", "I"]], segue: "Informação prestada" },
    { u: "SEREC/TRIBUTARIO", s: "Julgamento tributário", a: "Julgar e notificar o contribuinte.", p: 30, d: [["Decisão de primeira instância", 1, "D", "I"]] },
  ]),
  P("FAZ.JUL.009", "Consulta tributária", "Responder formalmente à consulta do contribuinte sobre a aplicação da legislação tributária municipal.", "Contribuintes", "3.0.03.02.05", [
    { u: "SEREC/ATENDIMENTO", s: "Atendimento ao contribuinte", a: "Receber a consulta com a descrição do fato.", p: 2, d: [["Petição de consulta", 1, "X", "I"]], segue: "Consulta recebida" },
    { u: "SEREC/TRIBUTARIO", s: "Julgamento tributário", a: "Analisar e emitir a resposta à consulta.", p: 30, d: [["Resposta à consulta", 1, "D", "I"]] },
  ]),
  P("FAZ.FPM.010", "Recurso sobre o índice do FPM", "Solicitar à Procuradoria a interposição de recurso contra o índice ou a distribuição do Fundo de Participação dos Municípios.", "Gestão fazendária", "3.0.03.03.01", [
    { u: "SEREC/RECEITA", s: "Receita", a: "Analisar o índice publicado e levantar os dados divergentes.", p: 10, d: [["Nota técnica do índice", 1, "D", "I"]], segue: "Divergência confirmada" },
    { u: "PGM", s: "Procuradoria", a: "Interpor o recurso no órgão competente.", p: 10, d: [["Recurso administrativo", 1, "D", "I"]] },
  ]),
  P("FAZ.CAD.011", "Atualização do cadastro imobiliário (proprietário e endereço)", "Atualizar o nome do proprietário ou o endereço de correspondência no cadastro imobiliário.", "Contribuintes", "3.0.04.00.05", [
    { u: "SEREC/ATENDIMENTO", s: "Atendimento ao contribuinte", a: "Receber o pedido com o documento de propriedade.", p: 2, d: [["Requerimento de atualização cadastral", 1, "X", "I"], ["Matrícula ou escritura", 1, "X"]], segue: "Pedido instruído" },
    { u: "SEREC/CADASTRO", s: "Cadastro imobiliário", a: "Conferir e atualizar o cadastro.", p: 10, d: [["Boletim de cadastro imobiliário", 1, "D", "I"]] },
  ]),
  P("FAZ.VVE.012", "Certidão de valor venal", "Emitir certidão do valor venal do imóvel para fins de ITBI, financiamento ou inventário.", "Contribuintes", "3.0.04.00.07", [
    { u: "SEREC/ATENDIMENTO", s: "Atendimento ao contribuinte", a: "Receber o pedido.", p: 1, d: [["Requerimento de certidão", 1, "X", "I"]], segue: "Pedido recebido" },
    { u: "SEREC/CADASTRO", s: "Cadastro imobiliário", a: "Emitir a certidão.", p: 5, d: [["Certidão de valor venal", 1, "D", "I"]] },
  ]),
  P("FAZ.CUS.013", "Certidão de uso do solo (controle urbano)", "Emitir certidão de uso do solo do imóvel conforme o zoneamento.", "Proprietários e empresas", "3.0.04.01.06", [
    { u: "SEREC/ATENDIMENTO", s: "Atendimento", a: "Receber o pedido com a localização do imóvel e a atividade.", p: 1, d: [["Requerimento de certidão de uso do solo", 1, "X", "I"]], segue: "Pedido recebido" },
    { u: "SEREC/CONTROLE_URBANO", s: "Controle urbano", a: "Verificar o zoneamento e emitir a certidão.", p: 10, d: [["Certidão de uso do solo", 1, "D", "I"]] },
  ]),
  P("FAZ.ORC.014", "Suplementação orçamentária", "Abrir crédito suplementar ou especial para despesa sem dotação suficiente.", "Secretarias municipais", "3.0.05.00.08", [
    { u: "SECRETARIA", s: "Secretaria solicitante", a: "Justificar a necessidade e indicar a fonte de recurso.", p: 3, d: [["Solicitação de suplementação", 1, "D", "I"]], segue: "Pedido justificado" },
    { u: "SEPLAN/ORCAMENTO", s: "Orçamento", a: "Verificar a disponibilidade e minutar o decreto.", p: 5, d: [["Minuta de decreto de crédito", 1, "D", "I"]], segue: "Crédito viável", dil: ["Fonte insuficiente", "Indicar outra fonte de recurso", 1] },
    { u: "GAB", s: "Prefeito", a: "Assinar e publicar o decreto.", p: 3, d: [["Decreto de abertura de crédito", 1, "D", "I"]] },
  ]),
  P("PAT.BAI.001", "Baixa de bem patrimonial", "Dar baixa de bem móvel inservível, extraviado ou alienado, com laudo e autorização.", "Secretarias municipais", "3.0.05.01.01", [
    { u: "SECRETARIA", s: "Unidade detentora", a: "Solicitar a baixa com o número de patrimônio e o motivo.", p: 3, d: [["Solicitação de baixa", 1, "D", "I"], ["Boletim de ocorrência (extravio)", 0, "X"]], segue: "Pedido instruído" },
    { u: "SEFIN/PATRIMONIO", s: "Comissão de patrimônio", a: "Vistoriar e emitir o laudo de inservibilidade.", p: 15, d: [["Laudo de avaliação", 1, "D", "C"]], segue: "Baixa recomendada" },
    { u: "SEFIN/PATRIMONIO", s: "Patrimônio", a: "Registrar a baixa e destinar o bem.", p: 5, d: [["Termo de baixa", 1, "D", "I"]] },
  ]),
  P("PAT.TRA.002", "Transferência de bem patrimonial", "Transferir a carga de bem móvel entre unidades, atualizando o responsável.", "Secretarias municipais", "3.0.05.01.04", [
    { u: "SECRETARIA", s: "Unidade cedente", a: "Solicitar a transferência indicando a unidade recebedora.", p: 2, d: [["Solicitação de transferência", 1, "D", "I"]], segue: "Pedido feito" },
    { u: "SEFIN/PATRIMONIO", s: "Patrimônio", a: "Emitir o termo de transferência de responsabilidade.", p: 5, d: [["Termo de transferência", 1, "D", "C"]] },
  ]),
  P("PAT.INC.003", "Incorporação de bem permanente", "Incorporar ao patrimônio o bem adquirido, doado ou cedido, com plaqueta e termo de responsabilidade.", "Secretarias municipais", "3.0.05.01.15", [
    { u: "SEFIN/PATRIMONIO", s: "Patrimônio", a: "Conferir a nota fiscal ou o termo de doação e emplaquetar.", p: 5, d: [["Nota fiscal ou termo de doação", 1, "X"], ["Ficha de cadastro do bem", 1, "D", "I"]], segue: "Bem cadastrado" },
    { u: "SECRETARIA", s: "Unidade recebedora", a: "Assinar o termo de responsabilidade.", p: 3, d: [["Termo de responsabilidade", 1, "D", "I"]] },
  ]),

  // ------------------------------------------------------------- Controle interno (4.0)
  P("CTR.AUD.002", "Auditoria interna", "Planejar e executar auditoria em unidade ou processo, com relatório e plano de providências.", "Unidades administrativas", "4.0.01.01.00", [
    { u: "UCCI", s: "Controladoria", a: "Planejar a auditoria e comunicar a unidade auditada.", p: 10, d: [["Plano de auditoria", 1, "D", "I"], ["Ofício de comunicação", 1, "D", "I"]], segue: "Auditoria iniciada" },
    { u: "UCCI", s: "Equipe de auditoria", a: "Executar os testes e emitir o relatório preliminar.", p: 45, d: [["Relatório preliminar", 1, "D", "C"]], segue: "Relatório emitido" },
    { u: "SECRETARIA", s: "Unidade auditada", a: "Manifestar-se sobre os achados.", p: 15, d: [["Manifestação da unidade", 1, "D", "I"]], segue: "Manifestação recebida" },
    { u: "UCCI", s: "Controladoria", a: "Emitir o relatório final e o plano de providências.", p: 15, d: [["Relatório final de auditoria", 1, "D", "C"]] },
  ], restrito("LAI, art. 7º, §3º (documento preparatório até a conclusão)")),

  // ------------------------------------------------------------- Habitação (6.0)
  P("HAB.ITB.004", "Isenção de ITBI em programa habitacional", "Reconhecer a isenção de ITBI para beneficiário de programa habitacional de interesse social.", "Beneficiários de programas habitacionais", "6.0.01.00.00", [
    { u: "SEHAB/PROGRAMAS", s: "Programas habitacionais", a: "Confirmar o enquadramento do beneficiário no programa.", p: 5, d: [["Requerimento de isenção de ITBI", 1, "X", "I"], ["Declaração de enquadramento", 1, "D", "I"]], segue: "Beneficiário enquadrado" },
    { u: "SEREC/TRIBUTARIO", s: "Julgamento tributário", a: "Reconhecer a isenção e emitir a guia zerada.", p: 10, d: [["Decisão de isenção", 1, "D", "I"]] },
  ], PESSOAL),
  P("HAB.MCM.005", "Titulação de beneficiário de programa habitacional", "Emitir o título de propriedade ao beneficiário de programa habitacional (ex.: Minha Casa Minha Vida).", "Beneficiários de programas habitacionais", "6.0.01.00.04", [
    { u: "SEHAB/PROGRAMAS", s: "Programas habitacionais", a: "Conferir a ocupação e os requisitos do beneficiário.", p: 15, d: [["Cadastro do beneficiário", 1, "D", "I"], ["Documentos pessoais", 1, "X"]], segue: "Requisitos atendidos" },
    pgm(),
    { u: "SEHAB/PROGRAMAS", s: "Programas habitacionais", a: "Emitir o título e encaminhar ao cartório.", p: 10, d: [["Título de propriedade", 1, "D", "C"]] },
  ], PESSOAL),
  P("HAB.RRU.006", "Regularização fundiária rural", "Regularizar a ocupação rural individual de interesse social (REURB-S) ou específico (REURB-E).", "Ocupantes de áreas rurais", "6.0.01.02.00", [
    { u: "SEHAB/REURB", s: "Regularização fundiária", a: "Cadastrar o ocupante e enquadrar a modalidade.", p: 30, d: [["Requerimento de regularização", 1, "X", "I"], ["Cadastro socioeconômico", 1, "D", "I"]], segue: "Modalidade enquadrada" },
    { u: "SEHAB/TOPOGRAFIA", s: "Topografia e cartografia", a: "Levantar a área e elaborar o memorial descritivo.", p: 60, d: [["Levantamento topográfico", 1, "X"], ["Memorial descritivo", 1, "D", "C"]], segue: "Levantamento concluído" },
    { u: "SEHAB/REURB", s: "Regularização fundiária", a: "Emitir a certidão de regularização e encaminhar ao cartório.", p: 15, d: [["Certidão de Regularização Fundiária (CRF)", 1, "D", "C"]] },
  ], PESSOAL),
  P("HAB.DES.007", "Desmembramento e remembramento de imóvel", "Aprovar a divisão ou a unificação de lotes urbanos conforme a lei de parcelamento do solo.", "Proprietários", "6.0.02.01.10", [
    { u: "SEHAB/PROTOCOLO", s: "Atendimento", a: "Receber o pedido com o projeto e as matrículas.", p: 3, d: [["Requerimento de desmembramento/remembramento", 1, "X", "I"], ["Projeto e memorial descritivo", 1, "X"], ["Matrículas atualizadas", 1, "X"]], segue: "Pedido instruído" },
    { u: "SEHAB/CARTOGRAFIA", s: "Averbação e cartografia", a: "Analisar o projeto e as dimensões mínimas dos lotes.", p: 20, d: [["Parecer técnico", 1, "D", "I"]], segue: "Projeto aprovado", dil: ["Lote abaixo do mínimo", "Corrigir o projeto", 1] },
    { u: "SEHAB/CARTOGRAFIA", s: "Averbação e cartografia", a: "Emitir a aprovação para registro no cartório.", p: 5, d: [["Certidão de aprovação", 1, "D", "I"]] },
  ]),
  P("HAB.ALI.008", "Alinhamento de lote", "Emitir laudo e nota de alinhamento do lote em relação ao logradouro.", "Proprietários e responsáveis técnicos", "6.0.02.01.00", [
    { u: "SEHAB/PROTOCOLO", s: "Atendimento", a: "Receber o pedido com a localização do lote.", p: 2, d: [["Requerimento de alinhamento", 1, "X", "I"]], segue: "Pedido recebido" },
    { u: "SEHAB/TOPOGRAFIA", s: "Topografia", a: "Vistoriar e emitir o laudo e a nota de alinhamento.", p: 15, d: [["Laudo e nota de alinhamento", 1, "D", "I"]] },
  ]),

  // ------------------------------------------------------------- Meio ambiente e agricultura (8.0)
  P("MAM.FIS.002", "Fiscalização ambiental", "Apurar denúncia ou constatação de infração ambiental, com auto de infração e prazo de defesa.", "Infratores e denunciantes", "8.0.01.00.00", [
    { u: "SEMMA/FISCALIZACAO", s: "Fiscalização ambiental", a: "Vistoriar o local e lavrar o auto de infração ou de notificação.", p: 10, d: [["Relatório de vistoria", 1, "D", "I"], ["Auto de infração ambiental", 0, "D", "I"]], segue: "Auto lavrado" },
    { u: "SEMMA/JULGAMENTO", s: "Julgamento ambiental", a: "Receber a defesa e julgar em primeira instância.", p: 30, d: [["Defesa do autuado", 0, "X"], ["Decisão de primeira instância", 1, "D", "I"]] },
  ]),
  P("MAM.ARV.003", "Remoção ou supressão de árvores", "Autorizar a poda, a remoção ou a supressão de vegetação, com compensação ambiental quando exigida.", "Cidadãos e empreendedores", "8.0.01.02.03", [
    { u: "SEMMA/ATENDIMENTO", s: "Atendimento", a: "Receber o pedido com a localização e o motivo.", p: 2, d: [["Requerimento de remoção/supressão", 1, "X", "I"], ["Fotos do local", 0, "X"]], segue: "Pedido recebido" },
    { u: "SEMMA/TECNICO", s: "Análise técnica", a: "Vistoriar e emitir o laudo e a compensação.", p: 15, d: [["Laudo técnico", 1, "D", "I"]], segue: "Remoção autorizada" },
    { u: "SEMMA/LICENCIAMENTO", s: "Licenciamento ambiental", a: "Emitir a autorização com as condicionantes.", p: 5, d: [["Autorização de remoção/supressão", 1, "D", "I"], ["Termo de compensação ambiental", 0, "D", "C"]] },
  ]),
  P("MAM.REC.004", "Recurso ao Conselho de Meio Ambiente (CONSEMMA)", "Julgar em segunda instância o recurso contra auto de infração ambiental.", "Autuados", "8.0.01.03.02", [
    { u: "SEMMA/CONSEMMA", s: "Secretaria executiva do CONSEMMA", a: "Receber o recurso no prazo e distribuir a um relator.", p: 5, d: [["Recurso administrativo", 1, "X", "I"]], segue: "Recurso distribuído" },
    { u: "SEMMA/CONSEMMA", s: "Plenária do CONSEMMA", a: "Julgar o recurso e publicar a decisão.", p: 60, d: [["Voto do relator", 1, "D", "I"], ["Decisão do conselho", 1, "D", "C"]] },
  ]),
  P("AGR.SIM.001", "Registro no Serviço de Inspeção Municipal (SIM)", "Registrar estabelecimento produtor de alimentos de origem animal no SIM.", "Produtores rurais e agroindústrias", "8.0.02.01.02", [
    { u: "SEMAGRI/SIM", s: "Serviço de Inspeção Municipal", a: "Receber o pedido com a planta e o memorial do estabelecimento.", p: 5, d: [["Requerimento de registro no SIM", 1, "X", "I"], ["Planta e memorial técnico", 1, "X"]], segue: "Pedido instruído" },
    { u: "SEMAGRI/SIM", s: "Médico veterinário", a: "Vistoriar as instalações e emitir o laudo.", p: 30, d: [["Laudo de vistoria", 1, "D", "I"]], segue: "Vistoria favorável", dil: ["Instalações inadequadas", "Adequar as instalações", 1] },
    { u: "SEMAGRI/SIM", s: "Serviço de Inspeção Municipal", a: "Emitir o certificado de registro.", p: 5, d: [["Certificado de registro no SIM", 1, "D", "I"]] },
  ]),
  P("AGR.TRA.002", "Serviço de trator e implementos ao produtor rural", "Agendar e executar serviço de mecanização agrícola para o pequeno produtor.", "Pequenos produtores rurais", "8.0.02.02.00", [
    { u: "SEMAGRI/ATENDIMENTO", s: "Atendimento ao produtor", a: "Cadastrar o pedido e conferir o enquadramento do produtor.", p: 3, d: [["Requerimento de serviço de trator", 1, "X", "I"]], segue: "Produtor enquadrado" },
    { u: "SEMAGRI/MECANIZACAO", s: "Patrulha mecanizada", a: "Agendar, executar e registrar as horas trabalhadas.", p: 30, d: [["Ordem de serviço", 1, "D", "I"], ["Termo de recebimento do serviço", 1, "X", "I"]] },
  ]),
  P("AGR.POC.003", "Perfuração de poço artesiano em área rural", "Avaliar e executar a perfuração de poço para abastecimento de propriedade rural.", "Produtores rurais", "8.0.02.03.02", [
    { u: "SEMAGRI/ATENDIMENTO", s: "Atendimento ao produtor", a: "Receber o pedido e a documentação da propriedade.", p: 3, d: [["Requerimento de perfuração de poço", 1, "X", "I"], ["Documento da propriedade", 1, "X"]], segue: "Pedido instruído" },
    { u: "SEMAGRI/EXTENSAO", s: "Extensão rural", a: "Avaliar a viabilidade técnica e a outorga necessária.", p: 30, d: [["Parecer técnico", 1, "D", "I"]], segue: "Perfuração viável" },
    { u: "SEMAGRI/EXTENSAO", s: "Extensão rural", a: "Executar ou contratar a perfuração e registrar a entrega.", p: 60, d: [["Termo de entrega do poço", 1, "D", "C"]] },
  ]),
  P("AGR.EST.004", "Manutenção de estrada rural", "Atender pedido de manutenção ou recuperação de estrada vicinal.", "Comunidades rurais", "8.0.02.04.00", [
    { u: "SEMAGRI/ATENDIMENTO", s: "Atendimento", a: "Registrar o pedido com a localização do trecho.", p: 2, d: [["Requerimento de manutenção de estrada", 1, "X", "I"]], segue: "Pedido registrado" },
    { u: "SEMAGRI/ESTRADAS", s: "Manutenção de estradas", a: "Vistoriar, programar e executar o serviço.", p: 45, d: [["Relatório de vistoria", 1, "D", "I"], ["Relatório de execução", 1, "D", "I"]] },
  ]),

  // ------------------------------------------------------------- Desenvolvimento econômico (9.0)
  P("DEC.INC.002", "Incentivo fiscal a empresa", "Conceder incentivo fiscal previsto na lei municipal de incentivo, com contrapartidas e acompanhamento.", "Empresas", "9.0.01.00.04", [
    { u: "SEDEC/ATENDIMENTO", s: "Sala do Empreendedor", a: "Receber o pedido com o projeto de investimento.", p: 5, d: [["Requerimento de incentivo fiscal", 1, "X", "I"], ["Projeto de investimento", 1, "X"]], segue: "Pedido instruído" },
    { u: "SEDEC/CONSELHO", s: "Conselho de desenvolvimento", a: "Analisar o projeto e as contrapartidas.", p: 30, d: [["Parecer do conselho", 1, "D", "C"]], segue: "Projeto aprovado" },
    { u: "SEREC/TRIBUTARIO", s: "Julgamento tributário", a: "Conceder o benefício e registrar as condições.", p: 15, d: [["Termo de concessão de incentivo", 1, "D", "C"]] },
  ]),
  P("DEC.IND.003", "Concessão de área em distrito industrial", "Conceder área em distrito industrial a empresa, conforme o roteiro do conselho diretor (CODIP), com escritura após a implantação.", "Empresas", "9.0.01.02.00", [
    { u: "SEDEC/INDUSTRIA", s: "Indústria", a: "Receber o pedido com o projeto e o cronograma de implantação.", p: 5, d: [["Requerimento de área industrial", 1, "X", "I"], ["Projeto de implantação", 1, "X"]], segue: "Pedido instruído" },
    { u: "SEDEC/CODIP", s: "Conselho Diretor (CODIP)", a: "Deliberar a concessão e as obrigações.", p: 30, d: [["Resolução do CODIP", 1, "D", "C"]], segue: "Concessão aprovada" },
    pgm("Contrato aprovado"),
    { u: "SEDEC/INDUSTRIA", s: "Indústria", a: "Firmar o contrato e acompanhar a implantação até a escritura.", p: 30, d: [["Contrato de concessão", 1, "D", "C"]] },
  ]),

  // ------------------------------------------------------------- Assistência social (10.0)
  P("SAS.CNV.002", "Convênio ou parceria na assistência social", "Firmar convênio ou parceria com outro ente ou com organização para executar ação socioassistencial.", "Entes e organizações parceiras", "10.0.01.00.03", [
    { u: "SEMPRAS/GESTAO", s: "Gestão do SUAS", a: "Elaborar o plano de trabalho e a justificativa.", p: 15, d: [["Plano de trabalho", 1, "D", "C"], ["Justificativa da parceria", 1, "D", "I"]], segue: "Plano aprovado" },
    pgm("Minuta aprovada", ["Ajustes na minuta", "Ajustar o plano ou a minuta", 1]),
    { u: "GAB", s: "Prefeito", a: "Assinar o termo e publicar o extrato.", p: 5, d: [["Termo de convênio/parceria", 1, "D", "C"], ["Extrato publicado", 1, "X"]] },
  ]),
  P("SAS.CHA.003", "Chamamento público de OSC", "Selecionar organização da sociedade civil por chamamento público (Lei 13.019/2014).", "Organizações da sociedade civil", "10.0.03.00.06", [
    { u: "SEMPRAS/GESTAO", s: "Gestão do SUAS", a: "Elaborar e publicar o edital de chamamento.", p: 15, d: [["Edital de chamamento público", 1, "D", "C"]], segue: "Edital publicado" },
    { u: "SEMPRAS/COMISSAO", s: "Comissão de seleção", a: "Avaliar as propostas e publicar o resultado.", p: 30, d: [["Ata de julgamento", 1, "D", "C"], ["Resultado do chamamento", 1, "D", "C"]], segue: "Resultado homologado" },
    { u: "SEMPRAS/GESTAO", s: "Gestão do SUAS", a: "Firmar o termo de colaboração ou fomento.", p: 15, d: [["Termo de colaboração/fomento", 1, "D", "C"]] },
  ]),
  P("SAS.BIL.004", "Bilhete único especial para pessoa com deficiência", "Conceder a gratuidade no transporte coletivo municipal à pessoa com deficiência e acompanhante, quando indicado.", "Pessoas com deficiência", "10.0.06.00.40", [
    { u: "SEMPRAS/ATENDIMENTO", s: "Atendimento", a: "Receber o pedido com o laudo médico e os documentos.", p: 2, d: [["Requerimento de bilhete único especial", 1, "X", "I"], ["Laudo médico", 1, "X"]], segue: "Pedido instruído" },
    { u: "SEMPRAS/AVALIACAO", s: "Avaliação social", a: "Avaliar os requisitos e a necessidade de acompanhante.", p: 15, d: [["Parecer social", 1, "D", "I"]], segue: "Benefício deferido" },
    { u: "SEMPRAS/ATENDIMENTO", s: "Atendimento", a: "Solicitar a emissão do cartão e entregar ao beneficiário.", p: 15, d: [["Termo de entrega do cartão", 1, "X", "I"]] },
  ], SAUDE),

  // ------------------------------------------------------------- Saúde (12.0)
  P("SAU.EMD.002", "Emenda parlamentar para a saúde", "Cadastrar, executar e prestar contas de recurso de emenda parlamentar destinado à saúde.", "Secretaria de Saúde", "12.0.01.00.13", [
    { u: "SMS/PLANEJAMENTO", s: "Planejamento em saúde", a: "Cadastrar a proposta no sistema do Ministério e acompanhar a aprovação.", p: 30, d: [["Proposta de aplicação", 1, "D", "I"]], segue: "Recurso creditado" },
    { u: "SMS/FUNDO", s: "Fundo Municipal de Saúde", a: "Executar a despesa conforme o objeto.", p: 180, d: [["Processo de execução da despesa", 1, "D", "I"]], segue: "Objeto executado" },
    { u: "SMS/FUNDO", s: "Fundo Municipal de Saúde", a: "Prestar contas no sistema federal.", p: 30, d: [["Prestação de contas", 1, "D", "C"]] },
  ]),
  P("SAU.CNE.003", "Cadastro de estabelecimento no SCNES", "Incluir, alterar ou excluir estabelecimento e profissionais no Cadastro Nacional de Estabelecimentos de Saúde.", "Estabelecimentos de saúde", "12.0.01.01.09", [
    { u: "SMS/CNES", s: "Cadastro CNES", a: "Receber a ficha cadastral e os documentos do estabelecimento.", p: 5, d: [["Ficha cadastral do CNES", 1, "X", "I"], ["Licença sanitária", 1, "X"]], segue: "Ficha conferida" },
    { u: "SMS/CNES", s: "Cadastro CNES", a: "Atualizar o SCNES e enviar a competência.", p: 10, d: [["Comprovante de atualização", 1]] },
  ]),
  P("SAU.EST.004", "Convênio de estágio nas unidades de saúde", "Firmar convênio com instituição de ensino para estágio supervisionado nas unidades de saúde.", "Instituições de ensino", "12.0.01.02.00", [
    { u: "SMS/EDUCACAO", s: "Educação permanente", a: "Receber a proposta e verificar as vagas nas unidades.", p: 10, d: [["Proposta de convênio de estágio", 1, "X", "I"], ["Plano de estágio", 1, "X"]], segue: "Vagas confirmadas" },
    pgm(),
    { u: "SMS/EDUCACAO", s: "Educação permanente", a: "Firmar o convênio e distribuir os estagiários.", p: 10, d: [["Convênio de estágio", 1, "D", "C"]] },
  ]),
  P("SAU.AUD.005", "Auditoria de serviço de saúde", "Auditar serviço próprio ou contratado do SUS, com relatório e providências.", "Prestadores do SUS", "12.0.01.05.01", [
    { u: "SMS/AUDITORIA", s: "Auditoria (DIACAUD)", a: "Planejar e comunicar a auditoria.", p: 10, d: [["Plano de auditoria", 1, "D", "I"]], segue: "Auditoria iniciada" },
    { u: "SMS/AUDITORIA", s: "Equipe de auditoria", a: "Executar e emitir o relatório preliminar.", p: 30, d: [["Relatório preliminar", 1, "D", "C"]], segue: "Relatório emitido" },
    { u: "SMS/AUDITORIA", s: "Auditoria (DIACAUD)", a: "Receber a manifestação do auditado e emitir o relatório final.", p: 30, d: [["Relatório final", 1, "D", "C"]] },
  ], SAUDE),
  P("SAU.PCF.006", "Prestação de contas do Fundo Municipal de Saúde", "Elaborar e apresentar a prestação de contas quadrimestral do Fundo Municipal de Saúde ao Conselho e ao Legislativo.", "Conselho Municipal de Saúde e Câmara", "12.0.01.08.03", [
    { u: "SMS/FUNDO", s: "Fundo Municipal de Saúde", a: "Consolidar a execução orçamentária e física do quadrimestre.", p: 20, d: [["Relatório Detalhado do Quadrimestre Anterior (RDQA)", 1, "D", "C"]], segue: "Relatório consolidado" },
    { u: "SMS/CMS", s: "Conselho Municipal de Saúde", a: "Apreciar e deliberar sobre o relatório.", p: 30, d: [["Resolução do Conselho de Saúde", 1, "D", "C"]], segue: "Relatório apreciado" },
    { u: "SMS/FUNDO", s: "Fundo Municipal de Saúde", a: "Apresentar em audiência pública na Câmara.", p: 15, d: [["Ata da audiência pública", 1, "X"]] },
  ]),
  P("SAU.DIU.007", "Inserção de DIU", "Atender a solicitação de método contraceptivo de longa duração (DIU) na rede municipal.", "Usuárias do SUS", "12.0.02.00.81", [
    { u: "SMS/UBS", s: "Unidade Básica de Saúde", a: "Acolher a solicitação, orientar e encaminhar com os exames.", p: 7, d: [["Solicitação de DIU", 1, "D", "I"]], segue: "Encaminhada" },
    { u: "SMS/SAUDE_MULHER", s: "Saúde da mulher", a: "Agendar e realizar o procedimento.", p: 30, d: [["Termo de consentimento", 1, "X", "I"], ["Registro do procedimento", 1, "D", "I"]] },
  ], SAUDE),
  P("SAU.CNS.008", "Cartão Nacional de Saúde", "Cadastrar ou atualizar o usuário no Cartão Nacional de Saúde (CNS).", "Usuários do SUS", "12.0.04.00.01", [
    { u: "SMS/UBS", s: "Recepção da unidade", a: "Conferir os documentos e cadastrar ou atualizar no CADSUS.", p: 1, d: [["Documento de identificação", 1, "X"], ["Comprovante de endereço", 1, "X"]], segue: "Cadastro feito" },
    { u: "SMS/UBS", s: "Recepção da unidade", a: "Entregar o número do CNS ao usuário.", p: 1, d: [] },
  ], SAUDE),
  P("SAU.PRO.009", "Cópia de prontuário", "Fornecer cópia do prontuário ao paciente ou ao representante legal, preservando o sigilo.", "Pacientes e representantes legais", "12.0.07.02.04", [
    { u: "SMS/ARQUIVO", s: "Arquivo médico", a: "Receber o pedido e conferir a legitimidade do solicitante.", p: 3, d: [["Requerimento de cópia de prontuário", 1, "X", "I"], ["Documento de identificação ou procuração", 1, "X"]], segue: "Solicitante legítimo" },
    { u: "SMS/ARQUIVO", s: "Arquivo médico", a: "Reproduzir o prontuário e entregar mediante recibo.", p: 10, d: [["Recibo de entrega", 1, "X", "I"]] },
  ], SAUDE),
  P("SAU.OPM.010", "Órteses, próteses e meios auxiliares (OPM)", "Cadastrar e atender o paciente no programa de órteses, próteses e materiais especiais (auditivo, ortopédico).", "Usuários do SUS", "12.0.07.04.16", [
    { u: "SMS/OPM", s: "Programa de OPM", a: "Cadastrar o paciente com a prescrição médica.", p: 5, d: [["Prescrição médica", 1, "X", "I"], ["Ficha de cadastro no programa", 1, "D", "I"]], segue: "Cadastro feito" },
    { u: "SMS/REGULACAO", s: "Complexo regulador", a: "Autorizar e agendar a concessão.", p: 30, d: [["Autorização de procedimento", 1, "D", "I"]], segue: "Concessão autorizada" },
    { u: "SMS/OPM", s: "Programa de OPM", a: "Entregar o equipamento e orientar o uso.", p: 30, d: [["Termo de recebimento", 1, "X", "I"]] },
  ], SAUDE),
  P("SAU.PLA.011", "Inclusão ou exclusão de plantão médico", "Ajustar a escala de plantões médicos conforme a demanda das unidades.", "Coordenações de unidades de saúde", "12.0.08.05.08", [
    { u: "SMS/UNIDADE", s: "Coordenação da unidade", a: "Solicitar a inclusão ou exclusão com a justificativa de demanda.", p: 3, d: [["Solicitação de alteração de plantão", 1, "D", "I"]], segue: "Pedido justificado" },
    { u: "SMS/RH_MEDICOS", s: "RH dos médicos", a: "Ajustar a escala e informar a folha.", p: 5, d: [["Escala de plantões", 1, "D", "I"]] },
  ]),
  P("SAU.AGU.012", "Análise microbiológica de água", "Receber amostra e emitir laudo de análise microbiológica de água para consumo.", "Cidadãos, empresas e vigilância", "12.0.11.03.01", [
    { u: "SMS/LACEN_AGUA", s: "Laboratório de análise de água", a: "Receber a amostra com a ficha de coleta.", p: 1, d: [["Solicitação de análise", 1, "X", "I"], ["Ficha de coleta", 1, "X"]], segue: "Amostra aceita" },
    { u: "SMS/LACEN_AGUA", s: "Laboratório de análise de água", a: "Analisar e emitir o laudo.", p: 7, d: [["Laudo de análise microbiológica", 1, "D", "I"]] },
  ]),
];

const arquivo = {
  procedimentos: procedimentos.map((p) => ({
    codigo_processual: p.codigo,
    titulo: p.titulo,
    objetivo: p.objetivo,
    publico_alvo: p.publico,
    nivel_acesso: p.nivel ?? "PUBLICO",
    ...(p.hipotese ? { hipotese_legal_restricao: p.hipotese } : {}),
    codigo_ttdd: p.ttdd,
    etapas: p.etapas.map((e, i) => {
      const transicoes = [];
      if (i < p.etapas.length - 1) transicoes.push({ destino_ordem: i + 2, condicao_transicao: e.segue ?? "Etapa concluída" });
      if (e.dil) transicoes.push({ destino_ordem: e.dil[2], condicao_transicao: e.dil[0], is_devolucao_diligencia: true, descricao_diligencia: e.dil[1] });
      return {
        ordem: i + 1,
        unidade_administrativa: e.u,
        nome_setor: e.s,
        atribuicoes_setor: e.a,
        prazo_sla_em_dias: e.p,
        ...(e.d.length
          ? {
              documentos: e.d.map(([nome, obrig, f = "D", a = "I"]) => ({
                nome_documento: nome,
                obrigatorio: obrig === 1,
                formato: FORMATO[f],
                tipo_assinatura: ASSINATURA[a],
              })),
            }
          : {}),
        ...(transicoes.length ? { transicoes } : {}),
      };
    }),
  })),
};

const destino = join(dirname(fileURLToPath(import.meta.url)), "rascunhos-procedimentos-2.json");
writeFileSync(destino, JSON.stringify(arquivo, null, 2) + "\n");
console.log(`${arquivo.procedimentos.length} rascunhos → ${destino}`);
