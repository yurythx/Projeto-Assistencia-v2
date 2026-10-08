import { describe, expect, it } from "vitest";

import { hasPermission } from "@/lib/nexus/permissions";

import { CONFIG_TABS, visibleTabs } from "./sectionTabs";

describe("abas de Configurações", () => {
  it("cada aba só aparece com a permissão correspondente", () => {
    const tabs = visibleTabs(CONFIG_TABS, (p) => hasPermission(["users:read", "iam:*"], p), () => true);
    expect(tabs.map((t) => t.href)).toEqual([
      "/configuracao/organizacao",
      "/configuracao/perfis",
      "/configuracao/mapeamento-ad",
      "/configuracao/usuarios",
    ]);
  });

  it("Egress exige a permissão E o plug-in ativo", () => {
    const can = (p: string) => hasPermission(["*"], p);
    expect(visibleTabs(CONFIG_TABS, can, () => true).some((t) => t.href.endsWith("egress"))).toBe(true);
    expect(visibleTabs(CONFIG_TABS, can, (m) => m !== "egress").some((t) => t.href.endsWith("egress"))).toBe(false);
  });

  it("administração delegada (ADR 013): perfis e mapeamentos só com concessão global", () => {
    const can = (p: string) => hasPermission(["users:read", "iam:*"], p);
    const tabs = visibleTabs(CONFIG_TABS, can, () => true, () => false);
    expect(tabs.map((t) => t.href)).toEqual(["/configuracao/organizacao", "/configuracao/usuarios"]);
  });

  it("sem permissões: nenhuma aba", () => {
    expect(visibleTabs(CONFIG_TABS, () => false, () => true)).toEqual([]);
  });
});
