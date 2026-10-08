import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Seal } from "./Seal";

describe("Seal", () => {
  it("por padrão é acessível: role=img com aria-label", () => {
    const { container } = render(<Seal />);
    const svg = container.querySelector("svg")!;
    expect(svg).toHaveAttribute("role", "img");
    expect(svg).toHaveAttribute("aria-label", "Selo do Projeto Nexus");
  });

  it("decorative=true remove da árvore de acessibilidade", () => {
    const { container } = render(<Seal decorative />);
    const svg = container.querySelector("svg")!;
    expect(svg).toHaveAttribute("aria-hidden", "true");
    expect(svg).not.toHaveAttribute("role");
  });

  it("renderiza os dizeres curvos passados", () => {
    const { getByText } = render(
      <Seal topText="TEXTO DE CIMA" bottomText="TEXTO DE BAIXO" />,
    );
    expect(getByText("TEXTO DE CIMA")).toBeInTheDocument();
    expect(getByText("TEXTO DE BAIXO")).toBeInTheDocument();
  });

  it("aplica o tamanho recebido", () => {
    const { container } = render(<Seal size={120} />);
    const svg = container.querySelector("svg")!;
    expect(svg).toHaveAttribute("width", "120");
    expect(svg).toHaveAttribute("height", "120");
  });
});
