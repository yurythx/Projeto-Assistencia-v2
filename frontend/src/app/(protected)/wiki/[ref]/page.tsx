"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { History, Pencil, RotateCcw, Trash2 } from "lucide-react";
import { useState } from "react";

import { PublicoBadge } from "@/components/iam/PublicoAlvo";
import { ConfirmButton } from "@/components/nexus/ConfirmButton";
import { DataState } from "@/components/nexus/DataState";
import { Markdown } from "@/components/nexus/Markdown";
import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import { WikiEditor } from "@/components/wiki/WikiEditor";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { WikiPage, WikiRevision } from "@/lib/nexus/types";

function Revisions({ page, onRestored }: { page: WikiPage; onRestored: () => void }) {
  const revs = useApiQuery<WikiRevision[]>(`v1/wiki/pages/${page.id}/revisions`);
  const [viewing, setViewing] = useState<number | null>(null);
  const rev = useApiQuery<WikiRevision>(viewing ? `v1/wiki/pages/${page.id}/revisions/${viewing}` : null);
  const { run, pending } = useAction();

  return (
    <div className="grid gap-4 md:grid-cols-[14rem_1fr]">
      <ul className="flex flex-col gap-1 text-sm">
        {(revs.data ?? []).map((r) => (
          <li key={r.id}>
            <button
              type="button"
              onClick={() => setViewing(r.version)}
              aria-pressed={viewing === r.version}
              className={`w-full rounded-md px-2 py-1.5 text-left ${viewing === r.version ? "bg-primary/10" : "hover:bg-surface-hover"}`}
            >
              <span className="font-medium">v{r.version}</span> · {fmtDateTime(r.edited_at)}
              <span className="block truncate text-xs text-muted">
                {r.edited_by_name}
                {r.summary && ` — ${r.summary}`}
              </span>
            </button>
          </li>
        ))}
      </ul>
      <div className="min-w-0">
        {rev.data ? (
          <div className="flex flex-col gap-3">
            {rev.data.version !== page.version && (
              <Button
                size="sm"
                variant="secondary"
                className="self-start"
                loading={pending}
                onClick={() => void run(() => apiClient.post(`v1/wiki/pages/${page.id}/revisions/${rev.data!.version}/restore`), `Versão ${rev.data!.version} restaurada`).then(onRestored)}
              >
                <RotateCcw size={14} aria-hidden="true" className="mr-1" /> Restaurar esta versão
              </Button>
            )}
            <div className="rounded-lg border border-surface-border p-4">
              <h3 className="text-lg font-semibold">{rev.data.title}</h3>
              <Markdown source={rev.data.body ?? ""} />
            </div>
          </div>
        ) : (
          <p className="text-sm text-muted">Selecione uma versão para visualizar.</p>
        )}
      </div>
    </div>
  );
}

export default function WikiPageView() {
  const { ref } = useParams<{ ref: string }>();
  const router = useRouter();
  const { can } = useNexus();
  const page = useApiQuery<WikiPage>(`v1/wiki/pages/${encodeURIComponent(ref)}`);
  const tree = useApiQuery<WikiPage[]>("v1/wiki/tree");
  const { run } = useAction();
  const [mode, setMode] = useState<"edit" | "history" | null>(null);
  const p = page.data;

  return (
    <DataState loading={page.isLoading} error={page.error} empty={!p}>
      {p && (
        <article className="flex flex-col gap-4">
          {(p.breadcrumbs ?? []).length > 0 && (
            <nav aria-label="Trilha" className="text-xs text-muted">
              {(p.breadcrumbs ?? []).map((b) => (
                <span key={b.id}>
                  <Link href={`/wiki/${b.slug}`} className="hover:text-foreground hover:underline">
                    {b.title}
                  </Link>{" "}
                  /{" "}
                </span>
              ))}
            </nav>
          )}
          <header className="flex flex-wrap items-start justify-between gap-3 border-b border-surface-border pb-4">
            <div>
              <h1 className="text-3xl font-bold">{p.title}</h1>
              <PublicoBadge publico={p.publico} />
              <p className="mt-1 text-xs text-muted">
                Versão {p.version} · atualizada por {p.updated_by_name} em {fmtDateTime(p.updated_at)}
              </p>
            </div>
            <div className="flex gap-2">
              <Button variant="secondary" size="sm" onClick={() => setMode("edit")}>
                <Pencil size={14} aria-hidden="true" className="mr-1" /> Editar
              </Button>
              <Button variant="ghost" size="sm" onClick={() => setMode("history")}>
                <History size={14} aria-hidden="true" className="mr-1" /> Histórico
              </Button>
              {can("wiki:manage") && (
                <ConfirmButton
                  aria-label="Excluir página"
                  title={`Excluir "${p.title}"?`}
                  description="Subpáginas precisam ser movidas ou excluídas antes."
                  confirmLabel="Excluir"
                  onConfirm={() =>
                    run(() => apiClient.delete(`v1/wiki/pages/${p.id}`), "Página excluída").then((ok) => {
                      if (ok !== undefined) {
                        void tree.mutate();
                        router.push("/wiki");
                      }
                    })
                  }
                >
                  <Trash2 size={14} aria-hidden="true" />
                </ConfirmButton>
              )}
            </div>
          </header>
          <Markdown source={p.body ?? ""} />

          <Dialog open={mode !== null} onClose={() => setMode(null)} title={mode === "edit" ? `Editar — ${p.title}` : "Histórico de versões"} size="xl">
            {mode === "edit" && (
              <WikiEditor
                page={p}
                pages={tree.data ?? []}
                onSaved={(np) => {
                  setMode(null);
                  void tree.mutate();
                  if (np.slug !== ref) router.replace(`/wiki/${np.slug}`);
                  else void page.mutate();
                }}
              />
            )}
            {mode === "history" && (
              <Revisions
                page={p}
                onRestored={() => {
                  setMode(null);
                  void page.mutate();
                }}
              />
            )}
          </Dialog>
        </article>
      )}
    </DataState>
  );
}
