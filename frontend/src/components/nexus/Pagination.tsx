"use client";

import { Button } from "@/components/ui/Button";
import type { PageMeta } from "@/lib/nexus/types";

export function Pagination({ meta, onPage }: { meta?: PageMeta; onPage: (page: number) => void }) {
  if (!meta || meta.total_pages <= 1) return null;
  return (
    <nav aria-label="Paginação" className="mt-4 flex items-center justify-between gap-3 text-sm text-muted">
      <span>
        Página {meta.page} de {meta.total_pages} · {meta.total_items} registros
      </span>
      <div className="flex gap-2">
        <Button variant="secondary" size="sm" disabled={meta.page <= 1} onClick={() => onPage(meta.page - 1)}>
          Anterior
        </Button>
        <Button variant="secondary" size="sm" disabled={meta.page >= meta.total_pages} onClick={() => onPage(meta.page + 1)}>
          Próxima
        </Button>
      </div>
    </nav>
  );
}
