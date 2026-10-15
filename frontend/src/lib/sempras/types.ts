// Tipos e rótulos do SEMPRAS (espelho dos DTOs de
// backend/internal/modules/sempras/domain — ADR 030).

export interface Unidade {
  id: string;
  entidade_id: string;
  nome: string;
  sigla: string;
  ativa: boolean;
}

export interface Resumo {
  id: string;
  nome: string;
  nome_social: string;
  cpf: string;
  nis: string;
  data_nascimento: string;
  atendimentos_na_area: number;
}

export interface Cidadao {
  id: string;
  nome: string;
  nome_social: string;
  cpf: string;
  nis: string;
  data_nascimento: string;
  nome_mae: string;
  telefone: string;
  endereco: string;
}

export type Status = "aguardando" | "chamado" | "em_atendimento" | "concluido" | "nao_compareceu" | "cancelado";

export interface Atendimento {
  id: string;
  cidadao_id: string;
  unidade_id: string;
  dia: string;
  senha: number;
  tipo: "social" | "psicologico";
  prioridade: string;
  status: Status;
  local: string;
  created_at: string;
  chamado_em: string | null;
  iniciado_em: string | null;
  encerrado_em: string | null;
}

export interface FilaItem extends Atendimento {
  senha_exibida: string;
  nome: string;
}

export interface Registro {
  id: string;
  unidade_id: string;
  atendimento_id: string | null;
  tipo: "social" | "psicologico";
  texto: string;
  sigiloso: boolean;
  autor_nome: string;
  created_at: string;
}

export interface ContatoRedeApoio {
  id: string;
  cidadao_id: string;
  nome: string;
  parentesco: string;
  telefone: string;
  created_at: string;
}

export type TipoBeneficio = "natalidade" | "funeral" | "cesta_basica" | "passagem" | "aluguel_social" | "outro";
export type StatusBeneficio = "solicitado" | "concedido" | "negado" | "suspenso";
export type FormaAcessoBeneficio = "demanda_espontanea" | "busca_ativa" | "encaminhamento_rede_socioassistencial" | "encaminhamento_sistema_garantia_direitos";

export interface Beneficio {
  id: string;
  beneficiario_id: string;
  solicitante_id: string | null;
  unidade_id: string;
  tipo: TipoBeneficio;
  forma_acesso: FormaAcessoBeneficio;
  quantidade: number;
  status: StatusBeneficio;
  observacao: string;
  motivo_decisao: string;
  tecnico_decisao_id: string | null;
  data_solicitacao: string;
  data_decisao: string | null;
}

export const TIPOS_BENEFICIO: Record<TipoBeneficio, string> = {
  natalidade: "Auxílio natalidade",
  funeral: "Auxílio funeral",
  cesta_basica: "Cesta básica",
  passagem: "Passagem",
  aluguel_social: "Aluguel social",
  outro: "Outro",
};

export const FORMAS_ACESSO_BENEFICIO: Record<FormaAcessoBeneficio, string> = {
  demanda_espontanea: "Demanda espontânea",
  busca_ativa: "Busca ativa",
  encaminhamento_rede_socioassistencial: "Encaminhamento da rede socioassistencial",
  encaminhamento_sistema_garantia_direitos: "Encaminhamento do sistema de garantia de direitos",
};

export const STATUS_BENEFICIO: Record<StatusBeneficio, { label: string; tone: "warning" | "success" | "danger" | "neutral" }> = {
  solicitado: { label: "Solicitado", tone: "warning" },
  concedido: { label: "Concedido", tone: "success" },
  negado: { label: "Negado", tone: "danger" },
  suspenso: { label: "Suspenso", tone: "neutral" },
};

export type StatusEncaminhamento = "pendente" | "aceito" | "recusado";

