"use client";

import Link from "next/link";
import { Archive, ExternalLink, Pencil, Plus, Send, Trash2, Undo2 } from "lucide-react";
import { useState, type FormEvent } from "react";

import { ConfirmButton } from "@/components/nexus/ConfirmButton";
import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { Pagination } from "@/components/nexus/Pagination";
import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Table, TableBody, TableCell, TableHead, TableHeaderCell, TableRow } from "@/components/ui/Table";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiPage, useApiQuery, withQuery } from "@/lib/api/swr";
import { SERVICE_ICON_NAMES } from "@/lib/catalog/icons";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { unidadesGeridas } from "@/lib/nexus/permissions";
import type { CatalogService, OrgTree, ServiceChannel } from "@/lib/nexus/types";

const STATUS: Record<CatalogService["status"], { label: string; tone: "neutral" | "success" | "warning" }> = {
  draft: { label: "Rascunho", tone: "neutral" },
  published: { label: "Publicado", tone: "success" },
  archived: { label: "Arquivado", tone: "warning" },
};

const lines = (s: string) => s.split("\n").map((l) => l.trim()).filter(Boolean);

function ServiceForm({ service, onDone }: { service?: CatalogService; onDone: () => void }) {
  const { run, pending } = useAction();
  const tree = useApiQuery<OrgTree[]>("v1/iam/org-tree");
  const [channels, setChannels] = useState<ServiceChannel[]>(service?.channels ?? []);
  const { me } = useNexus();
  // Só as unidades que a pessoa gerencia; sem unidade (institucional) só
  // para a gestão global (ADR 013 — a API confere de novo).
  const geridas = unidadesGeridas(me, tree.data, "catalog:manage");
  const unidades = (tree.data ?? []).flatMap((e) => e.unidades).filter((u) => geridas.unidades.has(u.id));

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    const s = (k: string) => String(fd.get(k) ?? "").trim();
    const payload = {
      title: s("title"),
      slug: s("slug"),
      summary: s("summary"),
      description: s("description"),
      category: s("category"),
      audience: s("audience"),
      requirements: lines(s("requirements")),
      steps: lines(s("steps")),
      channels: channels.filter((c) => c.label && c.value),
      sla: s("sla"),
      cost: s("cost"),
      icon: s("icon"),
      responsible_unidade_id: s("responsible_unidade_id") || null,
      position: Number(s("position")) || 0,
    };
    const ok = await run(
      () => (service ? apiClient.put(`v1/catalog/admin/services/${service.id}`, payload) : apiClient.post("v1/catalog/admin/services", payload)),
      "Serviço salvo",
    );
    if (ok) onDone();
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-3">
      <div className="grid gap-3 sm:grid-cols-2">
        <Input id="svc-title" name="title" label="Título *" required maxLength={200} defaultValue={service?.title} />
        <Input id="svc-slug" name="slug" label="Slug" maxLength={100} defaultValue={service?.slug} placeholder="gerado do título" />
      </div>
      <Textarea id="svc-summary" name="summary" label="Resumo" rows={2} maxLength={500} defaultValue={service?.summary} />
      <div className="grid gap-3 sm:grid-cols-3">
        <Input id="svc-category" name="category" label="Categoria" maxLength={80} defaultValue={service?.category} />
        <Input id="svc-audience" name="audience" label="Público" maxLength={200} defaultValue={service?.audience} />
        <Select id="svc-icon" name="icon" label="Ícone" placeholder="Padrão" defaultValue={service?.icon ?? ""} options={SERVICE_ICON_NAMES.map((n) => ({ value: n, label: n }))} />
      </div>
      <div className="grid gap-3 sm:grid-cols-4">
        <Input id="svc-sla" name="sla" label="Prazo" maxLength={120} defaultValue={service?.sla} placeholder="até 15 dias" />
        <Input id="svc-cost" name="cost" label="Custo" maxLength={120} defaultValue={service?.cost} placeholder="Gratuito" />
        <Select
          id="svc-unidade"
          name="responsible_unidade_id"
          label="Unidade responsável"
          placeholder={geridas.global ? "Institucional (sem unidade)" : "Selecione…"}
          required={!geridas.global}
          defaultValue={service?.responsible_unidade_id ?? ""}
          options={unidades.map((u) => ({ value: u.id, label: u.nome }))}
        />
        <Input id="svc-position" name="position" type="number" min={0} label="Ordem" defaultValue={service?.position ?? 0} />
      </div>
      <Textarea id="svc-description" name="description" label="Descrição (Markdown)" rows={6} maxLength={50000} defaultValue={service?.description} />
      <div className="grid gap-3 sm:grid-cols-2">
        <Textarea id="svc-req" name="requirements" label="Requisitos (um por linha)" rows={4} defaultValue={service?.requirements.join("\n")} />
        <Textarea id="svc-steps" name="steps" label="Etapas (uma por linha)" rows={4} defaultValue={service?.steps.join("\n")} />
      </div>
      <fieldset className="flex flex-col gap-2">
        <legend className="text-sm font-medium">Canais de atendimento</legend>
        {channels.map((c, i) => (
          <div key={i} className="grid gap-2 sm:grid-cols-[9rem_1fr_1fr_auto] sm:items-end">
            <Select
              id={`ch-type-${i}`}
              label="Tipo"
              value={c.type}
              onChange={(e) => setChannels(channels.map((x, j) => (j === i ? { ...x, type: e.target.value as ServiceChannel["type"] } : x)))}
              options={[
                { value: "online", label: "Online" },
                { value: "presencial", label: "Presencial" },
                { value: "telefone", label: "Telefone" },
                { value: "email", label: "E-mail" },
              ]}
            />
            <Input id={`ch-label-${i}`} label="Rótulo" value={c.label} maxLength={120} onChange={(e) => setChannels(channels.map((x, j) => (j === i ? { ...x, label: e.target.value } : x)))} />
            <Input id={`ch-value-${i}`} label="Valor" value={c.value} maxLength={300} onChange={(e) => setChannels(channels.map((x, j) => (j === i ? { ...x, value: e.target.value } : x)))} />
            <Button type="button" variant="ghost" size="sm" aria-label="Remover canal" onClick={() => setChannels(channels.filter((_, j) => j !== i))}>
              <Trash2 size={14} aria-hidden="true" />
            </Button>
          </div>
        ))}
        <Button type="button" variant="secondary" size="sm" className="self-start" onClick={() => setChannels([...channels, { type: "online", label: "", value: "" }])}>
          <Plus size={14} aria-hidden="true" className="mr-1" /> Canal
        </Button>
      </fieldset>
      <div className="flex justify-end">
        <Button type="submit" loading={pending}>
          Salvar serviço
        </Button>
      </div>
    </form>
  );
}

