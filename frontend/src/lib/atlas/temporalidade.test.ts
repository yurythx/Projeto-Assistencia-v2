import { describe, expect, it } from "vitest";

import { calcularDestinacao, dataBR, rotuloReferencia, somarAnos } from "./temporalidade";

const base = {
  fase_corrente_anos: null,
  fase_corrente_condicao: "",
  fase_interm_anos: null,
  fase_interm_condicao: "",
};
const ref = new Date(Date.UTC(2026, 9, 3)); // 03/10/2026

describe("calculadora de temporalidade", () => {
  it("anos + anos: fim de cada fase", () => {
    const r = calcularDestinacao({ ...base, fase_corrente_anos: 2, fase_interm_anos: 3 }, ref);
    expect(r.possivel).toBe(true);
    if (r.possivel) {
      expect(dataBR(r.fimCorrente)).toBe("03/10/2028");
      expect(dataBR(r.fimIntermediaria)).toBe("03/10/2031");
      expect(r.semIntermediaria).toBe(false);
    }
  });

  it("sem fase intermediária: destinação ao fim da corrente", () => {
    const r = calcularDestinacao({ ...base, fase_corrente_anos: 1 }, ref);
    expect(r.possivel && r.semIntermediaria && dataBR(r.fimIntermediaria)).toBe("03/10/2027");
  });

  it("corrente por condição: a referência é o fim da condição", () => {
    const c = {
      ...base,
      fase_corrente_condicao: "Enquanto estiver vigorando",
      fase_interm_anos: 5,
    };
    expect(rotuloReferencia(c)).toBe("Data em que deixou de vigorar");
    const r = calcularDestinacao(c, ref);
    expect(r.possivel && dataBR(r.fimCorrente)).toBe("03/10/2026");
    expect(r.possivel && dataBR(r.fimIntermediaria)).toBe("03/10/2031");
  });

  it("sem cálculo quando a TTDD não permite", () => {
    expect(calcularDestinacao(base, ref)).toMatchObject({
      possivel: false,
      motivo: expect.stringMatching(/não informa/),
    });
    expect(
      calcularDestinacao(
        { ...base, fase_corrente_anos: 1, fase_interm_condicao: "30 dias após a data do evento" },
        ref,
      ),
    ).toMatchObject({ possivel: false, motivo: expect.stringMatching(/30 dias após/) });
    expect(rotuloReferencia({ ...base, fase_corrente_anos: 1 })).toMatch(/encerramento/);
  });

  it("29/02 + 1 ano vira 28/02, não 01/03", () => {
    expect(dataBR(somarAnos(new Date(Date.UTC(2028, 1, 29)), 1))).toBe("28/02/2029");
  });
});
