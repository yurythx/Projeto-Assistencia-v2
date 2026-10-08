// Gera deploy/atlas/rascunhos-procedimentos.json: os RASCUNHOS dos
// procedimentos prioritários, no formato da importação do Atlas (ADR 025).
// São ponto de partida para as entrevistas de validação nos departamentos
// (ADR 028 — docs/atlas/PLANO_DE_VALIDACAO.md): etapas, setores, prazos e
// peças são hipóteses a confirmar, e as siglas das unidades devem ser
// trocadas pelas cadastradas em Configurações > Unidades.
//
// Uso: node deploy/atlas/gerar-rascunhos.mjs
// Importar: Atlas > Importar > "Importar como rascunho" (padrão) > Simular.

import { writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

// Peça: [nome, obrigatória?, formato?, assinatura?]
// Formato: D = nato-digital, X = externo digitalizado. Assinatura: I, C (conjunta), B (em bloco).
const FORMATO = { D: "NATO_DIGITAL", X: "EXTERNO_DIGITALIZADO" };
const ASSINATURA = { I: "INDIVIDUAL", C: "CONJUNTA_MULTINIVEL", B: "EM_BLOCO" };

// Etapa: { u: sigla, s: setor, a: atribuições, p: prazo (dias), d: peças,
//          segue: condição para seguir, dil: [condição, diligência, etapa de retorno] }
const P = (codigo, titulo, objetivo, publico, ttdd, etapas, extra = {}) => ({ codigo, titulo, objetivo, publico, ttdd, etapas, ...extra });

const procedimentos = [
  // ------------------------------------------------------------- Administração (2.0)
  P("ADM.CMP.004", "Adesão a Ata de Registro de Preços", "Contratar por adesão a ata de registro de preços de outro órgão (carona), com justificativa de vantajosidade e anuência do órgão gerenciador e do fornecedor.", "Secretarias municipais", "2.0.02.00.10", [
    { u: "SECRETARIA", s: "Setor demandante", a: "Formalizar a demanda e justificar a vantagem da adesão.", p: 5, d: [["Documento de Formalização da Demanda (DFD)", 1], ["Pesquisa de preços", 1], ["Justificativa de vantajosidade", 1]], segue: "Demanda formalizada" },
    { u: "SEMAD/COMPRAS", s: "Compras", a: "Solicitar a anuência do órgão gerenciador e do fornecedor e conferir a ata.", p: 10, d: [["Cópia da Ata de Registro de Preços", 1, "X"], ["Ofício de anuência do órgão gerenciador", 1, "X"], ["Aceite do fornecedor", 1, "X"]], segue: "Anuências recebidas", dil: ["Justificativa insuficiente", "Complementar a justificativa de vantajosidade", 1] },
    { u: "PGM", s: "Procuradoria", a: "Emitir parecer jurídico sobre a adesão.", p: 7, d: [["Parecer jurídico", 1, "D", "I"]], segue: "Parecer favorável", dil: ["Parecer com ressalvas", "Sanear os apontamentos do parecer", 2] },
    { u: "GAB", s: "Ordenador de despesa", a: "Autorizar a adesão e a contratação.", p: 3, d: [["Despacho de autorização", 1, "D", "I"]], segue: "Adesão autorizada" },
    { u: "SEMAD/CONTRATOS", s: "Contratos", a: "Formalizar o contrato e publicar o extrato.", p: 5, d: [["Contrato", 1, "D", "C"], ["Extrato publicado", 1, "X"]] },
  ]),
  P("ADM.CON.005", "Formalização de contrato administrativo e termos aditivos", "Elaborar, assinar e publicar o contrato administrativo ou o termo aditivo (prazo, valor ou objeto) depois da contratação autorizada.", "Secretarias municipais", "2.0.02.02.12", [
    { u: "SECRETARIA", s: "Setor demandante", a: "Solicitar a elaboração do contrato ou do aditivo, com a justificativa.", p: 3, d: [["Solicitação de elaboração de contrato/aditivo", 1], ["Justificativa do aditivo", 0]], segue: "Solicitação completa" },
    { u: "SEMAD/CONTRATOS", s: "Contratos", a: "Minutar o instrumento e conferir a regularidade fiscal do contratado.", p: 5, d: [["Minuta do contrato/aditivo", 1], ["Certidões de regularidade fiscal e trabalhista", 1, "X"]], segue: "Minuta pronta", dil: ["Documentação incompleta", "Enviar os documentos que faltam", 1] },
    { u: "PGM", s: "Procuradoria", a: "Analisar a minuta e emitir parecer.", p: 5, d: [["Parecer jurídico", 1, "D", "I"]], segue: "Minuta aprovada", dil: ["Ajustes na minuta", "Ajustar a minuta conforme o parecer", 2] },
    { u: "SEMAD/CONTRATOS", s: "Contratos", a: "Colher as assinaturas, publicar o extrato e designar o fiscal.", p: 5, d: [["Contrato/aditivo assinado", 1, "D", "C"], ["Extrato publicado", 1, "X"], ["Portaria de designação do fiscal", 1, "D", "I"]] },
  ]),
  P("ADM.CON.006", "Fiscalização e acompanhamento de contrato", "Acompanhar a execução do contrato pelo fiscal designado, com relatórios periódicos e atesto para pagamento.", "Fiscais de contrato", "2.0.02.02.15", [
    { u: "SECRETARIA", s: "Fiscal do contrato", a: "Acompanhar a execução, registrar ocorrências e emitir o relatório mensal.", p: 30, d: [["Relatório mensal do fiscal", 1, "D", "I"], ["Registro de ocorrências", 0]], segue: "Execução conforme" },
    { u: "SECRETARIA", s: "Gestor do contrato", a: "Atestar a nota fiscal e encaminhar para pagamento ou notificar o contratado.", p: 5, d: [["Nota fiscal atestada", 1, "X", "I"], ["Notificação ao contratado", 0]], segue: "Nota atestada", dil: ["Inexecução ou falha", "Notificar o contratado e registrar a ocorrência", 1] },
    { u: "SEFIN/CONTABILIDADE", s: "Contabilidade", a: "Liquidar e pagar a despesa.", p: 10, d: [["Nota de liquidação", 1], ["Ordem de pagamento", 1]] },
  ]),
  P("ADM.CMP.007", "Contratação por inexigibilidade de licitação", "Contratar diretamente quando a competição é inviável (fornecedor exclusivo, notória especialização, credenciamento — art. 74 da Lei 14.133/2021).", "Secretarias municipais", "2.0.02.01.03", [
    { u: "SECRETARIA", s: "Setor demandante", a: "Formalizar a demanda, o estudo técnico preliminar e a justificativa da inviabilidade de competição.", p: 7, d: [["Documento de Formalização da Demanda (DFD)", 1], ["Estudo Técnico Preliminar (ETP)", 1], ["Termo de Referência", 1], ["Comprovação de exclusividade ou notória especialização", 1, "X"]], segue: "Instrução completa" },
    { u: "SEMAD/COMPRAS", s: "Compras", a: "Justificar o preço e conferir a habilitação do contratado.", p: 7, d: [["Justificativa de preço", 1], ["Documentos de habilitação", 1, "X"]], segue: "Preço justificado", dil: ["Instrução incompleta", "Complementar a instrução", 1] },
    { u: "PGM", s: "Procuradoria", a: "Emitir parecer jurídico.", p: 7, d: [["Parecer jurídico", 1, "D", "I"]], segue: "Parecer favorável" },
    { u: "GAB", s: "Autoridade competente", a: "Ratificar a inexigibilidade e autorizar a contratação.", p: 3, d: [["Ato de ratificação", 1, "D", "I"], ["Publicação do ato", 1, "X"]] },
  ]),
  P("ADM.FRO.008", "Infração de trânsito de veículo oficial", "Identificar o condutor responsável por infração com veículo oficial, apresentar defesa quando cabível e ressarcir a multa.", "Condutores e gestores de frota", "2.0.01.02.15", [
    { u: "SEMAD/FROTAS", s: "Gestão de frotas", a: "Receber a notificação, identificar o condutor pelo diário de bordo e indicá-lo ao órgão de trânsito.", p: 10, d: [["Notificação de autuação", 1, "X"], ["Diário de bordo do período", 1, "X"], ["Formulário de indicação de condutor", 1, "X", "C"]], segue: "Condutor identificado" },
    { u: "SECRETARIA", s: "Chefia do condutor", a: "Colher a defesa do condutor ou a concordância com o desconto.", p: 10, d: [["Defesa do condutor", 0], ["Termo de ciência e autorização de desconto", 0, "D", "I"]], segue: "Manifestação recebida" },
    { u: "SEGEP/FOLHA", s: "Folha de pagamento", a: "Descontar em folha o valor da multa, quando devido.", p: 30, d: [["Comprovante de desconto", 1]] },
  ]),
  P("ADM.SIC.009", "Pedido de acesso à informação (LAI)", "Atender o pedido de acesso à informação no prazo da Lei 12.527/2011 (20 dias, prorrogáveis por mais 10), com recurso à autoridade superior.", "Cidadãos", "2.0.03.02.00", [
    { u: "SEMAD/SIC", s: "Serviço de Informação ao Cidadão", a: "Registrar o pedido e encaminhar à unidade que detém a informação.", p: 2, d: [["Formulário de pedido de informação", 1, "X"]], segue: "Pedido encaminhado" },
    { u: "SECRETARIA", s: "Unidade detentora", a: "Localizar a informação ou justificar o sigilo/indisponibilidade.", p: 15, d: [["Resposta da unidade", 1, "D", "I"]], segue: "Resposta pronta", dil: ["Pedido genérico ou incompreensível", "Pedir ao cidadão que esclareça o pedido", 1] },
    { u: "SEMAD/SIC", s: "Serviço de Informação ao Cidadão", a: "Enviar a resposta ao cidadão e informar o prazo de recurso.", p: 3, d: [["Carta resposta ao pedido de informação", 1, "D", "I"]] },
  ]),

  // ------------------------------------------------------------- Gestão de pessoas (2.0.05–2.0.08)
  P("RH.FER.001", "Férias do servidor", "Programar e conceder o gozo de férias do servidor, com o adicional de 1/3 na folha.", "Servidores municipais", "2.0.06.03.01", [
    { u: "SECRETARIA", s: "Chefia imediata", a: "Aprovar o período de gozo conforme a escala da unidade.", p: 5, d: [["Requerimento de gozo de férias", 1, "D", "I"]], segue: "Período aprovado" },
    { u: "SEGEP/RH", s: "Recursos Humanos", a: "Conferir o período aquisitivo e lançar as férias.", p: 10, d: [["Aviso de férias", 1, "D", "I"]], segue: "Férias lançadas", dil: ["Período aquisitivo incompleto", "Ajustar o período de gozo", 1] },
    { u: "SEGEP/FOLHA", s: "Folha de pagamento", a: "Pagar o adicional de 1/3 na folha do mês anterior ao gozo.", p: 30, d: [] },
  ]),
  P("RH.LIC.002", "Licença-prêmio por assiduidade", "Reconhecer o direito à licença-prêmio e programar o gozo, conforme o estatuto dos servidores.", "Servidores efetivos", "2.0.06.03.05", [
    { u: "SEGEP/RH", s: "Recursos Humanos", a: "Receber o requerimento e certificar o tempo de serviço e as ausências do período.", p: 15, d: [["Requerimento de usufruto de licença-prêmio", 1, "D", "I"], ["Certidão de tempo de serviço", 1, "D", "I"]], segue: "Tempo certificado" },
    { u: "PGM", s: "Procuradoria", a: "Analisar casos com afastamentos que interrompem o período (quando houver).", p: 10, d: [["Parecer jurídico", 0, "D", "I"]], segue: "Direito reconhecido" },
    { u: "SEGEP/RH", s: "Recursos Humanos", a: "Deferir ou indeferir e publicar o ato; programar o gozo com a chefia.", p: 10, d: [["Portaria de concessão", 1, "D", "I"], ["Requerimento de gozo de licença-prêmio", 0, "D", "I"]] },
  ]),
  P("RH.ADI.003", "Adicional de insalubridade", "Conceder o adicional de insalubridade com base em laudo técnico das condições de trabalho.", "Servidores municipais", "2.0.06.02.09", [
    { u: "SECRETARIA", s: "Chefia imediata", a: "Encaminhar o requerimento com a descrição das atividades e do local.", p: 5, d: [["Requerimento de adicional de insalubridade", 1, "D", "I"], ["Descrição das atividades", 1]], segue: "Requerimento instruído" },
    { u: "SEGEP/DESOPEM", s: "Saúde ocupacional (DESOPEM)", a: "Emitir o laudo técnico de insalubridade (LTCAT) e o grau.", p: 30, d: [["Laudo técnico (LTCAT)", 1, "D", "C"]], segue: "Laudo emitido" },
    { u: "SEGEP/RH", s: "Recursos Humanos", a: "Conceder o adicional e lançar na folha.", p: 10, d: [["Portaria de concessão", 1, "D", "I"]] },
  ]),
  P("RH.APO.004", "Aposentadoria voluntária por tempo de contribuição", "Instruir e conceder a aposentadoria voluntária do servidor efetivo pelo regime próprio (IMPRO).", "Servidores efetivos", "2.0.07.01.01", [
    { u: "SEGEP/RH", s: "Recursos Humanos", a: "Receber o requerimento e emitir a certidão de tempo de contribuição e o histórico funcional.", p: 20, d: [["Requerimento de aposentadoria", 1, "D", "I"], ["Certidão de tempo de contribuição", 1, "D", "I"], ["Ficha financeira", 1]], segue: "Tempo certificado" },
    { u: "IMPRO", s: "Instituto de Previdência (IMPRO)", a: "Calcular os proventos e conferir os requisitos.", p: 30, d: [["Cálculo de proventos", 1, "D", "C"]], segue: "Requisitos atendidos", dil: ["Tempo ou documento faltante", "Averbar o tempo ou juntar o documento", 1] },
    { u: "PGM", s: "Procuradoria", a: "Emitir parecer jurídico.", p: 15, d: [["Parecer jurídico", 1, "D", "I"]], segue: "Parecer favorável" },
    { u: "GAB", s: "Prefeito", a: "Assinar o ato de aposentadoria e publicar.", p: 10, d: [["Portaria de aposentadoria", 1, "D", "I"], ["Publicação no Diário Oficial", 1, "X"]], segue: "Ato publicado" },
    { u: "IMPRO", s: "Instituto de Previdência (IMPRO)", a: "Encaminhar o ato ao Tribunal de Contas para registro.", p: 30, d: [["Ofício ao TCE", 1, "D", "I"]] },
  ]),
  P("RH.PAD.005", "Processo Administrativo Disciplinar (PAD)", "Apurar infração disciplinar de servidor com comissão, ampla defesa e contraditório, conforme o estatuto.", "Servidores municipais", "2.0.05.02.04", [
    { u: "GAB", s: "Autoridade instauradora", a: "Instaurar o PAD e designar a comissão.", p: 5, d: [["Portaria de instauração", 1, "D", "I"]], segue: "Comissão designada" },
    { u: "SEGEP/CPAD", s: "Comissão processante", a: "Citar o servidor, instruir, colher a defesa e elaborar o relatório final.", p: 60, d: [["Termo de citação", 1, "D", "C"], ["Termos de depoimento", 1, "D", "C"], ["Defesa escrita", 1, "X"], ["Relatório final da comissão", 1, "D", "C"]], segue: "Relatório concluído" },
    { u: "PGM", s: "Procuradoria", a: "Emitir parecer sobre a regularidade do processo.", p: 15, d: [["Parecer jurídico", 1, "D", "I"]], segue: "Processo regular", dil: ["Nulidade ou vício", "Refazer o ato viciado", 2] },
    { u: "GAB", s: "Autoridade julgadora", a: "Julgar e aplicar a penalidade ou arquivar.", p: 20, d: [["Decisão de julgamento", 1, "D", "I"]] },
  ], { nivel: "RESTRITO", hipotese: "LAI, art. 31 (informação pessoal) e estatuto dos servidores" }),
  P("RH.EST.006", "Avaliação de estágio probatório", "Avaliar o servidor nomeado durante o estágio probatório e homologar a estabilidade.", "Servidores em estágio probatório", "2.0.08.01.02", [
    { u: "SECRETARIA", s: "Chefia imediata", a: "Preencher as avaliações periódicas com ciência do servidor.", p: 30, d: [["Avaliação de estágio probatório", 1, "D", "C"]], segue: "Avaliações concluídas" },
    { u: "SEGEP/CAEP", s: "Comissão de avaliação", a: "Consolidar as avaliações e emitir o parecer final.", p: 20, d: [["Parecer da comissão", 1, "D", "C"]], segue: "Parecer emitido", dil: ["Avaliação incompleta", "Completar as avaliações periódicas", 1] },
    { u: "SEGEP/RH", s: "Recursos Humanos", a: "Homologar o estágio probatório e registrar na pasta funcional.", p: 10, d: [["Homologação do estágio probatório", 1, "D", "I"]] },
  ]),
  P("RH.CON.007", "Posse de aprovado em concurso público", "Convocar o aprovado, conferir os documentos e a aptidão médica e dar posse no cargo.", "Aprovados em concurso público", "2.0.08.00.14", [
    { u: "SEGEP/RH", s: "Recursos Humanos", a: "Publicar a convocação e receber a documentação para a posse.", p: 15, d: [["Edital de convocação", 1, "X"], ["Documentação pessoal", 1, "X"], ["Declaração de acúmulo de cargos", 1, "X", "I"]], segue: "Documentação completa" },
    { u: "SEGEP/DESOPEM", s: "Saúde ocupacional (DESOPEM)", a: "Realizar a perícia de admissão e emitir o laudo de aptidão.", p: 10, d: [["Laudo de aptidão física e mental", 1, "D", "I"]], segue: "Apto", dil: ["Exame complementar", "Apresentar o exame solicitado", 1] },
    { u: "SEGEP/RH", s: "Recursos Humanos", a: "Lavrar o termo de posse e cadastrar o servidor.", p: 5, d: [["Portaria de nomeação", 1, "X"], ["Termo de posse", 1, "D", "C"]] },
  ]),

  // ------------------------------------------------------------- Fazenda (3.0)
  P("FAZ.DIA.001", "Concessão e prestação de contas de diárias", "Conceder diárias para viagem a serviço e receber a prestação de contas no retorno.", "Servidores municipais", "3.0.01.00.05", [
    { u: "SECRETARIA", s: "Chefia imediata", a: "Solicitar as diárias com a justificativa e o roteiro da viagem.", p: 3, d: [["Solicitação de diárias", 1, "D", "I"]], segue: "Viagem autorizada" },
    { u: "SEFIN/CONTABILIDADE", s: "Contabilidade", a: "Empenhar e pagar as diárias.", p: 5, d: [["Nota de empenho", 1], ["Ordem de pagamento", 1]], segue: "Diárias pagas" },
    { u: "SECRETARIA", s: "Servidor", a: "Prestar contas em até 5 dias do retorno, com os comprovantes.", p: 5, d: [["Relatório de viagem", 1, "D", "I"], ["Comprovantes de deslocamento", 1, "X"]], segue: "Contas prestadas" },
    { u: "SEFIN/CONTABILIDADE", s: "Contabilidade", a: "Analisar a prestação de contas e aprovar ou pedir a devolução.", p: 10, d: [["Parecer sobre a prestação de contas", 1, "D", "I"]], dil: ["Comprovação insuficiente", "Complementar a prestação de contas ou devolver o valor", 3] },
  ]),
  P("FAZ.DES.002", "Liquidação e pagamento de despesa", "Liquidar a despesa empenhada a partir do atesto e pagar o credor, na ordem cronológica.", "Fornecedores e secretarias", "3.0.01.00.02", [
    { u: "SECRETARIA", s: "Gestor do contrato", a: "Atestar o recebimento do bem ou serviço e encaminhar a nota fiscal.", p: 5, d: [["Nota fiscal atestada", 1, "X", "I"], ["Termo de recebimento", 0, "D", "I"]], segue: "Nota atestada" },
    { u: "SEFIN/CONTABILIDADE", s: "Contabilidade", a: "Conferir o empenho, as retenções e a regularidade fiscal e liquidar.", p: 5, d: [["Nota de liquidação", 1], ["Certidões de regularidade", 1, "X"]], segue: "Despesa liquidada", dil: ["Nota com erro ou certidão vencida", "Corrigir a nota ou atualizar as certidões", 1] },
    { u: "SEFIN/TESOURARIA", s: "Tesouraria", a: "Pagar o credor obedecendo a ordem cronológica.", p: 10, d: [["Ordem de pagamento", 1], ["Comprovante bancário", 1]] },
  ]),
  P("FAZ.PAR.003", "Parcelamento de débitos municipais", "Firmar acordo de parcelamento de tributos e da dívida ativa, com confissão de dívida.", "Contribuintes", "3.0.02.00.03", [
    { u: "SEREC/ATENDIMENTO", s: "Atendimento ao contribuinte", a: "Receber o requerimento e emitir o extrato dos débitos.", p: 2, d: [["Requerimento de parcelamento", 1, "X", "I"], ["Extrato de débitos", 1]], segue: "Débitos apurados" },
    { u: "SEREC/DIVIDA", s: "Dívida ativa", a: "Simular as parcelas conforme a lei e lavrar o termo de confissão.", p: 5, d: [["Termo de confissão de dívida", 1, "D", "C"]], segue: "Termo assinado" },
    { u: "SEREC/ARRECADACAO", s: "Arrecadação", a: "Emitir as guias e acompanhar o pagamento.", p: 2, d: [["Guias de pagamento", 1]] },
  ]),
  P("FAZ.IPT.004", "Isenção de IPTU", "Reconhecer a isenção de IPTU prevista em lei (ex.: aposentado, imóvel único) a pedido do contribuinte.", "Contribuintes", "3.0.02.01.01", [
    { u: "SEREC/ATENDIMENTO", s: "Atendimento ao contribuinte", a: "Receber o requerimento com os comprovantes.", p: 2, d: [["Requerimento de isenção", 1, "X", "I"], ["Comprovantes do requisito legal", 1, "X"], ["Matrícula ou documento do imóvel", 1, "X"]], segue: "Requerimento instruído" },
    { u: "SEREC/CADASTRO", s: "Cadastro imobiliário", a: "Conferir o cadastro do imóvel e os requisitos.", p: 15, d: [["Informação do cadastro imobiliário", 1, "D", "I"]], segue: "Requisitos conferidos", dil: ["Documento faltante", "Pedir o documento ao contribuinte", 1] },
    { u: "SEREC/TRIBUTARIO", s: "Julgamento tributário", a: "Decidir o pedido e lançar a isenção no cadastro.", p: 15, d: [["Decisão administrativa", 1, "D", "I"]] },
  ]),
  P("FAZ.ALV.005", "Alvará de funcionamento", "Licenciar o funcionamento de atividade econômica, com consulta de uso do solo e vistorias exigidas.", "Empresas e profissionais autônomos", "3.0.03.00.04", [
    { u: "SEREC/LICENCIAMENTO", s: "Licenciamento econômico", a: "Receber o pedido, conferir a inscrição municipal e a consulta de uso do solo.", p: 5, d: [["Requerimento de alvará", 1, "X", "I"], ["Contrato social ou CNPJ", 1, "X"], ["Certidão de uso do solo", 1, "X"]], segue: "Pedido instruído" },
    { u: "SEREC/FISCALIZACAO", s: "Fiscalização", a: "Vistoriar o estabelecimento quando a atividade exigir.", p: 10, d: [["Relatório de vistoria", 0, "D", "I"]], segue: "Vistoria favorável", dil: ["Irregularidade na vistoria", "Sanar a irregularidade apontada", 1] },
    { u: "SEREC/LICENCIAMENTO", s: "Licenciamento econômico", a: "Emitir o alvará após o pagamento da taxa.", p: 3, d: [["Alvará de funcionamento", 1, "D", "I"]] },
  ]),
  P("FAZ.CND.006", "Certidão negativa de débitos municipais", "Emitir certidão negativa (ou positiva com efeito de negativa) de débitos do contribuinte.", "Contribuintes", "3.0.02.00.00", [
    { u: "SEREC/ATENDIMENTO", s: "Atendimento ao contribuinte", a: "Receber o pedido e consultar a situação fiscal.", p: 1, d: [["Requerimento de certidão", 1, "X"]], segue: "Sem pendências" },
    { u: "SEREC/DIVIDA", s: "Dívida ativa", a: "Analisar pendências apontadas (parcelamento, suspensão de exigibilidade).", p: 5, d: [["Informação sobre pendências", 0, "D", "I"]], segue: "Situação definida" },
    { u: "SEREC/ATENDIMENTO", s: "Atendimento ao contribuinte", a: "Emitir a certidão.", p: 1, d: [["Certidão negativa de débitos", 1, "D", "I"]] },
  ]),

  // ------------------------------------------------------------- Controle interno (4.0)
  P("CTR.OUV.001", "Manifestação de ouvidoria", "Receber, encaminhar e responder reclamação, denúncia, sugestão ou elogio do cidadão (Lei 13.460/2017).", "Cidadãos", "4.0.01.02.00", [
    { u: "UCCI/OUVIDORIA", s: "Ouvidoria-Geral", a: "Registrar a manifestação, preservar a identidade do manifestante e encaminhar.", p: 3, d: [["Formulário de manifestação", 1, "X"]], segue: "Manifestação encaminhada" },
    { u: "SECRETARIA", s: "Unidade responsável", a: "Apurar e responder à ouvidoria.", p: 20, d: [["Resposta da unidade", 1, "D", "I"]], segue: "Resposta recebida" },
    { u: "UCCI/OUVIDORIA", s: "Ouvidoria-Geral", a: "Avaliar a resposta, responder ao manifestante e encerrar.", p: 5, d: [["Resposta ao manifestante", 1, "D", "I"]], dil: ["Resposta incompleta", "Complementar a resposta", 2] },
  ], { nivel: "RESTRITO", hipotese: "Lei 13.460/2017, art. 10, §7º (identidade do manifestante)" }),

  // ------------------------------------------------------------- Habitação e urbanismo (6.0)
  P("HAB.ALV.001", "Alvará de licença para construção", "Aprovar o projeto e licenciar a obra conforme o código de obras e o plano diretor.", "Proprietários e responsáveis técnicos", "6.0.02.00.00", [
    { u: "SEHAB/PROTOCOLO", s: "Atendimento", a: "Receber o pedido com o projeto e a responsabilidade técnica.", p: 3, d: [["Requerimento de alvará de construção", 1, "X", "I"], ["Projeto arquitetônico", 1, "X"], ["ART/RRT do responsável técnico", 1, "X"], ["Matrícula do imóvel", 1, "X"]], segue: "Pedido instruído" },
    { u: "SEHAB/APROVACAO", s: "Aprovação de projetos", a: "Analisar o projeto quanto às normas urbanísticas.", p: 30, d: [["Parecer técnico de análise", 1, "D", "I"]], segue: "Projeto aprovado", dil: ["Projeto com pendências", "Corrigir o projeto conforme o comunique-se", 1] },
    { u: "SEHAB/APROVACAO", s: "Aprovação de projetos", a: "Emitir o alvará após o pagamento das taxas.", p: 5, d: [["Alvará de licença para construção", 1, "D", "I"]] },
  ]),
  P("HAB.HAB.002", "Habite-se", "Vistoriar a obra concluída e emitir o habite-se (certificado de conclusão).", "Proprietários", "6.0.02.00.08", [
    { u: "SEHAB/PROTOCOLO", s: "Atendimento", a: "Receber o pedido com o alvará e os laudos exigidos.", p: 3, d: [["Requerimento de habite-se", 1, "X", "I"], ["Alvará de construção", 1, "X"], ["Certificado do Corpo de Bombeiros", 0, "X"]], segue: "Pedido instruído" },
    { u: "SEHAB/FISCALIZACAO", s: "Fiscalização de obras", a: "Vistoriar a obra e conferir com o projeto aprovado.", p: 15, d: [["Relatório de vistoria", 1, "D", "I"]], segue: "Obra conforme", dil: ["Obra em desacordo com o projeto", "Adequar a obra ou aprovar projeto modificativo", 1] },
    { u: "SEHAB/APROVACAO", s: "Aprovação de projetos", a: "Emitir o habite-se.", p: 5, d: [["Habite-se", 1, "D", "I"]] },
  ]),
  P("HAB.REU.003", "Regularização fundiária de interesse social (REURB-S)", "Regularizar a ocupação de núcleo urbano informal de baixa renda e titular o ocupante (Lei 13.465/2017).", "Ocupantes de núcleos informais", "6.0.01.01.00", [
    { u: "SEHAB/REURB", s: "Regularização fundiária", a: "Cadastrar o ocupante e enquadrar a modalidade REURB-S.", p: 30, d: [["Requerimento de REURB", 1, "X", "I"], ["Cadastro socioeconômico", 1, "D", "I"], ["Documentos pessoais do ocupante", 1, "X"]], segue: "Modalidade enquadrada" },
    { u: "SEHAB/TOPOGRAFIA", s: "Topografia e cartografia", a: "Elaborar o levantamento e o projeto de regularização.", p: 60, d: [["Levantamento topográfico", 1, "X"], ["Projeto de regularização fundiária", 1, "D", "C"]], segue: "Projeto concluído" },
    { u: "PGM", s: "Procuradoria", a: "Analisar a regularidade e notificar confrontantes.", p: 30, d: [["Parecer jurídico", 1, "D", "I"], ["Notificação aos confrontantes", 1, "D", "I"]], segue: "Sem impugnação", dil: ["Impugnação de confrontante", "Analisar a impugnação e ajustar o projeto", 2] },
    { u: "SEHAB/REURB", s: "Regularização fundiária", a: "Emitir a Certidão de Regularização Fundiária (CRF) e encaminhar ao cartório.", p: 15, d: [["Certidão de Regularização Fundiária (CRF)", 1, "D", "C"]] },
  ], { nivel: "RESTRITO", hipotese: "LAI, art. 31 (dados socioeconômicos do ocupante)" }),

  // ------------------------------------------------------------- Meio ambiente (8.0)
  P("MAM.LIC.001", "Licenciamento ambiental municipal", "Licenciar atividade de impacto ambiental local (licença prévia, de instalação e de operação).", "Empreendedores", "8.0.01.01.00", [
    { u: "SEMMA/LICENCIAMENTO", s: "Licenciamento ambiental", a: "Receber o requerimento e enquadrar a atividade e o porte.", p: 5, d: [["Requerimento de licença ambiental", 1, "X", "I"], ["Estudo ambiental exigido", 1, "X"], ["Certidão de uso do solo", 1, "X"]], segue: "Atividade enquadrada" },
    { u: "SEMMA/TECNICO", s: "Análise técnica", a: "Analisar os estudos e vistoriar o local.", p: 45, d: [["Parecer técnico", 1, "D", "C"], ["Relatório de vistoria", 1, "D", "I"]], segue: "Parecer favorável", dil: ["Estudo insuficiente", "Complementar o estudo ambiental", 1] },
    { u: "SEMMA/LICENCIAMENTO", s: "Licenciamento ambiental", a: "Emitir a licença com as condicionantes.", p: 5, d: [["Licença ambiental", 1, "D", "I"]] },
  ]),

  // ------------------------------------------------------------- Desenvolvimento econômico (9.0)
  P("DEC.USO.001", "Uso do solo para profissional liberal", "Emitir a certidão de uso do solo que autoriza o profissional liberal a exercer a atividade no endereço informado.", "Profissionais liberais", "9.0.01.01.04", [
    { u: "SEDEC/ATENDIMENTO", s: "Sala do Empreendedor", a: "Receber o requerimento e o comprovante de endereço.", p: 2, d: [["Requerimento de uso do solo para profissional liberal", 1, "X", "I"], ["Registro no conselho profissional", 1, "X"], ["Comprovante de endereço", 1, "X"]], segue: "Pedido instruído" },
    { u: "SEPLAN/URBANISMO", s: "Planejamento urbano", a: "Conferir o zoneamento do endereço para a atividade.", p: 10, d: [["Análise de zoneamento", 1, "D", "I"]], segue: "Uso permitido", dil: ["Uso não permitido no endereço", "Informar o requerente e indicar alternativas", 1] },
    { u: "SEDEC/ATENDIMENTO", s: "Sala do Empreendedor", a: "Emitir a certidão.", p: 2, d: [["Certidão de uso do solo para profissional liberal", 1, "D", "I"]] },
  ]),

  // ------------------------------------------------------------- Assistência social (10.0)
  P("SAS.OSC.001", "Inscrição de entidade (OSC) no Conselho de Assistência Social", "Inscrever a organização da sociedade civil no CMAS para atuar na política de assistência social.", "Organizações da sociedade civil", "10.0.02.00.00", [
    { u: "SEMPRAS/CMAS", s: "Secretaria executiva do CMAS", a: "Receber o pedido de inscrição com estatuto, atas e plano de ação.", p: 5, d: [["Requerimento de inscrição", 1, "X", "I"], ["Estatuto e ata de eleição da diretoria", 1, "X"], ["Plano de ação", 1, "X"]], segue: "Pedido instruído" },
    { u: "SEMPRAS/VIGILANCIA", s: "Vigilância socioassistencial", a: "Visitar a entidade e emitir o parecer técnico.", p: 30, d: [["Relatório de visita técnica", 1, "D", "C"]], segue: "Parecer emitido", dil: ["Documentação incompleta", "Complementar a documentação", 1] },
    { u: "SEMPRAS/CMAS", s: "Plenária do CMAS", a: "Deliberar a inscrição e publicar a resolução.", p: 30, d: [["Resolução do CMAS", 1, "D", "C"]] },
  ]),

  // ------------------------------------------------------------- Saúde (12.0)
  P("SAU.VSA.001", "Licença sanitária de estabelecimento", "Licenciar estabelecimento sujeito à vigilância sanitária, com inspeção (Código Sanitário Municipal).", "Estabelecimentos de interesse da saúde", "12.0.10.02.03", [
    { u: "SMS/VISA", s: "Vigilância sanitária", a: "Receber o pedido e cadastrar o estabelecimento.", p: 5, d: [["Requerimento de licença sanitária", 1, "X", "I"], ["Responsável técnico (quando exigido)", 0, "X"]], segue: "Cadastro feito" },
    { u: "SMS/VISA", s: "Inspeção sanitária", a: "Inspecionar o estabelecimento e lavrar o auto.", p: 30, d: [["Auto de inspeção", 1, "D", "C"]], segue: "Inspeção favorável", dil: ["Irregularidade sanitária", "Corrigir as exigências do auto de notificação e pedir nova inspeção", 1] },
    { u: "SMS/VISA", s: "Vigilância sanitária", a: "Emitir a licença (alvará) sanitária.", p: 5, d: [["Licença sanitária", 1, "D", "I"]] },
  ]),
  P("SAU.TFD.001", "Tratamento Fora do Domicílio (TFD)", "Autorizar o deslocamento do paciente do SUS para tratamento em outro município quando não houver oferta local.", "Usuários do SUS", "12.0.06.01.08", [
    { u: "SMS/TFD", s: "Setor de TFD", a: "Receber o laudo médico e o pedido do paciente.", p: 3, d: [["Laudo médico de TFD", 1, "X", "I"], ["Documentos pessoais e cartão SUS", 1, "X"]], segue: "Pedido instruído" },
    { u: "SMS/REGULACAO", s: "Complexo regulador", a: "Avaliar a indicação e agendar no município de referência.", p: 10, d: [["Parecer da regulação", 1, "D", "I"], ["Comprovante de agendamento", 1, "X"]], segue: "Agendado", dil: ["Laudo incompleto", "Complementar o laudo médico", 1] },
    { u: "SMS/TFD", s: "Setor de TFD", a: "Providenciar o transporte e a ajuda de custo e controlar o retorno.", p: 5, d: [["Lista de embarque", 1], ["Comprovante de ajuda de custo", 0]] },
  ], { nivel: "RESTRITO", hipotese: "LGPD, art. 11 (dado de saúde) e LAI, art. 31" }),
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
              documentos: e.d.map(([nome, obrig, f = "D", a = "INDIVIDUAL"]) => ({
                nome_documento: nome,
                obrigatorio: obrig === 1,
                formato: FORMATO[f],
                tipo_assinatura: ASSINATURA[a] ?? a,
              })),
            }
          : {}),
        ...(transicoes.length ? { transicoes } : {}),
      };
    }),
  })),
};

const destino = join(dirname(fileURLToPath(import.meta.url)), "rascunhos-procedimentos.json");
writeFileSync(destino, JSON.stringify(arquivo, null, 2) + "\n");
console.log(`${arquivo.procedimentos.length} rascunhos → ${destino}`);
