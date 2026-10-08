"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { Archive, ArrowLeft, Pencil, Send, Trash2, Undo2 } from "lucide-react";
import { useState } from "react";

import { PostEditor } from "@/components/blog/PostEditor";
import { ConfirmButton } from "@/components/nexus/ConfirmButton";
import { DataState } from "@/components/nexus/DataState";
import { Markdown } from "@/components/nexus/Markdown";
import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import { PublicoBadge } from "@/components/iam/PublicoAlvo";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { Post } from "@/lib/nexus/types";

export default function PostPage() {
  const { ref } = useParams<{ ref: string }>();
  const router = useRouter();
  const { can } = useNexus();
  const manage = can("blog:manage");
  const post = useApiQuery<Post>(`v1/blog/posts/${encodeURIComponent(ref)}`);
  const { run, pending } = useAction();
  const [editing, setEditing] = useState(false);
  const p = post.data;

  const transition = (action: "publish" | "unpublish" | "archive", msg: string) =>
    run(() => apiClient.post<Post>(`v1/blog/posts/${p!.id}/${action}`), msg).then(() => post.mutate());

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6">
      <Link href="/blog" className="inline-flex items-center gap-1 text-sm text-muted hover:text-foreground">
        <ArrowLeft size={14} aria-hidden="true" /> Todas as publicações
      </Link>
      <DataState loading={post.isLoading} error={post.error} empty={!p}>
        {p && (
          <article className="flex flex-col gap-4">
            <header className="flex flex-col gap-2">
              <div className="flex flex-wrap items-center gap-2 text-xs text-muted">
                <Badge tone={p.kind === "comunicado" ? "info" : "neutral"}>{p.kind === "comunicado" ? "Comunicado" : "Notícia"}</Badge>
                {p.status !== "published" && <Badge tone="warning">{p.status === "draft" ? "Rascunho" : "Arquivado"}</Badge>}
                <PublicoBadge publico={p.publico} />
                <span>{fmtDateTime(p.published_at ?? p.updated_at)}</span>
                {p.author_name && <span>· {p.author_name}</span>}
              </div>
              <h1 className="text-3xl font-bold">{p.title}</h1>
              {p.summary && <p className="text-lg text-muted">{p.summary}</p>}
            </header>

            {manage && (
              <div className="flex flex-wrap gap-2 rounded-lg border border-surface-border p-3">
                <Button variant="secondary" size="sm" onClick={() => setEditing(true)}>
                  <Pencil size={14} aria-hidden="true" className="mr-1" /> Editar
                </Button>
                {p.status !== "published" && (
                  <Button size="sm" loading={pending} onClick={() => void transition("publish", "Publicado — evento blog.post.published emitido")}>
                    <Send size={14} aria-hidden="true" className="mr-1" /> Publicar
                  </Button>
                )}
                {p.status === "published" && (
                  <Button variant="secondary" size="sm" loading={pending} onClick={() => void transition("unpublish", "Voltou para rascunho")}>
                    <Undo2 size={14} aria-hidden="true" className="mr-1" /> Despublicar
                  </Button>
                )}
                {p.status !== "archived" && (
                  <Button variant="secondary" size="sm" loading={pending} onClick={() => void transition("archive", "Arquivado")}>
                    <Archive size={14} aria-hidden="true" className="mr-1" /> Arquivar
                  </Button>
                )}
                <ConfirmButton
                  variant="ghost"
                  className="ml-auto text-danger"
                  title="Excluir publicação?"
                  description="A exclusão é definitiva e fica registrada na auditoria."
                  confirmLabel="Excluir"
                  onConfirm={() => run(() => apiClient.delete(`v1/blog/posts/${p.id}`), "Publicação excluída").then(() => router.push("/blog"))}
                >
                  <Trash2 size={14} aria-hidden="true" className="mr-1" /> Excluir
                </ConfirmButton>
              </div>
            )}

            {p.cover_url && (
              // eslint-disable-next-line @next/next/no-img-element
              <img src={p.cover_url} alt="" className="max-h-96 w-full rounded-xl object-cover" />
            )}
            <Markdown source={p.body ?? ""} />
          </article>
        )}
      </DataState>

      <Dialog open={editing} onClose={() => setEditing(false)} title="Editar publicação" size="xl">
        {editing && p && (
          <PostEditor
            post={p}
            onSaved={(np) => {
              setEditing(false);
              if (np.slug !== ref) router.replace(`/blog/${np.slug}`);
              else void post.mutate();
            }}
          />
        )}
      </Dialog>
    </div>
  );
}
