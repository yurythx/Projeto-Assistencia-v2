"use client";

import { Menu, Search } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, type FormEvent, type ReactNode } from "react";

import { useBranding } from "@/components/branding/BrandingContext";
import { AccessibilityBar } from "@/components/layout/AccessibilityBar";
import { Logo } from "@/components/ui/Logo";
import { safeResourceUrl } from "@/lib/security/safe-url";

/**
 * Cabeçalho institucional DSGov: barra de acessibilidade e-MAG + faixa
 * principal com a marca (white-label), o campo de busca global
 * (#global-search, alvo do Alt+3) e as ações da área (slot `actions`).
 */
export function GovHeader({
  homeHref,
  searchAction,
  onToggleMenu,
  menuExpanded,
  nav,
  actions,
}: {
  homeHref: string;
  /** Rota de resultados da busca (ex.: "/busca"); omitido = sem campo de busca. */
  searchAction?: string;
  onToggleMenu?: () => void;
  menuExpanded?: boolean;
  nav?: ReactNode;
  actions?: ReactNode;
}) {
  const { branding } = useBranding();
  const router = useRouter();
  const [q, setQ] = useState("");
  const [logoError, setLogoError] = useState(false);
  const logoSrc = logoError ? null : safeResourceUrl(branding.logoUrl);

  function submit(e: FormEvent) {
    e.preventDefault();
    if (searchAction && q.trim().length >= 2) router.push(`${searchAction}?q=${encodeURIComponent(q.trim())}`);
  }

  return (
    <header className="gov-header fixed inset-x-0 top-0 z-50 flex h-[var(--topbar-h)] flex-col border-b border-surface-border bg-surface/95 shadow-xs backdrop-blur-md">
      <AccessibilityBar showSearchShortcut={Boolean(searchAction)} />
      <div className="flex min-h-0 flex-1 items-center gap-3 px-4">
        {onToggleMenu && (
          <button
            type="button"
            onClick={onToggleMenu}
            aria-label="Alternar menu de navegação"
            aria-expanded={menuExpanded}
            aria-controls="main-menu"
            className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-md text-muted hover:bg-surface-hover hover:text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-primary"
          >
            <Menu size={18} aria-hidden="true" />
          </button>
        )}
        <Link href={homeHref} className="flex shrink-0 items-center gap-2.5 rounded-md p-1 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary">
          {logoSrc ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={logoSrc} alt={`Logomarca de ${branding.appName}`} height={32} onError={() => setLogoError(true)} className="h-8 w-auto max-w-[140px] object-contain" />
          ) : (
            <Logo size={30} />
          )}
          <span className="hidden flex-col sm:flex">
            <span className="text-sm font-bold leading-tight text-foreground">{branding.appName}</span>
            <span className="text-[10px] uppercase leading-none tracking-[0.12em] text-muted">{branding.orgName}</span>
          </span>
        </Link>

        {nav}

        <div className="ml-auto flex items-center gap-2">
          {searchAction && (
            <form role="search" onSubmit={submit} className="relative hidden md:block">
              <label htmlFor="global-search" className="sr-only">
                Buscar em todo o sistema
              </label>
              <Search size={15} aria-hidden="true" className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-muted" />
              <input
                id="global-search"
                type="search"
                value={q}
                onChange={(e) => setQ(e.target.value)}
                placeholder="Buscar (Alt+3)"
                autoComplete="off"
                className="h-9 w-64 rounded-md border border-surface-border bg-background pl-8 pr-3 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-primary lg:w-80"
              />
            </form>
          )}
          {actions}
        </div>
      </div>
    </header>
  );
}
