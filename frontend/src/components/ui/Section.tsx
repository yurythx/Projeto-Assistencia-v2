"use client";

import { useState, type ReactNode } from "react";
import { ChevronDown, ChevronUp } from "lucide-react";

export function Section({
  title,
  description,
  action,
  collapsible = false,
  defaultExpanded = true,
  children,
}: {
  title: string;
  description?: string;
  action?: ReactNode;
  collapsible?: boolean;
  defaultExpanded?: boolean;
  children: ReactNode;
}) {
  const [isExpanded, setIsExpanded] = useState(defaultExpanded);

  return (
    <section className="rounded-xl border border-surface-border bg-surface shadow-sm transition-all duration-200">
      {/* A-06: expandir/recolher fica SÓ no <button> do chevron — o cabeçalho
          não é mais um alvo de clique de mouse inacessível por teclado. */}
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-surface-border px-5 py-4">
        <div className="flex-1">
          <h2 className="text-base font-semibold text-foreground">{title}</h2>
          {description && <p className="mt-0.5 text-sm text-muted">{description}</p>}
        </div>
        <div className="flex items-center gap-2">
          {action}
          {collapsible && (
            <button
              type="button"
              onClick={() => setIsExpanded((v) => !v)}
              aria-expanded={isExpanded}
              className="rounded-md p-1.5 text-muted transition-colors hover:bg-surface-border/50 hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
              aria-label={isExpanded ? `Minimizar seção "${title}"` : `Expandir seção "${title}"`}
            >
              {isExpanded ? <ChevronUp size={20} aria-hidden="true" /> : <ChevronDown size={20} aria-hidden="true" />}
            </button>
          )}
        </div>
      </div>
      {(!collapsible || isExpanded) && <div className="p-5">{children}</div>}
    </section>
  );
}
