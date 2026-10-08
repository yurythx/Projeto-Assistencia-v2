import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { Me, OrgTree, ScopeRef } from "@/lib/nexus/types";

const un = (id: string, deps: string[]) => ({
  id, entidade_id: "e1", nome: `Unidade ${id}`, sigla: "", slug: "", ad_group: "", email: "", telefone: "", endereco: "", ativo: true,
  departamentos: deps.map((d) => ({ id: d, unidade_id: id, nome: `Depto ${d}`, sigla: "", slug: "", ad_group: "", email: "", telefone: "", ativo: true })),
});
const tree: OrgTree[] = [
  {
    id: "e1", nome: "Secretaria", sigla: "SEC", slug: "sec", documento: "", ativo: true,
    unidades: [
      { ...un("u1", ["d1"]), nome: "Protocolo", sigla: "PRT", departamentos: [{ ...un("u1", ["d1"]).departamentos[0]!, nome: "Triagem" }] },
      un("u2", ["d2", "d3"]),
    ],
  },
  { id: "e2", nome: "Outra", sigla: "", slug: "outra", documento: "", ativo: true, unidades: [{ ...un("u3", []), entidade_id: "e2" }] },
];
let me: Pick<Me, "roles" | "scopes"> | undefined;
vi.mock("@/lib/api/swr", () => ({ useApiQuery: () => ({ data: tree }) }));
vi.mock("@/lib/nexus/NexusProvider", () => ({ useNexus: () => ({ me }) }));

import { ScopePicker } from "./ScopePicker";

function Harness({ onChange, permission }: { onChange: (v: ScopeRef) => void; permission?: string }) {
  const [v, setV] = useState<ScopeRef>({});
  return (
    <ScopePicker
      idPrefix="t"
      value={v}
      permission={permission}
      onChange={(n) => {
        setV(n);
        onChange(n);
      }}
    />
  );
}

const opcoes = (label: string) => within(screen.getByLabelText(label)).getAllByRole("option").map((o) => o.textContent);

describe("ScopePicker", () => {
  beforeEach(() => {
    me = undefined;
  });

  it("cascata: unidade e departamento só habilitam após o nível acima; trocar o nível acima limpa os de baixo", async () => {
    const onChange = vi.fn();
    render(<Harness onChange={onChange} />);
    expect(screen.getByLabelText("Unidade")).toBeDisabled();
    expect(screen.getByLabelText("Departamento")).toBeDisabled();

    await userEvent.selectOptions(screen.getByLabelText("Entidade"), "e1");
    await userEvent.selectOptions(screen.getByLabelText("Unidade"), "u1");
    await userEvent.selectOptions(screen.getByLabelText("Departamento"), "d1");
    expect(onChange).toHaveBeenLastCalledWith({ entidade_id: "e1", unidade_id: "u1", departamento_id: "d1" });

    await userEvent.selectOptions(screen.getByLabelText("Entidade"), "");
    expect(onChange).toHaveBeenLastCalledWith({ entidade_id: undefined });
    expect(screen.getByLabelText("Unidade")).toBeDisabled();
  });

  it("administração global: tudo, inclusive o escopo global", () => {
    me = { roles: ["nexus-admin"], scopes: [] };
    render(<Harness onChange={vi.fn()} permission="iam:manage" />);
    expect(opcoes("Entidade")).toEqual(["Todas (global)", "SEC — Secretaria", "Outra"]);
  });

  it("delegada na unidade (ADR 013): só a entidade e a unidade dela; sem global nem entidade inteira", async () => {
    me = { roles: [], scopes: [{ perfil: "p", origem: "manual", entidade_id: "e1", unidade_id: "u1", permissions: ["iam:manage"] }] };
    render(<Harness onChange={vi.fn()} permission="iam:manage" />);
    expect(screen.getByLabelText("Entidade *")).toBeRequired();
    expect(opcoes("Entidade *")).toEqual(["Selecione…", "SEC — Secretaria"]);
    await userEvent.selectOptions(screen.getByLabelText("Entidade *"), "e1");
    expect(screen.getByLabelText("Unidade *")).toBeRequired();
    expect(opcoes("Unidade *")).toEqual(["Selecione…", "PRT — Protocolo"]);
    await userEvent.selectOptions(screen.getByLabelText("Unidade *"), "u1");
    expect(screen.getByLabelText("Departamento")).not.toBeRequired();
  });

  it("delegada no departamento: a unidade dele aparece, e o departamento é obrigatório", async () => {
    me = { roles: [], scopes: [{ perfil: "p", origem: "manual", entidade_id: "e1", unidade_id: "u2", departamento_id: "d2", permissions: ["iam:manage"] }] };
    render(<Harness onChange={vi.fn()} permission="iam:manage" />);
    await userEvent.selectOptions(screen.getByLabelText("Entidade *"), "e1");
    expect(opcoes("Unidade *")).toEqual(["Selecione…", "Unidade u2"]);
    await userEvent.selectOptions(screen.getByLabelText("Unidade *"), "u2");
    expect(screen.getByLabelText("Departamento *")).toBeRequired();
    expect(opcoes("Departamento *")).toEqual(["Selecione…", "Depto d2"]);
  });

  it("delegada na entidade: a unidade é opcional (escopo da entidade)", async () => {
    me = { roles: [], scopes: [{ perfil: "p", origem: "manual", entidade_id: "e2", permissions: ["iam:manage"] }] };
    render(<Harness onChange={vi.fn()} permission="iam:manage" />);
    expect(opcoes("Entidade *")).toEqual(["Selecione…", "Outra"]);
    await userEvent.selectOptions(screen.getByLabelText("Entidade *"), "e2");
    expect(screen.getByLabelText("Unidade")).not.toBeRequired();
  });
});
