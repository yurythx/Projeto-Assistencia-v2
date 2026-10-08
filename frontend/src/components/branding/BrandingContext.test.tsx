import { act, render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { BrandingProvider } from "./BrandingContext";
import { resetBrandingStore, updateBrandingStore } from "./brandingStore";

// Achado de auditoria original: branding.faviconUrl era um campo
// declarado desde sempre (tipo, default, persistido no cookie, campo no
// formulário de configurações) mas NADA o lia — o <link rel="icon">
// nunca refletia a personalização.
//
// Achado de uma 2ª rodada de verificação (confirmar que a troca é REAL,
// não só "o teste passa"): a 1ª correção criava um <link rel="icon">
// NOVO, concorrendo com os que app/icon.svg + app/favicon.ico do Next já
// renderizam no servidor — com três <link rel="icon"> na <head>, qual
// vence é uma heurística de cada navegador (o Chrome, por exemplo, tende
// a preferir um ícone SVG já existente independente de quando outro foi
// inserido), então a aba real podia continuar mostrando o ícone padrão
// mesmo com o <link> extra presente no DOM. A correção troca o href dos
// <link> que JÁ EXISTEM em vez de competir com eles — estes testes
// simulam esses <link> estáticos (como o Next de fato os renderiza)
// ANTES de montar o BrandingProvider, exatamente como uma página real.
function seedStaticIconLinks() {
  const ico = document.createElement("link");
  ico.rel = "icon";
  ico.href = "http://localhost:3000/favicon.ico";
  document.head.appendChild(ico);

  const svg = document.createElement("link");
  svg.rel = "icon";
  svg.href = "http://localhost:3000/icon.svg";
  document.head.appendChild(svg);

  const apple = document.createElement("link");
  apple.rel = "apple-touch-icon";
  apple.href = "http://localhost:3000/apple-icon.png";
  document.head.appendChild(apple);

  return { ico, svg, apple };
}

function allIconLinks(): HTMLLinkElement[] {
  return Array.from(
    document.querySelectorAll<HTMLLinkElement>('link[rel="icon"], link[rel="apple-touch-icon"]'),
  );
}

describe("BrandingProvider — favicon dinâmico (white-label)", () => {
  beforeEach(() => {
    seedStaticIconLinks();
  });

  afterEach(() => {
    act(() => resetBrandingStore());
    document
      .querySelectorAll('link[rel="icon"], link[rel="apple-touch-icon"]')
      .forEach((el) => el.remove());
  });

  it("troca o href dos <link> de ícone EXISTENTES (nunca cria um novo, concorrente)", () => {
    render(<BrandingProvider>{null}</BrandingProvider>);
    const before = allIconLinks();
    expect(before).toHaveLength(3); // ico + svg + apple-touch-icon, nenhum a mais ainda

    act(() => updateBrandingStore({ faviconUrl: "https://exemplo.gov.br/favicon.png" }));

    const after = allIconLinks();
    expect(after).toHaveLength(3); // mesma contagem — nada novo foi criado
    for (const link of after) {
      expect(link.href).toBe("https://exemplo.gov.br/favicon.png");
    }
  });

  it("rejeita esquemas perigosos (javascript:) — os <link> mantêm o href original", () => {
    render(<BrandingProvider>{null}</BrandingProvider>);

    act(() => updateBrandingStore({ faviconUrl: "javascript:alert(1)" }));

    for (const link of allIconLinks()) {
      expect(link.href).not.toContain("javascript:");
    }
  });

  it("limpar a URL restaura o href ORIGINAL de cada <link> (não fica preso no último valor customizado)", () => {
    render(<BrandingProvider>{null}</BrandingProvider>);
    const originalHrefs = allIconLinks().map((l) => l.href);

    act(() => updateBrandingStore({ faviconUrl: "https://exemplo.gov.br/favicon.png" }));
    for (const link of allIconLinks()) {
      expect(link.href).toBe("https://exemplo.gov.br/favicon.png");
    }

    act(() => updateBrandingStore({ faviconUrl: "" }));
    const restoredHrefs = allIconLinks().map((l) => l.href);
    expect(restoredHrefs).toEqual(originalHrefs);
  });
});
