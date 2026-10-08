import type { ClassificacaoTTDD } from "@/lib/nexus/types";

type Prazos = Pick<
  ClassificacaoTTDD,
  "fase_corrente_anos" | "fase_corrente_condicao" | "fase_interm_anos" | "fase_interm_condicao"
>;

/** Resultado da calculadora: as datas, ou o motivo de não haver cálculo. */
export type Calculo =
  | { possivel: true; fimCorrente: Date; fimIntermediaria: Date; semIntermediaria: boolean }
  | { possivel: false; motivo: string };

/** O que a data de referência significa para a série: com prazo corrente
 * por condição ("Enquanto estiver vigorando"), a contagem começa quando a
 * condição deixa de valer; com prazo em anos, no encerramento do documento. */
export function rotuloReferencia(c: Prazos): string {
  return c.fase_corrente_condicao
    ? "Data em que deixou de vigorar"
    : "Data de encerramento ou arquivamento do documento";
}

/** Soma anos sem escorregar de 29/02 para março. */
export function somarAnos(d: Date, anos: number): Date {
  const r = new Date(Date.UTC(d.getUTCFullYear() + anos, d.getUTCMonth(), 1));
  const ultimoDia = new Date(Date.UTC(r.getUTCFullYear(), r.getUTCMonth() + 1, 0)).getUTCDate();
  r.setUTCDate(Math.min(d.getUTCDate(), ultimoDia));
  return r;
}

/** Estima quando cada fase termina e a partir de quando cabe a destinação
 * final. Só calcula o que a TTDD permite: prazo em anos ou fase corrente
 * por condição (a referência é o fim da condição); fase intermediária por
 * condição ou fase corrente não informada não têm data. */
export function calcularDestinacao(c: Prazos, referencia: Date): Calculo {
  let fimCorrente: Date;
  if (c.fase_corrente_anos !== null) fimCorrente = somarAnos(referencia, c.fase_corrente_anos);
  else if (c.fase_corrente_condicao) fimCorrente = referencia;
  else
    return { possivel: false, motivo: "A TTDD não informa o prazo da fase corrente desta série." };

  if (c.fase_interm_anos !== null) {
    return {
      possivel: true,
      fimCorrente,
      fimIntermediaria: somarAnos(fimCorrente, c.fase_interm_anos),
      semIntermediaria: false,
    };
  }
  if (c.fase_interm_condicao) {
    return {
      possivel: false,
      motivo: `A fase intermediária depende de condição ("${c.fase_interm_condicao}") — não há data fixa.`,
    };
  }
  return { possivel: true, fimCorrente, fimIntermediaria: fimCorrente, semIntermediaria: true };
}

export const dataBR = (d: Date) => d.toLocaleDateString("pt-BR", { timeZone: "UTC" });
