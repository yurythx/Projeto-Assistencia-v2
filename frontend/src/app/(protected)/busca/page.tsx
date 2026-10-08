"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Search } from "lucide-react";
import { Suspense, useState } from "react";

import { ModuleIcon } from "@/components/layout/ModuleIcon";
import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { fmtDate } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { useApiQuery, withQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { SearchResponse } from "@/lib/nexus/types";

function Busca() {
  const params = useSearchParams();
  const router = useRouter();
  const { modules } = useNexus();
  const q = params.get("q") ?? "";
  const mod = params.get("modulo") ?? "";
  const [draft, setDraft] = useState(q);
  const res = useApiQuery<SearchResponse>(q.length >= 2 ? withQuery("v1/search", { q, module: mod, limit: 50 }) : null);
  const nameOf = (key: string) => modules.find((m) => m.key === key)?.name ?? key;
  const iconOf = (key: string) => modules.find((m) => m.key === key)?.icon ?? "box";

  const go = (nq: string, nm = mod) => router.replace(`/busca?${new URLSearchParams({ q: nq, ...(nm ? { modulo: nm } : {}) })}`);

  return (
    <div className="flex flex-col gap-6">
      <PageHeader eyebrow="Busca global" title={q ? `Resultados para “${q}”` : "Buscar"} description="Pesquisa unificada sobre os módulos ativos, respeitando suas permissões." />
      <form
        role="search"
        className="flex items-end gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          if (draft.trim().length >= 2) go(draft.trim());
        }}
      >
        <div className="flex-1">
          <Input id="busca-q" type="search" label="Termo" value={draft} onChange={(e) => setDraft(e.target.value)} minLength={2} />
        </div>
        <Button type="submit">
          <Search size={16} aria-hidden="true" className="mr-1" /> Buscar
        </Button>
      </form>

      {res.data && (
        <div className="flex flex-wrap items-center gap-2 text-xs text-muted">
          <span>
            {res.data.results.length} resultado(s) em {res.data.took_ms} ms ·
          </span>
          <button type="button" onClick={() => go(q, "")} aria-pressed={!mod} className={`rounded-full px-2 py-0.5 ${!mod ? "bg-primary text-primary-foreground" : "bg-surface-hover"}`}>
            Todos
          </button>
          {res.data.modules.map((m) => (
            <button key={m} type="button" onClick={() => go(q, m)} aria-pressed={mod === m} className={`rounded-full px-2 py-0.5 ${mod === m ? "bg-primary text-primary-foreground" : "bg-surface-hover"}`}>
              {nameOf(m)}
            </button>
          ))}
        </div>
      )}
      {res.data && res.data.degraded.length > 0 && (
        <p role="status" className="rounded-md bg-warning/10 p-3 text-xs text-warning">
          Resultados parciais: {res.data.degraded.map(nameOf).join(", ")} não respondeu a tempo.
        </p>
      )}

      {q.length >= 2 && (
        <DataState loading={res.isLoading} error={res.error} empty={(res.data?.results ?? []).length === 0} emptyTitle="Nada encontrado" emptyDescription="Tente outro termo ou remova o filtro de módulo.">
          <ol className="flex flex-col gap-3">
            {(res.data?.results ?? []).map((r) => (
              <li key={`${r.module}-${r.id}`} className="rounded-lg border border-surface-border bg-surface p-4">
                <div className="flex items-center gap-2 text-xs text-muted">
                  <ModuleIcon name={iconOf(r.module)} size={14} aria-hidden="true" />
                  <Badge>{nameOf(r.module)}</Badge>
                  {r.updated_at && <span>{fmtDate(r.updated_at)}</span>}
                </div>
                <Link href={r.url} className="mt-1 block font-semibold text-primary hover:underline">
                  {r.title}
                </Link>
                {r.snippet && <p className="mt-1 text-sm text-muted">{r.snippet}</p>}
              </li>
            ))}
          </ol>
        </DataState>
      )}
    </div>
  );
}

export default function BuscaPage() {
  return (
    <Suspense fallback={null}>
      <Busca />
    </Suspense>
  );
}