export interface Encaminhamento {
  id: string;
  cidadao_id: string;
  unidade_origem_id: string;
  unidade_destino_id: string;
  motivo: string;
  status: StatusEncaminhamento;
  observacao_decisao: string;
  encaminhado_por: string | null;
  decidido_por: string | null;
  created_at: string;
  decidido_em: string | null;
}

export const STATUS_ENCAMINHAMENTO: Record<StatusEncaminhamento, { label: string; tone: "warning" | "success" | "danger" }> = {
  pendente: { label: "Pendente", tone: "warning" },
  aceito: { label: "Aceito", tone: "success" },
  recusado: { label: "Recusado", tone: "danger" },
};

export type StatusPaif = "ativo" | "excluido";
export type TipoAtendimentoPaif = "particularizado" | "coletivo" | "ambos";

export const TIPOS_ATENDIMENTO_PAIF: Record<TipoAtendimentoPaif, string> = {
  particularizado: "Particularizado",
  coletivo: "Coletivo",
  ambos: "Ambos",
};

export interface PaifVinculo {
  id: string;
  cidadao_id: string;
  unidade_id: string;
  status: StatusPaif;
  tipo_atendimento: TipoAtendimentoPaif;
  extrema_pobreza: boolean;
  bolsa_familia: boolean;
  bpc: boolean;
  scfv_participantes: number;
  motivo_exclusao: string;
  incluido_por: string | null;
  excluido_por: string | null;
  data_inclusao: string;
  data_exclusao: string | null;
}

export const STATUS_PAIF: Record<StatusPaif, { label: string; tone: "success" | "neutral" }> = {
  ativo: { label: "Ativo", tone: "success" },
  excluido: { label: "Excluído", tone: "neutral" },
};

export type OrigemDemandaConselho =
  | "disque_100"
  | "escola"
  | "unidade_saude"
  | "plantao_24h"
  | "demanda_espontanea"
  | "policia";

export type MotivoViolacaoConselho =
  | "negligencia_abandono"
  | "violencia_fisica"
  | "abuso_exploracao_sexual"
  | "evasao_escolar"
  | "trabalho_infantil"
  | "conflito_familiar";

export type MedidaProtetiva =
  | ""
  | "termo_responsabilidade"
  | "inclusao_programa_social"
  | "matricula_escolar_obrigatoria"
  | "tratamento_medico_psicologico"
  | "acolhimento_emergencial";

export type StatusConselho = "em_andamento" | "medida_aplicada" | "encaminhado_mp" | "arquivado";

export interface ConselhoAtendimento {
  id: string;
  protocolo: string;
  crianca_id: string;
  unidade_id: string;
  responsavel_nome: string;
  responsavel_cpf: string;
  responsavel_telefone: string;
  origem_demanda: OrigemDemandaConselho;
  motivo_violacao: MotivoViolacaoConselho;
  medida_protetiva: MedidaProtetiva;
  status: StatusConselho;
  observacoes: string;
  conselheiro_id: string | null;
  created_at: string;
  atualizado_em: string | null;
}

export const ORIGENS_DEMANDA: Record<OrigemDemandaConselho, string> = {
  disque_100: "Disque 100",
  escola: "Escola",
  unidade_saude: "Unidade de saúde",
  plantao_24h: "Plantão 24h",
  demanda_espontanea: "Demanda espontânea",
  policia: "Polícia",
};

export const MOTIVOS_VIOLACAO: Record<MotivoViolacaoConselho, string> = {
  negligencia_abandono: "Negligência/abandono",
  violencia_fisica: "Violência física",
  abuso_exploracao_sexual: "Abuso/exploração sexual",
  evasao_escolar: "Evasão escolar",
  trabalho_infantil: "Trabalho infantil",
  conflito_familiar: "Conflito familiar",
};

