"use client";

import { deleteCookie, readCookie, writeCookie } from "@/lib/prefs/cookies";

// "External store" (useSyncExternalStore) do estado "Sidebar recolhida",
// uma preferência puramente deste dispositivo/navegador (DashboardShell.tsx).
//
// Persiste num COOKIE, não em localStorage: o layout do servidor lê o mesmo
// cookie e já renderiza o shell recolhido no 1º paint. O DashboardShell
// passa esse valor como server snapshot do useSyncExternalStore, então o
// SSR e o cliente coincidem — sem a Sidebar "expandir e recolher" a cada
// refresh. Mesmo padrão do tema (ver components/ui/ThemeToggle.tsx).
//
// Não é useState+useEffect porque ler a preferência e então chamar setState
// num efeito é o anti-padrão que react-hooks/set-state-in-effect
// desencoraja; useSyncExternalStore é a primitiva certa para "estado fora
// do React que precisa re-renderizar quando muda".
//
// Migração: versões anteriores gravavam em localStorage
// ("nexus-sidebar-collapsed"). Na 1ª leitura sem cookie, um valor legado no
// localStorage é promovido para cookie e o antigo apagado.

export const SIDEBAR_COLLAPSED_COOKIE = "nexus-sidebar-collapsed";
const LEGACY_STORAGE_KEY = "nexus-sidebar-collapsed";

type Listener = () => void;
const listeners = new Set<Listener>();
let cached: boolean | null = null;

function migrateLegacyStorage(): boolean | null {
  try {
    const raw = window.localStorage.getItem(LEGACY_STORAGE_KEY);
    if (raw === null) return null;
    const value = raw === "true";
    writeCookie(SIDEBAR_COLLAPSED_COOKIE, String(value));
    window.localStorage.removeItem(LEGACY_STORAGE_KEY);
    return value;
  } catch {
    return null;
  }
}

function read(): boolean {
  if (cached !== null) return cached;
  const fromCookie = readCookie(SIDEBAR_COLLAPSED_COOKIE);
  if (fromCookie !== null) {
    cached = fromCookie === "true";
  } else {
    cached = migrateLegacyStorage() ?? false;
  }
  return cached;
}

export function getSidebarCollapsedSnapshot(): boolean {
  return read();
}

// Fallback para contextos sem o cookie (SSR sem preferência, testes). O
// DashboardShell passa o seu próprio server snapshot, derivado do cookie
// lido no layout.
export function getSidebarCollapsedServerSnapshot(): boolean {
  return false;
}

export function setSidebarCollapsed(next: boolean): void {
  cached = next;
  if (next === false) {
    // Estado padrão: não deixa cookie sobrando (o layout já assume
    // "expandida" sem cookie).
    deleteCookie(SIDEBAR_COLLAPSED_COOKIE);
  } else {
    writeCookie(SIDEBAR_COLLAPSED_COOKIE, "true");
  }
  listeners.forEach((listener) => listener());
}

export function subscribeSidebarCollapsed(listener: Listener): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}
