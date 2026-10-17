// Casa Abrigo (ADR 032): tipos da API, rótulos e catálogos do assistente
// de 6 etapas, do PIA, das visitas, do diário, dos relatórios e do Kanban.
// O que o sistema da unidade (4.0) oferece como lista de escolha aqui é
// enum — o backend confere os mesmos valores.

import type { Registro } from "./types";

export type SimNao = "" | "sim" | "nao";
export type SimNaoNaoSei = "" | "sim" | "nao" | "nao_sei";

export const SIM_NAO: Record<Exclude<SimNao, "">, string> = { sim: "Sim", nao: "Não" };
export const SIM_NAO_NAO_SEI: Record<Exclude<SimNaoNaoSei, "">, string> = { sim: "Sim", nao: "Não", nao_sei: "Não sei" };

export type MotivoAcolhimento =
  | "abandono"
  | "abuso_sexual"
  | "ameaca_morte_responsaveis"
  | "ausencia_responsaveis_doenca"
  | "ausencia_responsaveis_prisao"
  | "conflito_familiar"
  | "entrega_voluntaria"
  | "exploracao_sexual"
  | "negligencia"
  | "orfandade"
  | "responsaveis_drogas"
  | "responsaveis_deficiencia"
  | "responsaveis_transtorno_mental"
  | "responsaveis_alcool"
  | "situacao_rua"
  | "trabalho_infantil"
  | "violencia_fisica"
  | "violencia_psicologica"
  | "outro";

export const MOTIVOS_ACOLHIMENTO: Record<MotivoAcolhimento, string> = {
  abandono: "Abandono",
  abuso_sexual: "Abuso sexual",
  ameaca_morte_responsaveis: "Ameaça de morte dos pais/responsáveis",
  ausencia_responsaveis_doenca: "Ausência dos pais/responsáveis por doença",
  ausencia_responsaveis_prisao: "Ausência dos pais/responsáveis por prisão/MSE",
  conflito_familiar: "Conflito familiar",
  entrega_voluntaria: "Entrega voluntária",
  exploracao_sexual: "Exploração sexual",
  negligencia: "Negligência",
  orfandade: "Orfandade",
  responsaveis_drogas: "Pais/responsáveis abusivos de drogas",
  responsaveis_deficiencia: "Pais/responsáveis com algum tipo de deficiência",
  responsaveis_transtorno_mental: "Pais/responsáveis com transtorno mental/psíquico",
  responsaveis_alcool: "Pais/responsáveis usuários abusivos de álcool",
  situacao_rua: "Situação de rua/mendicância",
  trabalho_infantil: "Trabalho infantil",
  violencia_fisica: "Violência física",
  violencia_psicologica: "Violência psicológica",
  outro: "Outro",
};

export type SituacaoJuridica =
  | ""
  | "emergencial_conselho_tutelar"
  | "adocao_internacional"
  | "adocao_nacional"
  | "processo_guarda"
  | "processo_tutela"
  | "poder_familiar_destituido"
  | "poder_familiar_suspenso"
  | "reintegracao_familia_extensa"
  | "reintegracao_familia_natural"
  | "sem_previsao";

export const SITUACOES_JURIDICAS: Record<Exclude<SituacaoJuridica, "">, string> = {
  emergencial_conselho_tutelar: "Acolhimento por situação emergencial pelo Conselho Tutelar",
  adocao_internacional: "Em processo de adoção internacional",
  adocao_nacional: "Em processo de adoção nacional",
  processo_guarda: "Em processo de guarda",
  processo_tutela: "Em processo de tutela",
  poder_familiar_destituido: "Poder familiar destituído",
  poder_familiar_suspenso: "Poder familiar suspenso",
  reintegracao_familia_extensa: "Preparação para reintegração familiar (família extensa)",
  reintegracao_familia_natural: "Preparação para reintegração familiar (família natural)",
  sem_previsao: "Sem previsão de finalização da medida protetiva",
};

export const VINCULOS_RESPONSAVEL: Record<string, string> = {
  mae: "Mãe",
  pai: "Pai",
  madrasta: "Madrasta",
  padrasto: "Padrasto",
  avo_feminino: "Avó",
  avo_masculino: "Avô",
  irma: "Irmã",
  irmao: "Irmão",
  tia: "Tia",
  tio: "Tio",
  prima: "Prima",
  primo: "Primo",
  familia_extensa: "Família extensa (outros)",
  familia_substituta: "Família substituta",
  instituicao: "Instituição",
  sem_parentesco: "Sem parentesco",
};