export default function GestaoServicosPage() {
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);
  const [editing, setEditing] = useState<CatalogService | "new" | null>(null);
  const list = useApiPage<CatalogService>(withQuery("v1/catalog/admin/services", { status, page, page_size: 25 }));
  const { run } = useAction();
  const refresh = () => void list.mutate();
  const transition = (s: CatalogService, action: "publish" | "unpublish" | "archive", msg: string) =>
    run(() => apiClient.post(`v1/catalog/admin/services/${s.id}/${action}`), msg).then(refresh);

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Catálogo de serviços"
        title="Gerenciar serviços"
        description="Carta de serviços publicada no site institucional (/servicos)."
        actions={
          <>
            <Link href="/servicos" target="_blank" className="inline-flex items-center gap-1 text-sm text-primary hover:underline">
              Ver site <ExternalLink size={12} aria-hidden="true" />
            </Link>
            <Button onClick={() => setEditing("new")}>
              <Plus size={16} aria-hidden="true" className="mr-1" /> Novo serviço
            </Button>
          </>
        }
      />
      <Select
        id="gs-status"
        label="Situação"
        value={status}
        onChange={(e) => {
          setPage(1);
          setStatus(e.target.value);
        }}
        options={[{ value: "", label: "Todas" }, ...Object.entries(STATUS).map(([value, s]) => ({ value, label: s.label }))]}
        className="max-w-xs"
      />
      <DataState loading={list.isLoading} error={list.error} onRetry={refresh} empty={(list.data?.items ?? []).length === 0} emptyTitle="Nenhum serviço">
        <Table caption="Serviços do catálogo">
          <TableHead>
            <TableRow>
              <TableHeaderCell>Serviço</TableHeaderCell>
              <TableHeaderCell>Categoria</TableHeaderCell>
              <TableHeaderCell>Situação</TableHeaderCell>
              <TableHeaderCell>Atualizado</TableHeaderCell>
              <TableHeaderCell>
                <span className="sr-only">Ações</span>
              </TableHeaderCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {(list.data?.items ?? []).map((s) => (
              <TableRow key={s.id}>
                <TableCell>
                  <span className="font-medium">{s.title}</span>
                  <span className="block text-xs text-muted">/{s.slug}</span>
                </TableCell>
                <TableCell className="text-sm">{s.category || "—"}</TableCell>
                <TableCell>
                  <Badge tone={STATUS[s.status].tone}>{STATUS[s.status].label}</Badge>
                </TableCell>
                <TableCell className="text-xs text-muted">{fmtDateTime(s.updated_at)}</TableCell>
                <TableCell className="text-right">
                  <span className="inline-flex gap-1">
                    <Button variant="ghost" size="sm" aria-label={`Editar ${s.title}`} onClick={() => setEditing(s)}>
                      <Pencil size={14} aria-hidden="true" />
                    </Button>
                    {s.status !== "published" ? (
                      <Button variant="ghost" size="sm" aria-label={`Publicar ${s.title}`} onClick={() => void transition(s, "publish", "Serviço publicado")}>
                        <Send size={14} aria-hidden="true" />
                      </Button>
                    ) : (
                      <Button variant="ghost" size="sm" aria-label={`Despublicar ${s.title}`} onClick={() => void transition(s, "unpublish", "Serviço despublicado")}>
                        <Undo2 size={14} aria-hidden="true" />
                      </Button>
                    )}
                    {s.status !== "archived" && (
                      <Button variant="ghost" size="sm" aria-label={`Arquivar ${s.title}`} onClick={() => void transition(s, "archive", "Serviço arquivado")}>
                        <Archive size={14} aria-hidden="true" />
                      </Button>
                    )}
                    <ConfirmButton
                      aria-label={`Excluir ${s.title}`}
                      title={`Excluir "${s.title}"?`}
                      confirmLabel="Excluir"
                      onConfirm={() => run(() => apiClient.delete(`v1/catalog/admin/services/${s.id}`), "Serviço excluído").then(refresh)}
                    >
                      <Trash2 size={14} aria-hidden="true" />
                    </ConfirmButton>
                  </span>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        <Pagination meta={list.data?.meta} onPage={setPage} />
      </DataState>
      <Dialog open={editing !== null} onClose={() => setEditing(null)} title={editing === "new" ? "Novo serviço" : "Editar serviço"} size="xl">
        {editing && (
          <ServiceForm
            key={editing === "new" ? "new" : editing.id}
            service={editing === "new" ? undefined : editing}
            onDone={() => {
              setEditing(null);
              refresh();
            }}
          />
        )}
      </Dialog>
    </div>
  );
}
