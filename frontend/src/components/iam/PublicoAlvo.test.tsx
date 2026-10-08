import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import type { OrgTree, Publico } from "@/lib/nexus/types";

const tree = [
  {
    id: "e1", nome: "Secretaria de Saúde", sigla: "SEMSA", ativo: true,
    unidades: [
      { id: "u1", entidade_id: "e1", nome: "UPA", sigla: "", departamentos: [] },
      { id: "u2", entidade_id: "e1", nome: "PSF Conjunto", sigla: "PSF-CJ", departamentos: [] },
    ],
  },
  { id: "e2", nome: "Educação", sigla: "", ativo: true, unidades: [] },
] as unknown as OrgTree[];
vi.mock("@/lib/api/swr", () => ({ useApiQuery: (key: string | null) => ({ data: key ? tree : undefined }) }));

import { PublicoAlvoPicker, PublicoBadge, TODOS, nomesDoPublico, publicoVazio } from "./PublicoAlvo";

function Harness({ onChange }: { onChange: (p: Publico) => void }) {
  const [v, setV] = useState<Publico>(TODOS);
  return (
    <PublicoAlvoPicker
      idPrefix="t"
      value={v}
      onChange={(p) => {
        setV(p);
        onChange(p);
      }}
    />
  );
}

describe("Público-alvo (ADR 014)", () => {
  it("marca a secretaria inteira e unidades; desmarcar tira; nada marcado = todos", async () => {
    const onChange = vi.fn();
    render(<Harness onChange={onChange} />);
    expect(screen.getByText("Todos os servidores (sem restrição).")).toBeInTheDocument();

    await userEvent.click(screen.getAllByLabelText("Toda a secretaria")[0]!);
    await userEvent.click(screen.getByLabelText("PSF-CJ — PSF Conjunto"));
    expect(onChange).toHaveBeenLastCalledWith({ entidades: ["e1"], unidades: ["u2"] });
    expect(screen.getByText(/Só para: SEMSA, PSF-CJ/)).toBeInTheDocument();

    await userEvent.click(screen.getAllByLabelText("Toda a secretaria")[0]!);
    expect(onChange).toHaveBeenLastCalledWith({ entidades: [], unidades: ["u2"] });
  });

  it("selo: nomes do público; sem público, nada", () => {
    const { container, rerender } = render(<PublicoBadge publico={{ entidades: ["e2"], unidades: ["u1"] }} />);
    expect(screen.getByText("Para: UPA, Educação")).toBeInTheDocument();
    rerender(<PublicoBadge publico={{ entidades: ["sumiu"], unidades: [] }} />);
    expect(screen.getByText("Público restrito")).toBeInTheDocument();
    rerender(<PublicoBadge publico={TODOS} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("auxiliares", () => {
    expect(publicoVazio(undefined)).toBe(true);
    expect(publicoVazio({ entidades: [], unidades: ["u1"] })).toBe(false);
    expect(nomesDoPublico(null, tree)).toEqual([]);
    expect(nomesDoPublico({ entidades: ["e1"], unidades: [] }, undefined)).toEqual([]);
  });
});