/** Medidas protetivas do Art. 101 do ECA. */
export const MEDIDAS_PROTETIVAS: Record<Exclude<MedidaProtetiva, "">, string> = {
  termo_responsabilidade: "Termo de responsabilidade",
  inclusao_programa_social: "Inclusão em programa social",
  matricula_escolar_obrigatoria: "Matrícula escolar obrigatória",
  tratamento_medico_psicologico: "Tratamento médico/psicológico",
  acolhimento_emergencial: "Acolhimento emergencial",
};

export const STATUS_CONSELHO: Record<StatusConselho, { label: string; tone: "warning" | "success" | "danger" | "neutral" }> = {
  em_andamento: { label: "Em andamento", tone: "warning" },
  medida_aplicada: { label: "Medida aplicada", tone: "success" },
  encaminhado_mp: { label: "Encaminhado ao MP", tone: "danger" },
  arquivado: { label: "Arquivado", tone: "neutral" },
};

// -------------------------------------------------------------------- PAEFI

export type ViolacaoPaefi =
  | "violencia_fisica"
  | "violencia_psicologica"
  | "violencia_sexual"
  | "negligencia"
  | "abandono"
  | "trabalho_infantil"
  | "discriminacao"
  | "medida_socioeducativa_meio_aberto";

export type StatusPaefi = "em_acompanhamento" | "desligado";

export type MotivoEncerramentoPaefi = "" | "objetivos_alcancados" | "mudanca_territorio" | "nao_localizado" | "a_pedido_da_familia" | "outro";

export interface PaefiCaso {
  id: string;
  caso_codigo: string;
  cidadao_id: string;
  unidade_id: string;
  violacoes: ViolacaoPaefi[];
  membros_vulneraveis: string;
  pia_elaborado: boolean;
  status: StatusPaefi;
  motivo_encerramento: MotivoEncerramentoPaefi;
  tecnico_referencia_id: string | null;
  observacoes: string;
  data_abertura: string;
  data_encerramento: string | null;
}

/** Tipificação Nacional de Serviços Socioassistenciais (Resolução CNAS nº 109/2009). */
export const VIOLACOES_PAEFI: Record<ViolacaoPaefi, string> = {
  violencia_fisica: "Violência física",
  violencia_psicologica: "Violência psicológica",
  violencia_sexual: "Violência sexual",
  negligencia: "Negligência",
  abandono: "Abandono",
  trabalho_infantil: "Trabalho infantil",
  discriminacao: "Discriminação",
  medida_socioeducativa_meio_aberto: "Medida socioeducativa em meio aberto",
};

export const STATUS_PAEFI: Record<StatusPaefi, { label: string; tone: "warning" | "neutral" }> = {
  em_acompanhamento: { label: "Em acompanhamento", tone: "warning" },
  desligado: { label: "Desligado", tone: "neutral" },
};

export const MOTIVOS_ENCERRAMENTO_PAEFI: Record<Exclude<MotivoEncerramentoPaefi, "">, string> = {
  objetivos_alcancados: "Objetivos alcançados",
  mudanca_territorio: "Mudança de território",
  nao_localizado: "Não localizado",
  a_pedido_da_familia: "A pedido da família",
  outro: "Outro",
};

// ---------------------------------------------------------------------- MSE

export type TipoMse = "liberdade_assistida" | "prestacao_servico_comunidade";
export type StatusMse = "em_cumprimento" | "cumprida" | "descumprida" | "extinta";

export interface MseMedida {
  id: string;
  adolescente_id: string;
  unidade_id: string;
  numero_processo: string;
  vara_origem: string;
  ato_infracional: string;
  tipo_medida: TipoMse;
  instituicao_parceira: string;
  status: StatusMse;
  tecnico_referencia_id: string | null;
  observacoes: string;
  data_inicio: string;
  prazo_previsto: string | null;
  data_conclusao: string | null;
}

/** Medidas socioeducativas em meio aberto (ECA art. 112, III e VI). */
export const TIPOS_MSE: Record<TipoMse, string> = {
  liberdade_assistida: "Liberdade Assistida (LA)",
  prestacao_servico_comunidade: "Prestação de Serviços à Comunidade (PSC)",
};