export const IDENTIDADES_GENERO: Record<string, string> = {
  cisgenero: "Cisgênero",
  transgenero: "Transgênero",
  nao_binario: "Não-binário",
  outro: "Outro",
};
export const ORIENTACOES_SEXUAIS: Record<string, string> = {
  heterossexual: "Heterossexual",
  homossexual: "Homossexual",
  bissexual: "Bissexual",
  pansexual: "Pansexual",
  outro: "Outro",
};
export const RACAS_CORES: Record<string, string> = {
  branca: "Branca",
  preta: "Preta",
  parda: "Parda",
  amarela: "Amarela",
  indigena: "Indígena",
  quilombola: "Quilombola",
};
export const RELACOES_TRABALHO: Record<string, string> = {
  jovem_aprendiz: "Jovem Aprendiz",
  meu_primeiro_trampo: "Meu Primeiro Trampo",
  formal: "Trabalho formal",
  informal: "Trabalho informal",
  nao: "Não",
};

export type TipoDesligamento =
  | "reintegracao_familia_natural"
  | "reintegracao_familia_extensa"
  | "adocao"
  | "guarda"
  | "transferencia"
  | "maioridade"
  | "evasao"
  | "outro";

export const TIPOS_DESLIGAMENTO: Record<TipoDesligamento, string> = {
  reintegracao_familia_natural: "Reintegração à família natural",
  reintegracao_familia_extensa: "Reintegração à família extensa",
  adocao: "Adoção",
  guarda: "Guarda",
  transferencia: "Transferência",
  maioridade: "Maioridade",
  evasao: "Evasão",
  outro: "Outro",
};

export type SexoAcolhido = "masculino" | "feminino";
export type StatusAcolhimento = "acolhido" | "desligado";
export type CategoriaDiagnostico = "doenca" | "deficiencia" | "transtorno" | "outro";

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

/** O checklist da etapa 5 do 4.0: cada item vira um diagnóstico com o nome
 * padronizado (R6). "Possui transtorno?" abre o grupo dos transtornos. */
export const CHECKLIST_SAUDE: { categoria: CategoriaDiagnostico; titulo: string; itens: string[] }[] = [
  {
    categoria: "doenca",
    titulo: "Doenças gerais e infecciosas",
    itens: ["Diabetes", "Hipertensão", "Síndrome Alcoólica Fetal", "Hidrocefalia", "HIV", "Hepatite", "Sífilis", "Tuberculose", "Outra(s) doença(s)"],
  },
  {
    categoria: "deficiencia",
    titulo: "Deficiências físicas e sensoriais",
    itens: [
      "Deficiência física leve",
      "Deficiência física moderada",
      "Deficiência física grave",
      "Deficiência física (cadeirante)",
      "Nanismo",
      "Deficiência física com ostomia",
      "Deficiência auditiva",
      "Surdez",
      "Deficiência visual",
      "Cegueira",
      "Surdocegueira",
    ],
  },
  {
    categoria: "transtorno",
    titulo: "Transtornos, síndromes e deficiência intelectual",
    itens: [
      "Deficiência intelectual leve",
      "Deficiência intelectual moderada",
      "Deficiência intelectual grave",
      "Síndrome de Down",
      "Deficiência psicossocial leve",
      "Deficiência psicossocial moderada",
      "Deficiência psicossocial grave",
      "TEA - Autismo (Grau 1)",
      "TEA - Autismo (Grau 2)",
      "TEA - Autismo (Grau 3)",
    ],
  },
];

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

