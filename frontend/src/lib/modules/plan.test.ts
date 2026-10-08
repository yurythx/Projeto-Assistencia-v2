import { describe, expect, it } from "vitest";

import type { ModuleStatus } from "@/lib/nexus/types";

import { moduleNames, planToggle } from "./plan";

function mod(key: string, extra: Partial<ModuleStatus> = {}): ModuleStatus {
  return {
    key, name: key.toUpperCase(), description: "", core: false, default_enabled: true, depends_on: [], permissions: [],
    public: false, icon: "box", route: `/${key}`, enabled: true, configured: true, dependents: [], blocked_by: [], ...extra,
  };
}

// cc <- bb <- aa (aa depende de bb, que depende de cc)
const graph = () => [
  mod("iam", { core: true }),
  mod("cc", { dependents: ["bb"] }),
  mod("bb", { depends_on: ["cc"], dependents: ["aa"] }),
  mod("aa", { depends_on: ["bb"] }),
];

describe("planToggle", () => {
  it("desativar desliga antes os dependentes, de cima para baixo", () => {
    const { steps, error } = planToggle(graph(), "cc", false);
    expect(error).toBeUndefined();
    expect(steps.map((s) => s.key)).toEqual(["aa", "bb", "cc"]);
    expect(steps.every((s) => !s.enabled)).toBe(true);
  });

  it("dependente já desligado não entra no plano", () => {
    const g = graph().map((m) => (m.key === "aa" ? { ...m, configured: false, enabled: false } : m));
    expect(planToggle(g, "cc", false).steps.map((s) => s.key)).toEqual(["bb", "cc"]);
  });

  it("ativar liga antes as dependências inativas, de baixo para cima", () => {
    const off = graph().map((m) => (m.core ? m : { ...m, configured: false, enabled: false }));
    expect(planToggle(off, "aa", true).steps.map((s) => s.key)).toEqual(["cc", "bb", "aa"]);
  });

  it("dependência já ativa não é religada", () => {
    const g = graph().map((m) => (m.key === "aa" ? { ...m, configured: false, enabled: false } : m));
    expect(planToggle(g, "aa", true).steps.map((s) => s.key)).toEqual(["aa"]);
  });

  it("módulo configurado mas bloqueado por dependência: ativar religa só a dependência e ele", () => {
    const g = graph().map((m) =>
      m.key === "bb" ? { ...m, configured: false, enabled: false } : m.key === "aa" ? { ...m, enabled: false, blocked_by: ["bb"] } : m,
    );
    expect(planToggle(g, "aa", true).steps.map((s) => s.key)).toEqual(["bb", "aa"]);
  });

  it("núcleo nunca desativa", () => {
    expect(planToggle(graph(), "iam", false).error).toMatch(/núcleo/);
  });

  it("já no estado pedido: nada a fazer", () => {
    expect(planToggle(graph(), "aa", true).steps).toEqual([]);
  });

  it("moduleNames traduz chaves", () => {
    expect(moduleNames(graph(), ["aa", "zz"])).toBe("AA, zz");
  });
});