export const STATUS_MSE: Record<StatusMse, { label: string; tone: "warning" | "success" | "danger" | "neutral" }> = {
  em_cumprimento: { label: "Em cumprimento", tone: "warning" },
  cumprida: { label: "Cumprida", tone: "success" },
  descumprida: { label: "Descumprida", tone: "danger" },
  extinta: { label: "Extinta", tone: "neutral" },
};

// ------------------------------------------------------ notificações (CREAS)

export type OrigemNotificacaoCreas = "ministerio_publico" | "poder_judiciario" | "conselho_tutelar" | "disque_100_180" | "delegacia" | "outro";
export type StatusNotificacaoCreas = "recebida" | "em_diligencia" | "respondida" | "arquivada";

export interface NotificacaoCreas {
  id: string;
  protocolo: string;
  vitima_id: string;
  unidade_id: string;
  origem_entidade: OrigemNotificacaoCreas;
  assunto: string;
  status: StatusNotificacaoCreas;
  resposta: string;
  tecnico_referencia_id: string | null;
  data_entrada: string;
  prazo_resposta: string | null;
  data_resposta: string | null;
}

export const ORIGENS_NOTIFICACAO_CREAS: Record<OrigemNotificacaoCreas, string> = {
  ministerio_publico: "Ministério Público",
  poder_judiciario: "Poder Judiciário",
  conselho_tutelar: "Conselho Tutelar",
  disque_100_180: "Disque 100/180",
  delegacia: "Delegacia",
  outro: "Outro",
};

export const STATUS_NOTIFICACAO_CREAS: Record<StatusNotificacaoCreas, { label: string; tone: "warning" | "success" | "danger" | "neutral" }> = {
  recebida: { label: "Recebida", tone: "warning" },
  em_diligencia: { label: "Em diligência", tone: "warning" },
  respondida: { label: "Respondida", tone: "success" },
  arquivada: { label: "Arquivada", tone: "neutral" },
};

// ------------------------------------------------------------- Ofícios (CRAS)

export type StatusOficioCras = "pendente" | "respondido";

export interface OficioCras {
  id: string;
  cidadao_id: string;
  unidade_id: string;
  orgao_emissor: string;
  numero_oficio: string;
  data_oficio: string | null;
  data_recebimento: string;
  assunto: string;
  status: StatusOficioCras;
  numero_resposta: string;
  data_resposta: string | null;
  tecnico_responsavel_id: string | null;
}

export const STATUS_OFICIO_CRAS: Record<StatusOficioCras, { label: string; tone: "warning" | "success" | "danger" | "neutral" }> = {
  pendente: { label: "Pendente", tone: "warning" },
  respondido: { label: "Respondido", tone: "success" },
};

// ---------------------------------------------------- Atendimentos coletivos

export type CategoriaColetivo = "paif_familias" | "scfv_0_6" | "scfv_7_14" | "scfv_15_17" | "scfv_18_59" | "scfv_idosos"
  | "palestras_nao_continuado" | "pcd";

export interface AtendimentoColetivo {
  id: string;
  unidade_id: string;
  data_evento: string;
  categoria: CategoriaColetivo;
  participantes: number;
  descricao: string;
  created_at: string;
}

export const CATEGORIAS_COLETIVO: Record<CategoriaColetivo, string> = {
  paif_familias: "Famílias em grupos do PAIF",
  scfv_0_6: "SCFV — crianças até 6 anos",
  scfv_7_14: "SCFV — crianças/adolescentes de 7 a 14 anos",
  scfv_15_17: "SCFV — adolescentes de 15 a 17 anos",
  scfv_18_59: "SCFV — adultos de 18 a 59 anos",
  scfv_idosos: "SCFV — idosos",
  palestras_nao_continuado: "Palestras e atividades não continuadas",
  pcd: "Pessoas com deficiência",
};

