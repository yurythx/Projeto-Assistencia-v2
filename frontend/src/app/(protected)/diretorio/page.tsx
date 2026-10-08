"use client";

import { useRouter } from "next/navigation";
import { Building, Mail, MessagesSquare, Phone, Search } from "lucide-react";
import { useState } from "react";

import { ScopePicker } from "@/components/iam/ScopePicker";
import { SectionTabsInline } from "@/components/nexus/SectionTabsInline";
import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { Pagination } from "@/components/nexus/Pagination";
import { useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { apiClient } from "@/lib/api/client";
import { useApiPage, useApiQuery, withQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { ChatRoom, Person, ScopeRef, Sector } from "@/lib/nexus/types";

function initials(name: string) {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p[0]?.toUpperCase())
    .join("");
}

function People() {
  const { enabled, me } = useNexus();
  const router = useRouter();
  const { run } = useAction();
  const [q, setQ] = useState("");
  const [query, setQuery] = useState("");
  const [scope, setScope] = useState<ScopeRef>({});
  const [page, setPage] = useState(1);
  const list = useApiPage<Person>(withQuery("v1/directory/people", { q: query, unidade_id: scope.unidade_id, departamento_id: scope.departamento_id, page, page_size: 24 }));

  async function chat(p: Person) {
    const res = await run(() => apiClient.post<ChatRoom>("v1/mercurio/direct", { user_id: p.user_id }));
    if (res) router.push(`/mercurio?sala=${res.data.id}`);
  }

  return (
    <div className="flex flex-col gap-4">
      <form
        role="search"
        className="flex flex-col gap-3"
        onSubmit={(e) => {
          e.preventDefault();
          setPage(1);
          setQuery(q.trim());
        }}
      >
        <div className="flex items-end gap-2">
          <div className="flex-1">
            <Input id="dir-q" label="Nome, cargo ou e-mail" value={q} onChange={(e) => setQ(e.target.value)} />
          </div>
          <Button type="submit" variant="secondary">
            <Search size={14} aria-hidden="true" className="mr-1" /> Buscar
          </Button>
        </div>
        <ScopePicker
          idPrefix="dir-filter"
          value={scope}
          onChange={(v) => {
            setPage(1);
            setScope(v);
          }}
        />
      </form>
      <DataState loading={list.isLoading} error={list.error} onRetry={() => void list.mutate()} empty={(list.data?.items ?? []).length === 0} emptyTitle="Ninguém encontrado">
        <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {(list.data?.items ?? []).map((p) => (
            <li key={p.user_id}>
              <Card className="flex h-full gap-3 p-4">
                <span aria-hidden="true" className="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-primary/10 text-sm font-semibold text-primary">
                  {initials(p.name || p.username)}
                </span>
                <div className="flex min-w-0 flex-1 flex-col gap-1 text-sm">
                  <p className="font-semibold">{p.name || p.username}</p>
                  {p.job_title && <p className="text-xs text-muted">{p.job_title}</p>}
                  {(p.departamento || p.unidade) && (
                    <p className="flex items-center gap-1 text-xs text-muted">
                      <Building size={12} aria-hidden="true" /> {[p.departamento, p.unidade].filter(Boolean).join(" · ")}
                    </p>
                  )}
                  {p.email && (
                    <a href={`mailto:${p.email}`} className="flex items-center gap-1 truncate text-xs text-primary hover:underline">
                      <Mail size={12} aria-hidden="true" /> {p.email}
                    </a>
                  )}
                  {(p.phone || p.extension) && (
                    <p className="flex items-center gap-1 text-xs">
                      <Phone size={12} aria-hidden="true" /> {p.phone} {p.extension && `ramal ${p.extension}`}
                    </p>
                  )}
                  <div className="mt-auto flex items-center justify-between pt-1">
                    {p.from_ad ? <Badge tone="info">AD</Badge> : <span />}
                    {enabled("mercurio") && p.user_id !== me?.id && (
                      <Button variant="ghost" size="sm" onClick={() => void chat(p)} aria-label={`Conversar com ${p.name || p.username}`}>
                        <MessagesSquare size={14} aria-hidden="true" />
                      </Button>
                    )}
                  </div>
                </div>
              </Card>
            </li>
          ))}
        </ul>
        <Pagination meta={list.data?.meta} onPage={setPage} />
      </DataState>
    </div>
  );
}

function Sectors() {
  const sectors = useApiQuery<Sector[]>("v1/directory/sectors");
  return (
    <DataState loading={sectors.isLoading} error={sectors.error} empty={(sectors.data ?? []).length === 0} emptyTitle="Nenhum setor cadastrado">
      <ul className="grid gap-3 md:grid-cols-2">
        {(sectors.data ?? []).map((s) => (
          <li key={s.id} className={`rounded-lg border border-surface-border p-4 text-sm ${s.kind === "departamento" ? "ml-4" : ""}`}>
            <p className="font-semibold">
              {s.nome} {s.sigla && <span className="text-muted">({s.sigla})</span>}
            </p>
            <p className="text-xs text-muted">{s.kind === "departamento" ? `Departamento · ${s.unidade}` : `Unidade · ${s.entidade}`}</p>
            {s.email && <a href={`mailto:${s.email}`} className="block text-xs text-primary hover:underline">{s.email}</a>}
            {s.telefone && <p className="text-xs">{s.telefone}</p>}
            {s.endereco && <p className="text-xs text-muted">{s.endereco}</p>}
          </li>
        ))}
      </ul>
    </DataState>
  );
}

export default function DiretorioPage() {
  const [tab, setTab] = useState<"pessoas" | "setores">("pessoas");
  return (
    <div className="flex flex-col gap-6">
      <PageHeader eyebrow="Diretório" title="Pessoas e setores" description="Consulta interna com base nos dados sincronizados do Active Directory." />
      <SectionTabsInline
        label="Visões do diretório"
        value={tab}
        onChange={setTab}
        tabs={[
          { value: "pessoas", label: "Pessoas" },
          { value: "setores", label: "Setores" },
        ]}
      />
      {tab === "pessoas" ? <People /> : <Sectors />}
    </div>
  );
}
