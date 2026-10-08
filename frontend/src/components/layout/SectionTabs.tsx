"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

export interface SectionTab {
  href: string;
  label: string;
}

/**
 * Tira de abas horizontal para sub-navegação de uma seção (Integrações,
 * Configurações…). Mesmo visual/lógica de estado ativo da que
 * configuracao/layout.tsx tem embutida — extraído porque outras seções
 * precisam da mesma tira mas não podem usar um layout Next (ex.: uma
 * seção com container de altura fixa próprio).
 *
 * `exactFirst`: quando true, a primeira aba (a "índice" da seção) só fica
 * ativa em match exato — as demais casam por prefixo de segmento.
 */
export function SectionTabs({
  tabs,
  ariaLabel,
  exactFirst = true,
  className = "",
}: {
  tabs: SectionTab[];
  ariaLabel: string;
  exactFirst?: boolean;
  className?: string;
}) {
  const pathname = usePathname();
  const indexHref = tabs[0]?.href;

  return (
    <nav aria-label={ariaLabel} className={`border-b border-surface-border ${className}`}>
      <ul className="-mb-px flex gap-4 overflow-x-auto">
        {tabs.map((tab) => {
          const isIndex = exactFirst && tab.href === indexHref;
          const active = isIndex
            ? pathname === tab.href
            : pathname === tab.href || pathname.startsWith(tab.href + "/");
          return (
            <li key={tab.href}>
              <Link
                href={tab.href}
                aria-current={active ? "page" : undefined}
                className={`inline-block whitespace-nowrap border-b-2 px-1 pb-3 text-sm font-medium transition-colors
                  ${active ? "border-primary text-primary" : "border-transparent text-muted hover:text-foreground"}`}
              >
                {tab.label}
              </Link>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