// ---------------------------------------------------------------- Casa Abrigo

export type MotivoAcolhimento =
  | "negligencia_abandono"
  | "violencia_fisica"
  | "violencia_psicologica"
  | "abuso_sexual"
  | "orfandade_ausencia_responsavel"
  | "usuario_drogas_responsavel"
  | "situacao_rua"
  | "trabalho_infantil"
  | "outro";

export type SexoAcolhido = "masculino" | "feminino";
export type StatusAcolhimento = "acolhido" | "desligado";
export type CategoriaDiagnostico = "doenca" | "deficiencia" | "transtorno" | "outro";

export interface Irmao {
  id: string;
  acolhimento_id: string;
  nome: string;
  data_nascimento: string | null;
  esta_acolhido: boolean;
  unidade_descricao: string;
  observacao: string;
  created_at: string;
}

export interface Diagnostico {
  id: string;
  acolhimento_id: string;
  categoria: CategoriaDiagnostico;
  diagnostico: string;
  descricao: string;
  created_at: string;
}

export interface Acolhimento {
  id: string;
  cidadao_id: string;
  unidade_id: string;
  sexo: SexoAcolhido;
  responsavel_nome: string;
  responsavel_nis: string;
  responsavel_telefone: string;
  responsavel_vinculo: string;
  responsavel_endereco: string;
  motivos: MotivoAcolhimento[];
  processo_judicial: string;
  guia_acolhimento: string;
  conselheiro_nome: string;
  conselheiro_telefone: string;
  situacao_juridica: string;
  historico_acolhido_anteriormente: boolean;
  historico_irmaos_outra_unidade: boolean;
  observacoes_gerais: string;
  status: StatusAcolhimento;
  motivo_desligamento: string;
  desligado_por: string | null;
  data_entrada: string;
  data_desligamento: string | null;
  irmaos: Irmao[];
  diagnosticos: Diagnostico[];
  visitantes: Visitante[];
  termos: Termo[];
  pia: PiaFicha | null;
  pia_metas: PiaMeta[];
  documentos: DocumentoCasaAbrigo[];
}

export type TipoDocumentoCasaAbrigo = "acolhido" | "responsavel";
export type StatusDocumentoCasaAbrigo = "pending" | "ready";

export interface DocumentoCasaAbrigo {
  id: string;
  acolhimento_id: string;
  tipo: TipoDocumentoCasaAbrigo;
  nome_original: string;
  object_key: string;
  content_type: string;
  size_bytes: number;
  status: StatusDocumentoCasaAbrigo;
  created_at: string;
}

export const TIPOS_DOCUMENTO_CASA_ABRIGO: Record<TipoDocumentoCasaAbrigo, string> = {
  acolhido: "Do acolhido",
  responsavel: "Do responsável",
};

export interface Visitante {
  id: string;
  acolhimento_id: string;
  nome_visitante: string;
  documento_identidade: string;
  parentesco_vinculo: string;
  data_visita: string;
  hora_entrada: string;
  hora_saida: string;
  observacoes: string;
  created_at: string;
}

export type TipoTermo = "advertencia" | "passeio" | "saida" | "outro";

export interface Termo {
  id: string;
  acolhimento_id: string;
  tipo_termo: TipoTermo;
  responsavel_retirada: string;
  parentesco_responsavel: string;
  motivo_observacao: string;
  data_termo: string;
  created_at: string;
}

export const TIPOS_TERMO: Record<TipoTermo, string> = {
  advertencia: "Advertência",
  passeio: "Passeio",
  saida: "Saída",
  outro: "Outro",
};

