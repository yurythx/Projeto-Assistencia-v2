import Link from "next/link";

import { Badge } from "@/components/ui/Badge";
import type { Workflow } from "@/lib/nexus/types";

import { NIVEL_ACESSO } from "./labels";
import { SITUACAO } from "./ValidacaoProcedimento";

/** Cartões de procedimento: cada um é um link para a página própria
 * (/atlas/procedimentos/{id}) — URL compartilhável e navegação nativa. */
export function WorkflowList({ items }: { items: Workflow[] }) {
  return (
    <ul className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3" aria-label="Procedimentos">
      {items.map((wf) => {
        const nivel = NIVEL_ACESSO[wf.nivel_acesso];
        return (
          <li key={wf.id}>
            <Link
              href={`/atlas/procedimentos/${wf.id}`}
              className="flex h-full flex-col rounded-lg border border-surface-border bg-surface p-4 transition-colors hover:border-primary/50 hover:bg-surface-hover focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary"
            >
              <span className="flex items-start justify-between gap-2">
                <span className="font-mono text-xs font-bold text-primary">
                  {wf.codigo_processual}
                </span>
                <span className="flex gap-1">
                  {wf.situacao && wf.situacao !== "HOMOLOGADO" ? (
                    <Badge tone={SITUACAO[wf.situacao].tone}>{SITUACAO[wf.situacao].label}</Badge>
                  ) : (
                    !wf.ativo && <Badge tone="danger">Inativo</Badge>
                  )}
                  {wf.classificacao?.revogada_em && <Badge tone="warning">Série revogada</Badge>}
                  <Badge tone={nivel.tone}>{nivel.label}</Badge>
                </span>
              </span>
              <span className="mt-1 block font-semibold text-foreground">{wf.titulo}</span>
              <span className="mt-1 line-clamp-2 block flex-1 text-xs text-muted">
                {wf.objetivo}
              </span>
              <span className="mt-3 flex items-center justify-between border-t border-surface-border pt-2 text-xs text-muted">
                <span className="font-mono">TTDD {wf.codigo_ttdd}</span>
                <span>
                  {wf.total_etapas} {wf.total_etapas === 1 ? "etapa" : "etapas"}
                </span>
              </span>
            </Link>
          </li>
        );
      })}
    </ul>
  );
}
