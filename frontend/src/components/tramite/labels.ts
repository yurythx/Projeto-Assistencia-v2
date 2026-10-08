import type { Documento, Processo } from "@/lib/nexus/types";

type Tone = "neutral" | "success" | "warning" | "danger" | "info";

export const SIGILO: Record<Processo["sigilo"], { label: string; tone: Tone }> = {
  publico: { label: "Público", tone: "neutral" },
  restrito: { label: "Restrito", tone: "warning" },
  sigiloso: { label: "Sigiloso", tone: "danger" },
};

export const PROCESSO_STATUS: Record<Processo["status"], { label: string; tone: Tone }> = {
  aberto: { label: "Aberto", tone: "info" },
  em_tramitacao: { label: "Em tramitação", tone: "warning" },
  concluido: { label: "Concluído", tone: "success" },
  arquivado: { label: "Arquivado", tone: "neutral" },
};

export const DOC_STATUS: Record<Documento["status"], { label: string; tone: Tone }> = {
  rascunho: { label: "Rascunho", tone: "neutral" },
  aguardando_assinatura: { label: "Aguardando assinatura", tone: "warning" },
  assinado: { label: "Assinado", tone: "success" },
  cancelado: { label: "Cancelado", tone: "danger" },
};