export interface PiaFicha {
  acolhimento_id: string;
  matricula_escolar: string;
  serie_escolar: string;
  escola: string;
  medicamentos: string;
  alergias: string;
  vacinacao_atualizada: boolean;
  cartao_sus: string;
  contato_familiar: string;
  visitas_autorizadas: string;
  historico_familiar: string;
  atividades_manha: string;
  atividades_tarde: string;
  atividades_noite: string;
  atividades_fim_semana: string;
  atualizado_por: string | null;
  atualizado_em: string;
}

export type TipoPiaMeta = "plano" | "intervencao";

export interface PiaMeta {
  id: string;
  acolhimento_id: string;
  tipo: TipoPiaMeta;
  descricao: string;
  providencia: string;
  responsavel: string;
  prazo: string | null;
  concluida: boolean;
  created_at: string;
}

export const TIPOS_PIA_META: Record<TipoPiaMeta, string> = {
  plano: "Plano",
  intervencao: "Intervenção",
};

export const MOTIVOS_ACOLHIMENTO: Record<MotivoAcolhimento, string> = {
  negligencia_abandono: "Negligência/abandono",
  violencia_fisica: "Violência física",
  violencia_psicologica: "Violência psicológica",
  abuso_sexual: "Abuso sexual",
  orfandade_ausencia_responsavel: "Orfandade/ausência de responsável",
  usuario_drogas_responsavel: "Responsável usuário de drogas",
  situacao_rua: "Situação de rua",
  trabalho_infantil: "Trabalho infantil",
  outro: "Outro",
};

export const STATUS_ACOLHIMENTO: Record<StatusAcolhimento, { label: string; tone: "warning" | "neutral" }> = {
  acolhido: { label: "Acolhido", tone: "warning" },
  desligado: { label: "Desligado", tone: "neutral" },
};

export const CATEGORIAS_DIAGNOSTICO: Record<CategoriaDiagnostico, string> = {
  doenca: "Doença",
  deficiencia: "Deficiência",
  transtorno: "Transtorno",
  outro: "Outro",
};

export interface Prontuario {
  cidadao: Cidadao;
  atendimentos: Atendimento[];
  registros: Registro[];
  rede_apoio: ContatoRedeApoio[];
  beneficios: Beneficio[];
  encaminhamentos: Encaminhamento[];
  paif_vinculos: PaifVinculo[];
  conselho_atendimentos: ConselhoAtendimento[];
  paefi_casos: PaefiCaso[];
  mse_medidas: MseMedida[];
  notificacoes_creas: NotificacaoCreas[];
  oficios_cras: OficioCras[];
  casa_abrigo_acolhimentos: Acolhimento[];
}

export interface PainelUnidade {
  unidade: Unidade;
  aguardando: number;
  em_atendimento: number;
  concluidos: number;
  nao_compareceram: number;
  espera_media_min: number;
}

/** RMA/MDS: Relatório Mensal de Atendimento de uma unidade numa competência. */
export interface RmaUnidade {
  unidade: Unidade;
  atendimentos: number;
  atendimentos_sociais: number;
  atendimentos_psicologicos: number;
  paif_ativos: number;
  paif_extrema_pobreza: number;
  paif_bolsa_familia: number;
  paif_bpc: number;
  paif_scfv_participantes: number;
  beneficios_concedidos_no_mes: number;
  paefi_casos_ativos: number;
  paefi_casos_abertos_no_mes: number;
  mse_ativas: number;
  mse_concluidas_no_mes: number;
  conselho_atendimentos_no_mes: number;
  creas_notificacoes_recebidas_no_mes: number;
  creas_notificacoes_respondidas_no_mes: number;
  casa_abrigo_acolhidos_ativos: number;
  casa_abrigo_entradas_no_mes: number;
  casa_abrigo_desligamentos_no_mes: number;
  encaminhamentos_enviados_no_mes: number;
  oficios_cras_recebidos_no_mes: number;
  oficios_cras_respondidos_no_mes: number;
  coletivo_paif_familias_no_mes: number;
  coletivo_scfv_0_6_no_mes: number;
  coletivo_scfv_7_14_no_mes: number;
  coletivo_scfv_15_17_no_mes: number;
  coletivo_scfv_18_59_no_mes: number;
  coletivo_scfv_idosos_no_mes: number;
  coletivo_palestras_no_mes: number;
  coletivo_pcd_no_mes: number;
  coletivo_total_no_mes: number;
}

