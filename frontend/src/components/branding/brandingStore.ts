"use client";

import {
  BRANDING_COOKIE,
  DEFAULT_BRANDING,
  parseBrandingCookie,
  type SystemBrandingConfig,
} from "./brandingConfig";
import { deleteCookie, readCookie, writeCookie } from "@/lib/prefs/cookies";

// Store externo (useSyncExternalStore) do branding no navegador. A
// identidade da organização chega do servidor (primeBrandingStore, com o
// que o layout leu de GET /branding); só as preferências de acessibilidade
// do visitante (Alto Contraste, escala de fonte) são gravadas no cookie —
// o mesmo que o layout do servidor lê, para o 1º paint já sair certo.

type Listener = () => void;
const listeners = new Set<Listener>();
let cached: SystemBrandingConfig | null = null;

function prefsFromCookie(): Pick<SystemBrandingConfig, "highContrast" | "fontSizeScale"> {
  const parsed = parseBrandingCookie(readCookie(BRANDING_COOKIE));
  return { highContrast: parsed.highContrast, fontSizeScale: parsed.fontSizeScale };
}

function read(): SystemBrandingConfig {
  if (!cached) cached = { ...DEFAULT_BRANDING, ...prefsFromCookie() };
  return cached;
}

// Assinatura da última identidade vinda do servidor que já foi aplicada.
let primedSignature: string | null = null;

/** Semeia o store com o branding que o servidor renderizou (identidade da
 * organização + preferências do cookie). Roda a cada render do provider,
 * mas só aplica uma identidade de servidor NOVA: comparar com o store
 * (como antes) desfazia na hora qualquer atualização local — salvar o
 * branding mudava o nome e o render seguinte o trocava de volta pelo que o
 * layout tinha lido antes. */
export function primeBrandingStore(server: SystemBrandingConfig): void {
  // Sem as preferências do visitante (undefined some do JSON).
  const signature = JSON.stringify({ ...server, highContrast: undefined, fontSizeScale: undefined });
  if (signature === primedSignature) return;
  primedSignature = signature;
  cached = { ...server, ...prefsFromCookie() };
}

export function getBrandingSnapshot(): SystemBrandingConfig {
  return read();
}

export function getBrandingServerSnapshot(): SystemBrandingConfig {
  return DEFAULT_BRANDING;
}

export function subscribeBranding(listener: Listener): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function emit() {
  for (const l of listeners) l();
}

function persistPrefs(c: SystemBrandingConfig) {
  writeCookie(BRANDING_COOKIE, JSON.stringify({ highContrast: c.highContrast, fontSizeScale: c.fontSizeScale }));
}

export function updateBrandingStore(partial: Partial<SystemBrandingConfig>): void {
  cached = { ...read(), ...partial };
  persistPrefs(cached);
  emit();
}

export function resetBrandingStore(): void {
  cached = { ...read(), highContrast: false, fontSizeScale: 100 };
  deleteCookie(BRANDING_COOKIE);
  emit();
}
