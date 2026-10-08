import type { Envelope, Signer } from "@/lib/nexus/types";

export const ENVELOPE_STATUS: Record<Envelope["status"], { label: string; tone: "warning" | "success" | "danger" | "neutral" }> = {
  pending: { label: "Aguardando assinaturas", tone: "warning" },
  completed: { label: "Concluído", tone: "success" },
  refused: { label: "Recusado", tone: "danger" },
  cancelled: { label: "Cancelado", tone: "neutral" },
};

export const SIGNER_STATUS: Record<Signer["status"], { label: string; tone: "warning" | "success" | "danger" }> = {
  pending: { label: "Pendente", tone: "warning" },
  signed: { label: "Assinado", tone: "success" },
  refused: { label: "Recusou", tone: "danger" },
};