// -------------------------------------------------- direitos do titular

export type TipoPedido = "acesso" | "correcao" | "anonimizacao";
export type StatusPedido = "pendente" | "concluido" | "negado";

export interface PedidoTitular {
  id: string;
  cidadao_id: string;
  cidadao_nome: string;
  tipo: TipoPedido;
  status: StatusPedido;
  justificativa_solicitacao: string;
  solicitado_por: string;
  solicitado_por_nome: string;
  justificativa_decisao: string;
  decidido_por: string | null;
  decidido_por_nome: string;
  decidido_em: string | null;
  created_at: string;
}

export const STATUS_PEDIDO: Record<StatusPedido, { label: string; tone: "warning" | "success" | "danger" }> = {
  pendente: { label: "Pendente", tone: "warning" },
  concluido: { label: "Concluído", tone: "success" },
  negado: { label: "Negado", tone: "danger" },
};

export const TIPOS_PEDIDO: Record<TipoPedido, string> = {
  acesso: "Acesso aos dados",
  correcao: "Correção de cadastro",
  anonimizacao: "Anonimização",
};

export const STATUS: Record<Status, { label: string; tone: "neutral" | "success" | "danger" | "warning" | "info" }> = {
  aguardando: { label: "Aguardando", tone: "warning" },
  chamado: { label: "Chamado", tone: "info" },
  em_atendimento: { label: "Em atendimento", tone: "info" },
  concluido: { label: "Concluído", tone: "success" },
  nao_compareceu: { label: "Não compareceu", tone: "danger" },
  cancelado: { label: "Cancelado", tone: "neutral" },
};

export const TIPOS = [
  { value: "social", label: "Atendimento social" },
  { value: "psicologico", label: "Atendimento psicológico" },
];

/** Prioridades legais (Lei 10.048/2000, Lei 12.764/2012, Lei 13.466/2017). */
export const PRIORIDADES = [
  { value: "", label: "Sem prioridade" },
  { value: "idoso_80", label: "Idoso 80+" },
  { value: "idoso", label: "Idoso 60+" },
  { value: "deficiencia", label: "Pessoa com deficiência" },
  { value: "tea", label: "Pessoa com TEA" },
  { value: "gestante", label: "Gestante" },
  { value: "lactante", label: "Lactante" },
  { value: "crianca_de_colo", label: "Com criança de colo" },
  { value: "obeso", label: "Pessoa com obesidade" },
];

export function rotuloPrioridade(valor: string): string {
  return PRIORIDADES.find((p) => p.value === valor)?.label ?? valor;
}

/** Campos editáveis do cadastro — cadastro novo (Recepcao) e correção
 * (ficha do munícipe) usam a mesma lista. */
export const CAMPOS_CIDADAO: { name: keyof Cidadao; label: string; type?: string; max: number }[] = [
  { name: "nome", label: "Nome civil *", max: 200 },
  { name: "nome_social", label: "Nome social", max: 200 },
  { name: "cpf", label: "CPF", max: 20 },
  { name: "nis", label: "NIS", max: 20 },
  { name: "data_nascimento", label: "Data de nascimento", type: "date", max: 10 },
  { name: "nome_mae", label: "Nome da mãe", max: 200 },
  { name: "telefone", label: "Telefone", max: 30 },
  { name: "endereco", label: "Endereço", max: 300 },
];

export const P = {
  recepcao: "sempras:recepcao",
  atendimento: "sempras:atendimento",
  psicologia: "sempras:psicologia",
  gestao: "sempras:gestao",
} as const;
