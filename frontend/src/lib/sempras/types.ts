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

export interface Prontuario {
  cidadao: Cidadao;
  atendimentos: Atendimento[];
  registros: Registro[];
}

export interface PainelUnidade {
  unidade: Unidade;
  aguardando: number;
  em_atendimento: number;
  concluidos: number;
  nao_compareceram: number;
  espera_media_min: number;
}

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

export const P = {
  recepcao: "sempras:recepcao",
  atendimento: "sempras:atendimento",
  psicologia: "sempras:psicologia",
  gestao: "sempras:gestao",
} as const;
