import { beforeEach, describe, expect, it, vi } from "vitest";

import { BRANDING_COOKIE, DEFAULT_BRANDING } from "./brandingConfig";

async function freshStore() {
  vi.resetModules();
  return import("./brandingStore");
}

function clearCookies() {
  for (const pair of document.cookie.split(";")) {
    const name = pair.split("=")[0]?.trim();
    if (name) document.cookie = `${name}=; path=/; max-age=0`;
  }
}

function cookieValue(): Record<string, unknown> {
  const raw = decodeURIComponent(document.cookie.split(`${BRANDING_COOKIE}=`)[1]?.split(";")[0] ?? "{}");
  return JSON.parse(raw);
}

describe("brandingStore", () => {
  beforeEach(() => clearCookies());

  it("sem cookie, o snapshot é o DEFAULT (Projeto Nexus)", async () => {
    const store = await freshStore();
    expect(store.getBrandingSnapshot()).toEqual(DEFAULT_BRANDING);
    expect(store.getBrandingSnapshot().appName).toBe("Projeto Nexus");
  });

  it("prime aplica a identidade do servidor mantendo as preferências do cookie", async () => {
    document.cookie = `${BRANDING_COOKIE}=${encodeURIComponent(JSON.stringify({ highContrast: true }))}; path=/`;
    const store = await freshStore();
    store.primeBrandingStore({ ...DEFAULT_BRANDING, appName: "Órgão X", tokens: { primary: "#003366" } });
    const snap = store.getBrandingSnapshot();
    expect(snap.appName).toBe("Órgão X");
    expect(snap.tokens.primary).toBe("#003366");
    expect(snap.highContrast).toBe(true);
  });

  it("prime não desfaz uma atualização local; identidade nova do servidor prevalece", async () => {
    const store = await freshStore();
    const server = { ...DEFAULT_BRANDING, appName: "Órgão X" };
    store.primeBrandingStore(server);
    store.updateBrandingStore({ appName: "Órgão Y" });
    store.primeBrandingStore(server); // novo render com o mesmo snapshot do layout
    expect(store.getBrandingSnapshot().appName).toBe("Órgão Y");
    store.primeBrandingStore({ ...server, appName: "Órgão Z" }); // layout releu a API
    expect(store.getBrandingSnapshot().appName).toBe("Órgão Z");
  });

  it("update grava SÓ as preferências de acessibilidade no cookie", async () => {
    const store = await freshStore();
    store.updateBrandingStore({ highContrast: true, fontSizeScale: 120 });
    expect(cookieValue()).toEqual({ highContrast: true, fontSizeScale: 120 });
    expect(store.getBrandingSnapshot().fontSizeScale).toBe(120);
  });

  it("reset volta às preferências padrão e apaga o cookie", async () => {
    const store = await freshStore();
    store.updateBrandingStore({ highContrast: true });
    store.resetBrandingStore();
    expect(store.getBrandingSnapshot().highContrast).toBe(false);
    expect(document.cookie).not.toContain(`${BRANDING_COOKIE}=`);
  });

  it("notifica os listeners e para depois do unsubscribe", async () => {
    const store = await freshStore();
    const listener = vi.fn();
    const unsub = store.subscribeBranding(listener);
    store.updateBrandingStore({ fontSizeScale: 110 });
    expect(listener).toHaveBeenCalledTimes(1);
    unsub();
    store.updateBrandingStore({ fontSizeScale: 120 });
    expect(listener).toHaveBeenCalledTimes(1);
  });
});
