import { describe, expect, it } from "vitest";

import type { OrgTree, Scope } from "@/lib/nexus/types";

import { departamentosGeridos, hasPermission, unidadesGeridas, unidadesLotadas } from "./permissions";

const un = (id: string, entidade_id: string, parent_id?: string) => ({
  id,
  entidade_id,
  parent_id,
  nome: id,
  sigla: "",
  slug: id,
  ad_group: "",
  email: "",
  telefone: "",
  endereco: "",
  ativo: true,
  departamentos: [{ id: `d-${id}`, unidade_id: id, nome: `Depto ${id}` }],
});
const TREE = [
  {
    id: "E",
    nome: "E",
    ativo: true,
    unidades: [un("A", "E"), un("SUB", "E", "A"), un("NETA", "E", "SUB"), un("B", "E")],
  },
  { id: "X", nome: "X", ativo: true, unidades: [un("W", "X")] },
] as unknown as OrgTree[];
const scope = (s: Partial<Scope>): Scope => ({
  perfil: "p",
  origem: "manual",
  permissions: ["catalog:manage"],
  ...s,
});
const ids = (r: { unidades: Set<string> }) => [...r.unidades].sort();

describe("unidadesGeridas (ADR 013)", () => {
  it("unidade cobre ela e as subunidades, não as irmãs", () => {
    const r = unidadesGeridas(
      { roles: [], scopes: [scope({ entidade_id: "E", unidade_id: "A" })] },
      TREE,
      "catalog:manage",
    );
    expect(r.global).toBe(false);
    expect(ids(r)).toEqual(["A", "NETA", "SUB"]);
  });
  it("entidade cobre todas as unidades dela; departamento não cobre unidade inteira", () => {
    const r = unidadesGeridas(
      {
        roles: [],
        scopes: [
          scope({ entidade_id: "X" }),
          scope({ entidade_id: "E", unidade_id: "B", departamento_id: "D" }),
        ],
      },
      TREE,
      "catalog:manage",
    );
    expect(ids(r)).toEqual(["W"]);
  });
  it("global e nexus-admin cobrem tudo (e o institucional)", () => {
    expect(unidadesGeridas({ roles: [], scopes: [scope({})] }, TREE, "catalog:manage").global).toBe(
      true,
    );
    const admin = unidadesGeridas({ roles: ["nexus-admin"], scopes: [] }, TREE, "catalog:manage");
    expect(admin.global).toBe(true);
    expect(admin.unidades.size).toBe(5);
  });
  it("só conta concessões com a permissão; sem dados, nada", () => {
    expect(
      ids(
        unidadesGeridas(
          { roles: [], scopes: [scope({ unidade_id: "A", permissions: ["blog:manage"] })] },
          TREE,
          "catalog:manage",
        ),
      ),
    ).toEqual([]);
    expect(unidadesGeridas(undefined, undefined, "catalog:manage")).toEqual({
      global: false,
      unidades: new Set(),
    });
  });
  it("curingas", () => {
    expect(hasPermission(["catalog:*"], "catalog:manage")).toBe(true);
    expect(hasPermission(undefined, "x:y")).toBe(false);
  });
});

describe("departamentosGeridos (ADR 013)", () => {
  it("departamentos das unidades cobertas mais os concedidos diretamente", () => {
    const me = {
      roles: [],
      scopes: [
        scope({ entidade_id: "E", unidade_id: "SUB", permissions: ["mercurio:manage"] }),
        scope({
          entidade_id: "E",
          unidade_id: "B",
          departamento_id: "d-B",
          permissions: ["mercurio:manage"],
        }),
      ],
    };
    const r = departamentosGeridos(me, TREE, "mercurio:manage");
    expect(r.global).toBe(false);
    expect([...r.departamentos].sort()).toEqual(["d-B", "d-NETA", "d-SUB"]);
  });
  it("global cobre todos", () => {
    const r = departamentosGeridos({ roles: ["nexus-admin"], scopes: [] }, TREE, "mercurio:manage");
    expect(r.global).toBe(true);
    expect(r.departamentos.size).toBe(5);
  });
});

describe("unidadesLotadas (ADR 013)", () => {
  it("unidade com as subunidades, entidade inteira, só a unidade do departamento; global não é lotação", () => {
    const r = (scopes: Scope[]) => [...unidadesLotadas({ scopes }, TREE)].sort();
    expect(r([scope({ entidade_id: "E", unidade_id: "SUB", permissions: [] })])).toEqual(["NETA", "SUB"]);
    expect(r([scope({ entidade_id: "X", permissions: [] })])).toEqual(["W"]);
    expect(r([scope({ entidade_id: "E", unidade_id: "A", departamento_id: "d-A" })])).toEqual(["A"]);
    expect(r([scope({ departamento_id: "inexistente" }), scope({})])).toEqual([]);
    expect([...unidadesLotadas(undefined, undefined)]).toEqual([]);
  });
});