/** Os campos do assistente de 6 etapas (os mesmos no cadastro e na edição). */
export interface AcolhimentoDados {
  sexo: SexoAcolhido;
  situacao_especial: boolean;
  pcd: boolean;
  apelido: string;
  identidade_genero: string;
  orientacao_sexual: string;
  raca_cor: string;
  filiacao_1: string;
  filiacao_2: string;
  responsavel_nao_possui: boolean;
  responsavel_nome: string;
  responsavel_nome_social: string;
  responsavel_apelido: string;
  responsavel_cpf: string;
  responsavel_nis: string;
  responsavel_nascimento: string | null;
  responsavel_sexo: string;
  responsavel_vinculo: string;
  responsavel_cep: string;
  responsavel_logradouro: string;
  responsavel_numero: string;
  responsavel_complemento: string;
  responsavel_bairro: string;
  responsavel_cidade: string;
  responsavel_uf: string;
  responsavel_telefone: string;
  responsavel_celular: string;
  responsavel_email: string;
  processo_judicial: string;
  processo_judicial_2: string;
  guia_acolhimento: string;
  data_guia: string | null;
  historico_acolhido_anteriormente: SimNaoNaoSei;
  historico_irmaos_nesta_unidade: SimNaoNaoSei;
  historico_irmaos_outra_unidade: SimNaoNaoSei;
  motivos: MotivoAcolhimento[];
  outro_motivo: string;
  situacao_juridica: SituacaoJuridica;
  data_situacao_juridica: string | null;
  conselheiro_nome: string;
  conselheiro_telefone: string;
  cna: SimNaoNaoSei;
  cncad: SimNaoNaoSei;
  laudo_medico: SimNao;
  laudo_medico_descricao: string;
  medicacao_continua: SimNao;
  dependencia_quimica: SimNao;
  possui_transtorno: SimNao;
  transtorno_descricao: string;
  estuda: SimNao;
  ameaca_morte: SimNao;
  recebe_visita: SimNao;
  beneficio_governo: SimNao;
  beneficio_qual: string;
  programa_social: SimNao;
  relacao_trabalho: string;
  observacoes_gerais: string;
}

export function dadosVazios(): AcolhimentoDados {
  return {
    sexo: "feminino", situacao_especial: false, pcd: false, apelido: "", identidade_genero: "", orientacao_sexual: "", raca_cor: "",
    filiacao_1: "", filiacao_2: "", responsavel_nao_possui: false, responsavel_nome: "", responsavel_nome_social: "",
    responsavel_apelido: "", responsavel_cpf: "", responsavel_nis: "", responsavel_nascimento: null, responsavel_sexo: "",
    responsavel_vinculo: "", responsavel_cep: "", responsavel_logradouro: "", responsavel_numero: "", responsavel_complemento: "",
    responsavel_bairro: "", responsavel_cidade: "", responsavel_uf: "", responsavel_telefone: "", responsavel_celular: "",
    responsavel_email: "", processo_judicial: "", processo_judicial_2: "", guia_acolhimento: "", data_guia: null,
    historico_acolhido_anteriormente: "", historico_irmaos_nesta_unidade: "", historico_irmaos_outra_unidade: "", motivos: [],
    outro_motivo: "", situacao_juridica: "", data_situacao_juridica: null, conselheiro_nome: "", conselheiro_telefone: "",
    cna: "", cncad: "", laudo_medico: "", laudo_medico_descricao: "", medicacao_continua: "", dependencia_quimica: "",
    possui_transtorno: "", transtorno_descricao: "", estuda: "", ameaca_morte: "", recebe_visita: "", beneficio_governo: "",
    beneficio_qual: "", programa_social: "", relacao_trabalho: "", observacoes_gerais: "",
  };
}

export interface Acolhimento extends AcolhimentoDados {
  id: string;
  cidadao_id: string;
  unidade_id: string;
  data_entrada: string;
  status: StatusAcolhimento;
  desligamento_tipo: TipoDesligamento | "";
  motivo_desligamento: string;
  guia_desligamento: string;
  data_guia_desligamento: string | null;
  registrado_por: string | null;
  desligado_por: string | null;
  data_desligamento: string | null;
  irmaos: Irmao[];
  diagnosticos: Diagnostico[];
  visitantes: Visitante[];
  termos: Termo[];
  pia: PiaFicha | null;
  pia_metas: PiaMeta[];
  documentos: DocumentoCasaAbrigo[];
}

