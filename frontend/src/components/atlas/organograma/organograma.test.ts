import { describe, expect, it } from "vitest";

import type { SubfuncaoOrganograma } from "@/lib/nexus/types";

import {
  cobertura,
  detalheProcedimentos,
  faixaCobertura,
  nomeCurto,
  rotulo,
  squarify,
  visaoValida,
} from "./organograma";

const sub = (publicados: number, rascunhos = 0): SubfuncaoOrganograma => ({
  codigo: "x",
  nome: "x",
  series: 1,
  series_de_processo: 0,
  publicados,
  em_validacao: 0,
  rascunhos,
  lacunas: 0,
});

describe("organograma: regras", () => {
  it("rótulos e detalhe: rascunhos só para a gestão; singular e plural", () => {
    const c = { series: 1, publicados: 1, em_validacao: 2, rascunhos: 1, lacunas: 0 };
    expect(rotulo(c, true)).toBe("1 série · 4 procedimentos");
    expect(rotulo({ ...c, series: 3 }, false)).toBe("3 séries · 1 procedimento");
    expect(detalheProcedimentos(c, true)).toBe("1 publicado · 2 em validação · 1 rascunho");
    expect(detalheProcedimentos({ ...c, publicados: 0, rascunhos: 2 }, true)).toBe(
      "0 publicados · 2 em validação · 2 rascunhos",
    );
    expect(detalheProcedimentos(c, false)).toBe("1 publicado");
  });

  it("cobertura pelas subfunções com procedimento e faixas de cor", () => {
    expect(cobertura([], true)).toBe(0);
    expect(cobertura([sub(0, 1), sub(0)], true)).toBe(0.5);
    expect(cobertura([sub(0, 1), sub(0)], false)).toBe(0);
    expect([0, 0.2, 0.5, 1].map(faixaCobertura)).toEqual([0, 1, 2, 3]);
  });

  it("visão padrão, nomes curtos", () => {
    expect(visaoValida("blocos")).toBe("blocos");
    expect(visaoValida("outra")).toBe("caixas");
    expect(visaoValida(null)).toBe("caixas");
    expect(nomeCurto("Secretaria Municipal de Saúde")).toBe("Saúde");
    expect(nomeCurto("Secretaria Municipal da Fazenda")).toBe("Fazenda");
    expect(nomeCurto("PROCON")).toBe("PROCON");
  });

  it("squarify: áreas proporcionais, dentro do retângulo, sem sobreposição", () => {
    const valores = [60, 30, 25, 10, 5, 0];
    const r = { x: 0, y: 0, w: 100, h: 60 };
    const out = squarify(
      valores.map((v, i) => ({ valor: v, item: i })),
      r,
    );
    expect(out.map((o) => o.item)).toEqual([0, 1, 2, 3, 4]); // o zero sai
    const total = 130;
    for (const o of out) {
      expect(o.w * o.h).toBeCloseTo((valores[o.item]! / total) * 6000, 6);
      expect(o.x).toBeGreaterThanOrEqual(-1e-9);
      expect(o.y).toBeGreaterThanOrEqual(-1e-9);
      expect(o.x + o.w).toBeLessThanOrEqual(100 + 1e-9);
      expect(o.y + o.h).toBeLessThanOrEqual(60 + 1e-9);
    }
    for (const a of out)
      for (const b of out) {
        if (a === b) continue;
        const sobrepoe =
          a.x + 1e-9 < b.x + b.w &&
          b.x + 1e-9 < a.x + a.w &&
          a.y + 1e-9 < b.y + b.h &&
          b.y + 1e-9 < a.y + a.h;
        expect(sobrepoe).toBe(false);
      }
    // Retângulo em pé: a primeira linha vai no topo.
    const alto = squarify(
      [
        { valor: 1, item: "a" },
        { valor: 1, item: "b" },
      ],
      { x: 0, y: 0, w: 10, h: 40 },
    );
    expect(alto.map((o) => o.y)).toEqual([0, 20]);
    expect(squarify([{ valor: 0, item: 1 }], r)).toEqual([]);
  });
});
