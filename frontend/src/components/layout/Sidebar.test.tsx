import { describe, expect, it } from "vitest";

import { buildNav } from "./Sidebar";
import type { ModuleStatus } from "@/lib/nexus/types";
import { hasPermission } from "@/lib/nexus/permissions";

function mod(key: string, extra: Partial<ModuleStatus> = {}): ModuleStatus {
  return {
    key, name: key, description: "", core: false, default_enabled: true, depends_on: [], permissions: [],
    public: false, icon: "box", route: `/${key}`, enabled: true, configured: true, dependents: [], blocked_by: [], ...extra,
  };
}

describe("buildNav", () => {
  const modules = [
    mod("iam", { core: true }), mod("audit", { core: true }), mod("blog"), mod("wiki", { enabled: false }),
    mod("egress"), mod("search"), mod("tramite"),
  ];

  it("lista só plugins ativos, fora do núcleo/ocultos", () => {
    const nav = buildNav(modules, () => false);
    expect(nav.modules.map((m) => m.href)).toEqual(["/blog", "/tramite"]);
    expect(nav.admin).toEqual([]);
  });

  it("administração depende das permissões efetivas (com curinga)", () => {
    const nav = buildNav(modules, (p) => hasPermission(["audit:*"], p));
    expect(nav.admin.map((a) => a.href)).toEqual(["/auditoria"]);
    const all = buildNav(modules, (p) => hasPermission(["*"], p));
    expect(all.admin.map((a) => a.href)).toEqual(["/auditoria", "/monitoramento", "/configuracao"]);
  });

  it("módulos de gestão (catálogo, contato) exigem a permissão para entrar no menu", () => {
    const mods = [mod("catalog", { route: "/gestao/servicos" }), mod("contact", { route: "/gestao/contato" })];
    expect(buildNav(mods, () => false).modules).toEqual([]);
    const nav = buildNav(mods, (p) => hasPermission(["contact:read"], p));
    expect(nav.modules.map((m) => m.href)).toEqual(["/gestao/contato"]);
  });

  it("SEMPRAS vira a própria seção (Atendimento + Painel + Pedidos) só pra quem tem sempras:gestao", () => {
    const mods = [mod("sempras", { route: "/sempras", name: "SEMPRAS" })];
    const semGestao = buildNav(mods, () => false);
    expect(semGestao.sections).toEqual([]);
    expect(semGestao.modules.map((m) => m.href)).toEqual(["/sempras"]);

    const comGestao = buildNav(mods, (p) => hasPermission(["sempras:gestao"], p));
    expect(comGestao.modules).toEqual([]);
    expect(comGestao.sections).toHaveLength(1);
    expect(comGestao.sections[0]!.key).toBe("sempras");
    expect(comGestao.sections[0]!.title).toBe("SEMPRAS");
    expect(comGestao.sections[0]!.items.map((i) => ({ href: i.href, label: i.label }))).toEqual([
      { href: "/sempras", label: "Atendimento" },
      { href: "/sempras/painel", label: "Painel de gestão" },
      { href: "/sempras/unidades", label: "Unidades" },
      { href: "/sempras/rma", label: "RMA/MDS" },
    ]);
  });

  it("SEMPRAS: quem só tem sempras:atendimento fica com o link direto (painel de chamada e coletivos são atalhos da unidade)", () => {
    const mods = [mod("sempras", { route: "/sempras", name: "SEMPRAS" })];
    const comAtendimento = buildNav(mods, (p) => hasPermission(["sempras:atendimento"], p));
    expect(comAtendimento.sections).toEqual([]);
    expect(comAtendimento.modules.map((m) => m.href)).toEqual(["/sempras"]);
  });

  it("módulo desligado some da sua seção sem recarregar (sempras:gestao, mas módulo off)", () => {
    const mods = [mod("sempras", { route: "/sempras", name: "SEMPRAS", enabled: false })];
    const nav = buildNav(mods, (p) => hasPermission(["sempras:gestao"], p));
    expect(nav.modules).toEqual([]);
    expect(nav.sections).toEqual([]);
  });
});