export interface AcolhimentoDetalhe extends Acolhimento {
  cidadao: { id: string; nome: string; nome_social: string; data_nascimento: string; sexo: string; cpf: string; nis: string; nome_mae: string };
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

export type TipoVisita = "acolhido" | "instituicao" | "doacao" | "voluntariado";

export const TIPOS_VISITA: Record<TipoVisita, string> = {
  acolhido: "Visitar acolhido",
  instituicao: "Visitar instituição",
  doacao: "Doação",
  voluntariado: "Voluntariado",
};

export interface Visitante {
  id: string;
  unidade_id: string;
  acolhimento_id: string | null;
  tipo_visita: TipoVisita;
  nome_visitante: string;
  documento_identidade: string;
  parentesco_vinculo: string;
  data_visita: string;
  hora_entrada: string;
  hora_saida: string;
  observacoes: string;
  acolhido_nome?: string;
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
  acolhido_nome?: string;
  created_at: string;
}

export const TIPOS_TERMO: Record<TipoTermo, string> = {
  advertencia: "Advertência",
  passeio: "Passeio",
  saida: "Saída",
  outro: "Outro",
};

/** Título do documento impresso de cada termo. */
export const TITULOS_TERMO: Record<TipoTermo, string> = {
  advertencia: "Termo de Advertência",
  passeio: "Termo de Liberação para Passeio",
  saida: "Termo de Autorização de Saída",
  outro: "Termo",
};

export interface PiaFichaDados {
  naturalidade: string;
  nacionalidade: string;
  religiao: string;
  doc_carteira_vacinacao: boolean;
  doc_certidao_nascimento: boolean;
  doc_rg_cpf: boolean;
  doc_cartao_sus: boolean;
  doc_nis: boolean;
  sinais_violencia_fisica: boolean;
  medicamentos: string;
  alergias: string;
  vacinacao_atualizada: boolean;
  cartao_sus: string;
  tratamento_odontologico: boolean;
  tratamento_medico_psicologico: boolean;
  deficiencias_restricoes: string;
  escola: string;
  serie_escolar: string;
  matricula_escolar: string;
  periodo_escolar: string;
  frequencia_escolar: string;
  aula_reforco: boolean;
  historico_familiar: string;
  possui_beneficios: boolean;
  moradia_anterior: string;
  contato_familiar: string;
  visitas_autorizadas: string;
  observacoes_familia: string;
  atividades_manha: string;
  atividades_tarde: string;
  atividades_noite: string;
  atividades_fim_semana: string;
  observacoes_comportamentais: string;
  intervencao_objetivos: string;
  intervencao_estrategias: string;
  intervencao_metas_prazos: string;
}

export function piaVazia(): PiaFichaDados {
  return {
    naturalidade: "", nacionalidade: "", religiao: "", doc_carteira_vacinacao: false, doc_certidao_nascimento: false,
    doc_rg_cpf: false, doc_cartao_sus: false, doc_nis: false, sinais_violencia_fisica: false, medicamentos: "", alergias: "",
    vacinacao_atualizada: false, cartao_sus: "", tratamento_odontologico: false, tratamento_medico_psicologico: false,
    deficiencias_restricoes: "", escola: "", serie_escolar: "", matricula_escolar: "", periodo_escolar: "", frequencia_escolar: "",
    aula_reforco: false, historico_familiar: "", possui_beneficios: false, moradia_anterior: "", contato_familiar: "",
    visitas_autorizadas: "", observacoes_familia: "", atividades_manha: "", atividades_tarde: "", atividades_noite: "",
    atividades_fim_semana: "", observacoes_comportamentais: "", intervencao_objetivos: "", intervencao_estrategias: "",
    intervencao_metas_prazos: "",
  };
}

export interface PiaFicha extends PiaFichaDados {
  acolhimento_id: string;
  atualizado_por: string | null;
  atualizado_em: string;
}

export const PERIODOS_ESCOLARES: Record<string, string> = {
  matutino: "Matutino",
  vespertino: "Vespertino",
  noturno: "Noturno",
  integral: "Integral",
};
export const MORADIAS_ANTERIORES: Record<string, string> = {
  propria: "Própria",
  alugada: "Alugada",
  cedida: "Cedida",
  situacao_rua: "Situação de rua",
};

export type TipoPiaMeta = "plano" | "intervencao";

export interface PiaMeta {
  id: string;
  acolhimento_id: string;
  tipo: TipoPiaMeta;
  descricao: string;
  providencia: string;
  responsavel: string;
  prazo: string | null;
  esportes: string;
  cultura: string;
  lazer: string;
  concluida: boolean;
  created_at: string;
}

export const TIPOS_PIA_META: Record<TipoPiaMeta, string> = {
  plano: "Plano de ação",
  intervencao: "Intervenção familiar",
};

export interface AcolhidoResumo {
  acolhimento_id: string;
  cidadao_id: string;
  nome: string;
  nome_social: string;
  data_nascimento: string;
  idade: number;
  sexo: SexoAcolhido;
  data_entrada: string;
  processo_judicial: string;
  guia_acolhimento: string;
  situacao_juridica: SituacaoJuridica;
  motivos: MotivoAcolhimento[];
  status: StatusAcolhimento;
  desligamento_tipo: TipoDesligamento | "";
  data_desligamento: string | null;
  pia_atualizado_em: string | null;
  proxima_reavaliacao_pia: string;
  documento_acolhido: boolean;
  documento_responsavel: boolean;
}

export type TipoAlerta = "pia" | "meta" | "relatorio" | "tarefa";

export interface Alerta {
  tipo: TipoAlerta;
  nivel: "aviso" | "atraso";
  mensagem: string;
  prazo: string;
  dias: number;
  acolhimento_id?: string | null;
  ref_id: string;
}

export interface PainelCasaAbrigo {
  criancas: number;
  adolescentes: number;
  ativos: number;
  pias_vencendo: number;
  desacolhidos: number;
  acolhidos: AcolhidoResumo[];
  desligados: AcolhidoResumo[];
  alertas: Alerta[];
}

export interface EstatisticasCasaAbrigo {
  de: string;
  ate: string;
  entradas: number;
  saidas: number;
  reintegracoes: number;
  adocoes: number;
  transferencias: number;
  saidas_por_tipo: Partial<Record<TipoDesligamento, number>>;
  motivos: Partial<Record<MotivoAcolhimento, number>>;
  ativos_no_fim: number;
  criancas: number;
  adolescentes: number;
  meninos: number;
  meninas: number;
}

export interface EvolucaoAcolhido extends Registro {
  acolhido_nome: string;
}

export interface AnotacaoDiario {
  id: string;
  unidade_id: string;
  acolhimento_id: string | null;
  data_referencia: string;
  texto: string;
  autor_nome: string;
  acolhido_nome?: string;
  created_at: string;
}

export type TipoRelatorio =
  | "cinco_dias"
  | "sete_dias"
  | "quinzenal"
  | "mensal"
  | "bimestral"
  | "trimestral"
  | "anual"
  | "psicossocial"
  | "circunstanciado"
  | "informativo"
  | "outro";

export const TIPOS_RELATORIO: Record<TipoRelatorio, string> = {
  cinco_dias: "Relatório para 5 dias",
  sete_dias: "Relatório para 7 dias",
  quinzenal: "Relatório quinzenal",
  mensal: "Relatório mensal",
  bimestral: "Relatório bimestral",
  trimestral: "Relatório trimestral",
  anual: "Relatório anual",
  psicossocial: "Relatório psicossocial",
  circunstanciado: "Relatório circunstanciado",
  informativo: "Relatório informativo",
  outro: "Outro (especificar)",
};

export type StatusRelatorio = "em_elaboracao" | "revisao" | "aprovado" | "entregue";

export const STATUS_RELATORIO: Record<StatusRelatorio, { label: string; tone: "warning" | "info" | "success" | "neutral" }> = {
  em_elaboracao: { label: "Em elaboração", tone: "warning" },
  revisao: { label: "Em revisão", tone: "info" },
  aprovado: { label: "Aprovado", tone: "success" },
  entregue: { label: "Entregue", tone: "neutral" },
};

export interface RelatorioCasaAbrigo {
  id: string;
  unidade_id: string;
  acolhimento_id: string | null;
  tipo: TipoRelatorio;
  tipo_outro: string;
  status: StatusRelatorio;
  data_entrega: string;
  conteudo: string;
  acolhido_nome?: string;
  created_at: string;
  atualizado_em: string;
}

/** Nome do relatório como aparece na lista e no editor. */
export function nomeRelatorio(r: Pick<RelatorioCasaAbrigo, "tipo" | "tipo_outro">): string {
  return r.tipo === "outro" ? r.tipo_outro : TIPOS_RELATORIO[r.tipo];
}

export type StatusTarefa = "backlog" | "todo" | "doing" | "review" | "done";
export type PrioridadeTarefa = "baixa" | "media" | "alta" | "critica";

export const COLUNAS_KANBAN: Record<StatusTarefa, string> = {
  backlog: "Backlog",
  todo: "A fazer",
  doing: "Em andamento",
  review: "Em revisão",
  done: "Concluído",
};

export const PRIORIDADES_TAREFA: Record<PrioridadeTarefa, { label: string; tone: "neutral" | "info" | "warning" | "danger" }> = {
  baixa: { label: "Baixa", tone: "neutral" },
  media: { label: "Média", tone: "info" },
  alta: { label: "Alta", tone: "warning" },
  critica: { label: "Crítica", tone: "danger" },
};

export interface TarefaCasaAbrigo {
  id: string;
  unidade_id: string;
  titulo: string;
  descricao: string;
  status: StatusTarefa;
  prioridade: PrioridadeTarefa;
  responsavel: string;
  sprint: string;
  prazo: string | null;
  ordem: number;
  created_at: string;
  atualizado_em: string;
}

// ----------------------------------------------------------------- helpers

/** Idade em anos completos numa data de referência (nascimento AAAA-MM-DD). */
export function idadeEm(nascimento: string, ref: Date = new Date()): number | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(nascimento);
  if (!m) return null;
  const [ano, mes, dia] = [Number(m[1]), Number(m[2]), Number(m[3])];
  let idade = ref.getFullYear() - ano;
  if (ref.getMonth() + 1 < mes || (ref.getMonth() + 1 === mes && ref.getDate() < dia)) idade--;
  return Math.max(0, idade);
}

