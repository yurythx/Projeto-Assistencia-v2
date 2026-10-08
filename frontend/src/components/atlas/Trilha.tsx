import Link from "next/link";
import { ChevronRight } from "lucide-react";

/** Trilha de navegação (breadcrumb) — o último item é a página atual. */
export function Trilha({ itens }: { itens: { label: string; href?: string }[] }) {
  return (
    <nav aria-label="Trilha de navegação" className="text-xs text-muted print:hidden">
      <ol className="flex flex-wrap items-center gap-1">
        {itens.map((it, i) => (
          <li key={`${i}-${it.label}`} className="flex items-center gap-1">
            {i > 0 && <ChevronRight size={12} aria-hidden="true" />}
            {it.href && i < itens.length - 1 ? (
              <Link href={it.href} className="hover:text-primary hover:underline">
                {it.label}
              </Link>
            ) : (
              <span
                aria-current={i === itens.length - 1 ? "page" : undefined}
                className="text-foreground"
              >
                {it.label}
              </span>
            )}
          </li>
        ))}
      </ol>
    </nav>
  );
}
