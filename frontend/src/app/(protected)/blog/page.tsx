"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { Pin, Plus } from "lucide-react";
import { useState } from "react";

import { PostEditor } from "@/components/blog/PostEditor";
import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { Pagination } from "@/components/nexus/Pagination";
import { fmtDate } from "@/components/nexus/useAction";
import { PublicoBadge } from "@/components/iam/PublicoAlvo";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { useApiPage, withQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { Post } from "@/lib/nexus/types";

const STATUS_LABEL: Record<Post["status"], string> = { draft: "Rascunho", published: "Publicado", archived: "Arquivado" };

export default function BlogPage() {
  const { can } = useNexus();
  const router = useRouter();
  const manage = can("blog:manage");
  const [q, setQ] = useState("");
  const [query, setQuery] = useState("");
  const [kind, setKind] = useState("");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);
  const [creating, setCreating] = useState(false);
  const list = useApiPage<Post>(withQuery("v1/blog/posts", { q: query, kind, status: manage ? status : "", page, page_size: 12 }));

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Comunicação"
        title="Blog & comunicados"
        description="Notícias e comunicados internos da instituição."
        actions={
          manage && (
            <Button onClick={() => setCreating(true)}>
              <Plus size={16} aria-hidden="true" className="mr-1" /> Nova publicação
            </Button>
          )
        }
      />

      <form
        role="search"
        className="flex flex-wrap items-end gap-3"
        onSubmit={(e) => {
          e.preventDefault();
          setPage(1);
          setQuery(q.trim());
        }}
      >
        <Input id="blog-q" label="Buscar" value={q} onChange={(e) => setQ(e.target.value)} />
        <Select
          id="blog-kind"
          label="Tipo"
          value={kind}
          onChange={(e) => {
            setPage(1);
            setKind(e.target.value);
          }}
          options={[
            { value: "", label: "Todos" },
            { value: "noticia", label: "Notícias" },
            { value: "comunicado", label: "Comunicados" },
          ]}
        />
        {manage && (
          <Select
            id="blog-status"
            label="Situação"
            value={status}
            onChange={(e) => {
              setPage(1);
              setStatus(e.target.value);
            }}
            options={[
              { value: "", label: "Todas" },
              { value: "draft", label: "Rascunhos" },
              { value: "published", label: "Publicadas" },
              { value: "archived", label: "Arquivadas" },
            ]}
          />
        )}
        <Button type="submit" variant="secondary">
          Filtrar
        </Button>
      </form>

      <DataState loading={list.isLoading} error={list.error} onRetry={() => void list.mutate()} empty={(list.data?.items ?? []).length === 0} emptyTitle="Nenhuma publicação">
        <ul className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {(list.data?.items ?? []).map((p) => (
            <li key={p.id}>
              <Link href={`/blog/${p.slug}`} className="block h-full rounded-xl focus:outline-none focus-visible:ring-2 focus-visible:ring-primary">
                <Card className="flex h-full flex-col overflow-hidden transition-shadow hover:shadow-md">
                  {p.cover_url && (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img src={p.cover_url} alt="" className="h-40 w-full object-cover" />
                  )}
                  <div className="flex flex-1 flex-col gap-2 p-4">
                    <div className="flex flex-wrap items-center gap-1 text-xs text-muted">
                      {p.pinned && <Pin size={12} aria-label="Fixado" />}
                      <Badge tone={p.kind === "comunicado" ? "info" : "neutral"}>{p.kind === "comunicado" ? "Comunicado" : "Notícia"}</Badge>
                      {p.status !== "published" && <Badge tone="warning">{STATUS_LABEL[p.status]}</Badge>}
                      <PublicoBadge publico={p.publico} />
                      <span>{fmtDate(p.published_at ?? p.updated_at)}</span>
                    </div>
                    <h2 className="font-semibold leading-snug">{p.title}</h2>
                    {p.summary && <p className="line-clamp-3 text-sm text-muted">{p.summary}</p>}
                    {p.author_name && <p className="mt-auto text-xs text-muted">por {p.author_name}</p>}
                  </div>
                </Card>
              </Link>
            </li>
          ))}
        </ul>
        <Pagination meta={list.data?.meta} onPage={setPage} />
      </DataState>

      <Dialog open={creating} onClose={() => setCreating(false)} title="Nova publicação" size="xl">
        {creating && <PostEditor onSaved={(p) => router.push(`/blog/${p.slug}`)} />}
      </Dialog>
    </div>
  );
}