/** Iniciais do nome ("Ana Maria da Silva" → "A. M. S."), para listas e
 * avisos sob sigilo (ECA art. 143), como o 4.0. */
export function iniciais(nome: string): string {
  const partes = nome
    .trim()
    .split(/\s+/)
    .filter((p) => p && !["de", "da", "do", "das", "dos", "e"].includes(p.toLowerCase()))
    .map((p) => `${p.charAt(0).toUpperCase()}.`);
  return partes.length ? partes.join(" ") : "N/I";
}

/** Data "pura" (AAAA-MM-DD) para o formato que o backend espera (meia-noite UTC). */
export function dataISO(data: string): string {
  return `${data}T00:00:00Z`;
}

/** O dia (AAAA-MM-DD) de uma data vinda do backend. As datas "puras" são
 * gravadas como meia-noite UTC; a API pode devolvê-las no fuso dela
 * (ex.: 20h da véspera em -04:00), então o dia é lido em UTC — o mesmo
 * critério do backend (diaCivil). */
export function diaDe(iso: string | null | undefined): string {
  if (!iso) return "";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso.slice(0, 10) : d.toISOString().slice(0, 10);
}

/** Formata AAAA-MM-DD em DD/MM/AAAA. */
export function fmtDia(iso: string | null | undefined): string {
  const d = diaDe(iso);
  return /^\d{4}-\d{2}-\d{2}$/.test(d) ? `${d.slice(8, 10)}/${d.slice(5, 7)}/${d.slice(0, 4)}` : "—";
}

