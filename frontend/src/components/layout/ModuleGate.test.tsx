import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ModuleStatus } from "@/lib/nexus/types";

let pathname = "/";
vi.mock("next/navigation", () => ({ usePathname: () => pathname }));
let modules: ModuleStatus[] = [];
let perms: string[] = [];
vi.mock("@/lib/nexus/NexusProvider", () => ({
  useNexus: () => ({ modules, can: (p: string) => perms.includes(p) }),
}));

import { ModuleGate, moduleForPath } from "./ModuleGate";

function mod(key: string, extra: Partial<ModuleStatus> = {}): ModuleStatus {
  return {
    key, name: key.toUpperCase(), description: "", core: false, default_enabled: true, depends_on: [], permissions: [],
    public: false, icon: "box", route: `/${key}`, enabled: true, configured: true, dependents: [], blocked_by: [], ...extra,
  };
}

describe("ModuleGate", () => {
  beforeEach(() => {
    perms = [];
    modules = [mod("iam", { core: true, route: "/configuracao/usuarios" }), mod("wiki"), mod("catalog", { route: "/gestao/servicos" })];
  });

  it("resolve o módulo pela rota mais específica", () => {
    expect(moduleForPath(modules, "/wiki/manual")?.key).toBe("wiki");
    expect(moduleForPath(modules, "/gestao/servicos")?.key).toBe("catalog");
    expect(moduleForPath(modules, "/wikipedia")).toBeUndefined();
    expect(moduleForPath(modules, "/configuracao/usuarios")).toBeUndefined();
  });

  it("módulo ativo renderiza a página", () => {
    pathname = "/wiki/manual";
    render(<ModuleGate>conteúdo</ModuleGate>);
    expect(screen.getByText("conteúdo")).toBeInTheDocument();
  });

  it("módulo desativado mostra o aviso no lugar da página", () => {
    pathname = "/wiki";
    modules = modules.map((m) => (m.key === "wiki" ? { ...m, enabled: false, configured: false } : m));
    render(<ModuleGate>conteúdo</ModuleGate>);
    expect(screen.queryByText("conteúdo")).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "WIKI está desativado" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Gerenciar módulos/ })).not.toBeInTheDocument();
  });

  it("bloqueado por dependência explica qual e oferece o atalho ao administrador", () => {
    pathname = "/tramite/123";
    perms = ["modules:manage"];
    modules = [mod("signum", { enabled: false, configured: false }), mod("tramite", { enabled: false, blocked_by: ["signum"] })];
    render(<ModuleGate>conteúdo</ModuleGate>);
    expect(screen.getByRole("status")).toHaveTextContent("depende de SIGNUM");
    expect(screen.getByRole("link", { name: /Gerenciar módulos/ })).toHaveAttribute("href", "/configuracao/modulos");
  });
});
