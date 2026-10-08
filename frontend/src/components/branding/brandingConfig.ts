// Formato + padrões do branding white-label do Projeto Nexus.
//
// Duas origens se combinam:
//  - identidade da organização (nome, órgão, contatos, logo e design
//    tokens de cor) — vem do backend (GET /api/v1/branding), a mesma para
//    todos os visitantes;
//  - preferências de acessibilidade do visitante (Alto Contraste e escala
//    de fonte e-MAG) — persistidas no cookie `nexus-branding` deste
//    navegador.

import type { Branding } from "@/lib/nexus/types";

export interface SystemBrandingConfig {
  appName: string;
  appDescription: string;
  orgName: string;
  logoUrl: string;
  faviconUrl: string;
  supportEmail: string;
  supportPhone: string;
  supportHours: string;
  /** Design tokens de cor (white-label) aplicados como CSS custom properties. */
  tokens: Record<string, string>;
  highContrast: boolean;
  /** Escala tipográfica e-MAG (%): 90, 100, 110, 120, 130. */
  fontSizeScale: number;
}

export const DEFAULT_BRANDING: SystemBrandingConfig = {
  appName: "Projeto Nexus",
  appDescription: "Plataforma corporativa modular",
  orgName: "Organização",
  logoUrl: "",
  faviconUrl: "",
  supportEmail: "",
  supportPhone: "",
  supportHours: "",
  tokens: {},
  highContrast: false,
  fontSizeScale: 100,
};

export const BRANDING_COOKIE = "nexus-branding";

/** Só as preferências do visitante vão para o cookie. */
type Prefs = Pick<SystemBrandingConfig, "highContrast" | "fontSizeScale">;

const FONT_SCALES = [90, 100, 110, 120, 130];

function sanitizePrefs(p: Partial<Prefs> | null | undefined): Partial<Prefs> {
  const out: Partial<Prefs> = {};
  if (!p || typeof p !== "object") return out;
  if (typeof p.highContrast === "boolean") out.highContrast = p.highContrast;
  if (typeof p.fontSizeScale === "number" && FONT_SCALES.includes(p.fontSizeScale)) out.fontSizeScale = p.fontSizeScale;
  return out;
}

export function parseBrandingCookie(raw: string | null | undefined): SystemBrandingConfig {
  if (!raw) return DEFAULT_BRANDING;
  const attempts = [raw];
  if (raw.includes("%")) {
    try {
      attempts.push(decodeURIComponent(raw));
    } catch {
      // valor malformado
    }
  }
  for (const text of attempts) {
    try {
      const parsed = JSON.parse(text) as Partial<SystemBrandingConfig> | null;
      if (parsed && typeof parsed === "object") {
        return { ...DEFAULT_BRANDING, ...parsed, ...sanitizePrefs(parsed) };
      }
    } catch {
      // tenta a próxima forma
    }
  }
  return DEFAULT_BRANDING;
}

export function serializeBrandingCookie(config: SystemBrandingConfig): string {
  return JSON.stringify(config);
}

const HEX = /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/;
const TOKEN_NAME = /^[a-z][a-z-]{1,40}$/;

/** Mescla a identidade vinda da API com as preferências do visitante. */
export function mergeServerBranding(api: Branding | null, prefs: SystemBrandingConfig): SystemBrandingConfig {
  if (!api) return prefs;
  return {
    ...prefs,
    appName: api.app_name || DEFAULT_BRANDING.appName,
    appDescription: api.app_description,
    orgName: api.org_name || DEFAULT_BRANDING.orgName,
    logoUrl: api.logo_url,
    faviconUrl: api.favicon_url,
    supportEmail: api.support_email,
    supportPhone: api.support_phone,
    supportHours: api.support_hours,
    tokens: api.tokens ?? {},
  };
}

/** CSS das variáveis de tema white-label. Só nomes e cores hexadecimais
 * válidos passam (defesa contra injeção de CSS, além da validação do
 * backend). O Alto Contraste e-MAG sempre prevalece. */
export function tokensToCss(tokens: Record<string, string>): string {
  const decls = Object.entries(tokens)
    .filter(([k, v]) => TOKEN_NAME.test(k) && HEX.test(v))
    .map(([k, v]) => `--${k}:${v};`)
    .join("");
  return decls ? `:root:not([data-high-contrast="true"]){${decls}}` : "";
}
