"use client";

import { Accessibility, Contrast } from "lucide-react";
import Link from "next/link";

import { useBranding } from "@/components/branding/BrandingContext";

/** Alvos dos atalhos de teclado e-MAG (skill §3). */
export const SKIP_TARGETS = [
  { key: "1", id: "main-content", label: "Ir para o conteúdo" },
  { key: "2", id: "main-menu", label: "Ir para o menu" },
  { key: "3", id: "global-search", label: "Ir para a busca" },
  { key: "4", id: "gov-footer", label: "Ir para o rodapé" },
] as const;

/**
 * Barra de Acessibilidade Governamental (DSGov / e-MAG 3.1):
 *  - atalhos Alt+1..4 (links visíveis ao receber foco; o listener global
 *    está em AccessibilityShortcuts);
 *  - identidade institucional (órgão · sistema);
 *  - página de acessibilidade, Alto Contraste e escala de fonte A- / A / A+
 *    (persistidos no cookie do visitante).
 */
export function AccessibilityBar({ showSearchShortcut = true }: { showSearchShortcut?: boolean }) {
  const { branding, toggleHighContrast, increaseFontSize, decreaseFontSize, resetFontSize } = useBranding();

  return (
    <div className="gov-accessibility-bar flex h-10 shrink-0 items-center justify-between bg-header-topbar px-3 text-xs font-semibold text-white sm:px-4">
      <nav aria-label="Atalhos de acessibilidade" className="flex min-w-0 items-center">
        <ul className="flex items-center gap-1">
          {SKIP_TARGETS.filter((t) => showSearchShortcut || t.id !== "global-search").map((t) => (
            <li key={t.id}>
              <a
                href={`#${t.id}`}
                accessKey={t.key}
                className="sr-only rounded bg-white px-2 py-1 font-bold text-slate-900 focus:not-sr-only focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-300"
              >
                {t.label} <kbd className="font-mono">[Alt+{t.key}]</kbd>
              </a>
            </li>
          ))}
        </ul>
        <span className="truncate text-[11px] uppercase tracking-wider text-white/90 sm:text-xs">{branding.orgName}</span>
        {branding.appDescription && (
          <span className="ml-2 hidden truncate font-normal text-white/75 md:inline">· {branding.appDescription}</span>
        )}
      </nav>

      <div className="flex shrink-0 items-center gap-1 sm:gap-2">
        <Link
          href="/acessibilidade"
          className="flex items-center gap-1.5 rounded-md px-2 py-1 hover:bg-white/15 focus:outline-none focus-visible:ring-2 focus-visible:ring-white"
        >
          <Accessibility size={15} aria-hidden="true" />
          <span className="hidden sm:inline">Acessibilidade</span>
        </Link>
        <button
          type="button"
          onClick={toggleHighContrast}
          aria-pressed={branding.highContrast}
          className="flex items-center gap-1.5 rounded-md px-2 py-1 hover:bg-white/15 focus:outline-none focus-visible:ring-2 focus-visible:ring-white"
        >
          <Contrast size={14} aria-hidden="true" />
          <span className="hidden sm:inline">Alto contraste</span>
        </button>
        <div role="group" aria-label="Tamanho da fonte" className="flex items-center gap-0.5 rounded-md border border-white/20 bg-white/10 p-0.5">
          <button type="button" onClick={decreaseFontSize} aria-label="Diminuir fonte" className="rounded px-2 py-0.5 hover:bg-white/25 focus:outline-none focus-visible:ring-2 focus-visible:ring-white">
            A-
          </button>
          <button type="button" onClick={resetFontSize} aria-label="Fonte padrão" className="rounded px-2 py-0.5 hover:bg-white/25 focus:outline-none focus-visible:ring-2 focus-visible:ring-white">
            A
          </button>
          <button type="button" onClick={increaseFontSize} aria-label="Aumentar fonte" className="rounded px-2 py-0.5 hover:bg-white/25 focus:outline-none focus-visible:ring-2 focus-visible:ring-white">
            A+
          </button>
        </div>
      </div>
    </div>
  );
}