/** Hoje em AAAA-MM-DD (fuso do navegador). */
export function hojeISO(agora: Date = new Date()): string {
  const p = (n: number) => String(n).padStart(2, "0");
  return `${agora.getFullYear()}-${p(agora.getMonth() + 1)}-${p(agora.getDate())}`;
}

/** Agora em HH:MM (entrada/saída de visitante). */
export function horaAgora(agora: Date = new Date()): string {
  return `${String(agora.getHours()).padStart(2, "0")}:${String(agora.getMinutes()).padStart(2, "0")}`;
}

/** Máscaras do 4.0 (só formatação; o backend guarda só os dígitos). */
export function mascaraCPF(v: string): string {
  return v
    .replace(/\D/g, "")
    .slice(0, 11)
    .replace(/(\d{3})(\d)/, "$1.$2")
    .replace(/(\d{3})(\d)/, "$1.$2")
    .replace(/(\d{3})(\d{1,2})$/, "$1-$2");
}

export function mascaraCEP(v: string): string {
  return v.replace(/\D/g, "").slice(0, 8).replace(/^(\d{5})(\d)/, "$1-$2");
}

export function mascaraTelefone(v: string): string {
  return v
    .replace(/\D/g, "")
    .slice(0, 11)
    .replace(/^(\d{2})(\d)/, "($1) $2")
    .replace(/(\d)(\d{4})$/, "$1-$2");
}

/** Baixa um arquivo gerado no navegador (CSV, TXT, DOC). */
export function baixarArquivo(nome: string, conteudo: string, tipo: string): void {
  const blob = new Blob(["﻿" + conteudo], { type: tipo });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = nome;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

/** Uma linha de CSV com ";" (Excel em pt-BR), com aspas quando precisa. */
export function linhaCSV(campos: (string | number)[]): string {
  return campos.map((c) => `"${String(c).replace(/"/g, '""')}"`).join(";");
}
