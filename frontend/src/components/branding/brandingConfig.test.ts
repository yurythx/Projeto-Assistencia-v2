import { describe, expect, it } from "vitest";

import {
  DEFAULT_BRANDING,
  parseBrandingCookie,
  serializeBrandingCookie,
} from "./brandingConfig";

describe("parseBrandingCookie", () => {
  it("sem valor → DEFAULT_BRANDING", () => {
    expect(parseBrandingCookie(undefined)).toBe(DEFAULT_BRANDING);
    expect(parseBrandingCookie(null)).toBe(DEFAULT_BRANDING);
    expect(parseBrandingCookie("")).toBe(DEFAULT_BRANDING);
  });

  it("JSON parcial mescla sobre o DEFAULT", () => {
    const out = parseBrandingCookie(JSON.stringify({ appName: "Município Y" }));
    expect(out.appName).toBe("Município Y");
    expect(out.orgName).toBe(DEFAULT_BRANDING.orgName);
    expect(out.highContrast).toBe(false);
  });

  it("aceita o valor ainda percent-encoded (runtime que não decodifica o cookie)", () => {
    const encoded = encodeURIComponent(JSON.stringify({ appName: "Rondonópolis" }));
    expect(parseBrandingCookie(encoded).appName).toBe("Rondonópolis");
  });

  it("JSON inválido → DEFAULT_BRANDING (nunca lança)", () => {
    expect(parseBrandingCookie("{nao-e-json")).toBe(DEFAULT_BRANDING);
    expect(parseBrandingCookie("null")).toBe(DEFAULT_BRANDING);
    expect(parseBrandingCookie("42")).toBe(DEFAULT_BRANDING);
  });

  it("round-trip serialize → parse preserva a config", () => {
    const config = { ...DEFAULT_BRANDING, appName: "X", highContrast: true };
    expect(parseBrandingCookie(serializeBrandingCookie(config))).toEqual(config);
  });
});
