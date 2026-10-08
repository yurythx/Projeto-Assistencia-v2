"use client";

import { useState, type FormEvent } from "react";

import { PublicoAlvoPicker, TODOS } from "@/components/iam/PublicoAlvo";
import { UnidadeDonaSelect } from "@/components/iam/UnidadeDonaSelect";
import { Markdown } from "@/components/nexus/Markdown";
import { useAction } from "@/components/nexus/useAction";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import type { WikiPage } from "@/lib/nexus/types";

/** Criação/edição de página. Na edição envia a versão aberta: se outra
 * pessoa salvou antes, o backend responde 409 WIKI_STALE_VERSION. */
export function WikiEditor({
  page,
  pages,
  defaultParent,
  onSaved,
}: {
  page?: WikiPage;
  pages: WikiPage[];
  defaultParent?: string;
  onSaved: (p: WikiPage) => void;
}) {
  const { run, pending } = useAction();
  const [body, setBody] = useState(page?.body ?? "");
  const [preview, setPreview] = useState(false);
  const [publico, setPublico] = useState(page?.publico ?? TODOS);

  // Uma página não pode ser filha de si mesma nem de descendentes.
  const descendants = new Set<string>();
  if (page) {
    const walk = (id: string) => pages.filter((p) => p.parent_id === id).forEach((c) => (descendants.add(c.id), walk(c.id)));
    descendants.add(page.id);
    walk(page.id);
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    const payload = {
      title: String(fd.get("title") ?? "").trim(),
      slug: String(fd.get("slug") ?? "").trim(),
      parent_id: String(fd.get("parent_id") ?? "") || null,
      unidade_id: String(fd.get("unidade_id") ?? "") || null,
      publico,
      position: Number(fd.get("position") ?? 0) || 0,
      summary: String(fd.get("summary") ?? "").trim(),
      body,
      version: page?.version,
    };
    const res = await run(
      () => (page ? apiClient.put<WikiPage>(`v1/wiki/pages/${page.id}`, payload) : apiClient.post<WikiPage>("v1/wiki/pages", payload)),
      "Página salva",
    );
    if (res) onSaved(res.data);
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-3">
      <div className="grid gap-3 sm:grid-cols-2">
        <Input id="wiki-title" name="title" label="Título *" required maxLength={200} defaultValue={page?.title} />
        <Input id="wiki-slug" name="slug" label="Slug" maxLength={100} defaultValue={page?.slug} placeholder="gerado do título" />
      </div>
      <div className="grid gap-3 sm:grid-cols-[1fr_8rem]">
        {/* key: select não controlado — se a árvore chega depois de abrir o
            editor, remonta com a mãe marcada (antes, salvar movia a página
            para a raiz). */}
        <Select
          key={`parent-${pages.length}`}
          id="wiki-parent"
          name="parent_id"
          label="Página-mãe"
          placeholder="(raiz)"
          defaultValue={page?.parent_id ?? defaultParent ?? ""}
          options={pages.filter((p) => !descendants.has(p.id)).map((p) => ({ value: p.id, label: p.title }))}
        />
        <Input id="wiki-position" name="position" type="number" min={0} label="Ordem" defaultValue={page?.position ?? 0} />
      </div>
      <UnidadeDonaSelect
        id="wiki-unidade"
        permission="wiki:manage"
        lotacao
        defaultValue={page?.unidade_id}
        placeholder={page ? "Institucional (sem unidade)" : "Herdar da página-mãe (na raiz: institucional)"}
      />
      {/* Vale com o das páginas acima: a subpágina restringe mais, não abre. */}
      <PublicoAlvoPicker idPrefix="wiki-publico" value={publico} onChange={setPublico} />
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium">Conteúdo (Markdown)</span>
        <Button type="button" variant="ghost" size="sm" onClick={() => setPreview((v) => !v)} aria-pressed={preview}>
          {preview ? "Editar" : "Pré-visualizar"}
        </Button>
      </div>
      {preview ? (
        <div className="min-h-48 rounded-lg border border-surface-border p-4">
          <Markdown source={body || "_(vazio)_"} />
        </div>
      ) : (
        <Textarea id="wiki-body" aria-label="Conteúdo em Markdown" rows={16} value={body} onChange={(e) => setBody(e.target.value)} className="font-mono" />
      )}
      <Input id="wiki-summary" name="summary" label="Resumo da alteração" maxLength={300} placeholder="o que mudou nesta versão" />
      <div className="flex justify-end">
        <Button type="submit" loading={pending}>
          Salvar
        </Button>
      </div>
    </form>
  );
}
