import { beforeEach, describe, expect, it, vi } from "vitest";

// A store é um singleton de módulo (cache em memória). vi.resetModules() +
// import dinâmico dá a cada teste uma instância zerada, como um refresh de
// página faria.
async function freshStore() {
  vi.resetModules();
  return import("./sidebarCollapsedStore");
}

function clearCookies() {
  for (const pair of document.cookie.split(";")) {
    const name = pair.split("=")[0]?.trim();
    if (name) document.cookie = `${name}=; path=/; max-age=0`;
  }
}

describe("sidebarCollapsedStore", () => {
  beforeEach(() => {
    clearCookies();
    window.localStorage.clear();
  });

  it("getServerSnapshot é sempre false (fallback; o server snapshot real vem do layout)", async () => {
    const store = await freshStore();
    expect(store.getSidebarCollapsedServerSnapshot()).toBe(false);
  });

  it("sem cookie nem localStorage, o snapshot é false", async () => {
    const store = await freshStore();
    expect(store.getSidebarCollapsedSnapshot()).toBe(false);
  });

  it("persiste a escolha num cookie e reflete no snapshot", async () => {
    const store = await freshStore();
    store.setSidebarCollapsed(true);
    expect(store.getSidebarCollapsedSnapshot()).toBe(true);
    expect(document.cookie).toContain("nexus-sidebar-collapsed=true");
  });

  it("voltar para expandida (false) apaga o cookie — layout já assume expandida sem cookie", async () => {
    const store = await freshStore();
    store.setSidebarCollapsed(true);
    store.setSidebarCollapsed(false);
    expect(store.getSidebarCollapsedSnapshot()).toBe(false);
    expect(document.cookie).not.toContain("nexus-sidebar-collapsed=true");
  });

  it("lê um cookie pré-existente no primeiro snapshot", async () => {
    document.cookie = "nexus-sidebar-collapsed=true; path=/";
    const store = await freshStore();
    expect(store.getSidebarCollapsedSnapshot()).toBe(true);
  });

  it("migra um valor legado do localStorage para cookie na 1ª leitura", async () => {
    window.localStorage.setItem("nexus-sidebar-collapsed", "true");
    const store = await freshStore();
    expect(store.getSidebarCollapsedSnapshot()).toBe(true);
    expect(document.cookie).toContain("nexus-sidebar-collapsed=true");
    expect(window.localStorage.getItem("nexus-sidebar-collapsed")).toBeNull();
  });

  it("notifica listeners inscritos quando o valor muda e para após unsubscribe", async () => {
    const store = await freshStore();
    const listener = vi.fn();
    const unsubscribe = store.subscribeSidebarCollapsed(listener);

    store.setSidebarCollapsed(true);
    expect(listener).toHaveBeenCalledTimes(1);

    unsubscribe();
    store.setSidebarCollapsed(false);
    expect(listener).toHaveBeenCalledTimes(1);
  });
});
