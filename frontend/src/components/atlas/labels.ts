import type {
  DestinacaoFinal,
  FormatoDocumento,
  NivelAcesso,
  OrgaoTTDD,
  TipoAssinatura,
} from "@/lib/nexus/types";

type Tone = "neutral" | "success" | "warning" | "danger" | "info";

export const NIVEL_ACESSO: Record<NivelAcesso, { label: string; tone: Tone }> = {
  PUBLICO: { label: "Público", tone: "neutral" },
  RESTRITO: { label: "Restrito", tone: "warning" },
  SIGILOSO: { label: "Sigiloso", tone: "danger" },
};

export const DESTINACAO: Record<DestinacaoFinal, { label: string; tone: Tone }> = {
  GUARDA_PERMANENTE: { label: "Guarda permanente", tone: "success" },
  ELIMINACAO: { label: "Eliminação", tone: "warning" },
};

export const FORMATO: Record<FormatoDocumento, string> = {
  NATO_DIGITAL: "Nato-digital",
  EXTERNO_DIGITALIZADO: "Externo digitalizado",
};

export const ASSINATURA: Record<TipoAssinatura, string> = {
  INDIVIDUAL: "Individual",
  CONJUNTA_MULTINIVEL: "Conjunta (multinível)",
  EM_BLOCO: "Em bloco",
};

/** Prazo de uma fase como no backend (domain.Fase): anos, condição ou ausência. */
export function fase(anos: number | null, condicao: string, corrente: boolean): string {
  if (anos !== null) return anos === 1 ? "1 ano" : `${anos} anos`;
  if (condicao) return condicao;
  return corrente ? "Não informado na TTDD" : "Não há";
}

/** Destinação final; null = a TTDD não define. */
export function destinacao(d: DestinacaoFinal | null): { label: string; tone: Tone } {
  return d ? DESTINACAO[d] : { label: "Não definida na TTDD", tone: "neutral" };
}

/** "TTDD da Secretaria X, versão II — Diário Oficial nº 6.275 de 11/09/2026". */
export function fonteTTDD(o: OrgaoTTDD): string {
  let s = `TTDD da ${o.nome}`;
  if (o.versao) s += `, versão ${o.versao}`;
  if (o.edicao_diario) {
    s += ` — Diário Oficial nº ${o.edicao_diario}`;
    if (o.data_publicacao)
      s += ` de ${new Date(o.data_publicacao).toLocaleDateString("pt-BR", { timeZone: "UTC" })}`;
  }
  return s;
}

/** "revogada em 10/05/2027 (Diário Oficial nº 6.500)"; null = vigente. */
export function revogacao(c: {
  revogada_em: string | null;
  revogada_edicao: string;
}): string | null {
  if (!c.revogada_em) return null;
  let s = `revogada em ${new Date(c.revogada_em).toLocaleDateString("pt-BR", { timeZone: "UTC" })}`;
  if (c.revogada_edicao) s += ` (Diário Oficial nº ${c.revogada_edicao})`;
  return s;
}
