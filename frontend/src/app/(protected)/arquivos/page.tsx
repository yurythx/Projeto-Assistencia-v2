"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Download, File as FileIcon, Folder as FolderIcon, FolderPlus, Pencil, Plus, Shield, Trash2, Upload } from "lucide-react";
import { Suspense, useRef, useState, type FormEvent } from "react";

import { UnidadeDonaSelect } from "@/components/iam/UnidadeDonaSelect";
import { ConfirmButton } from "@/components/nexus/ConfirmButton";
import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { fmtBytes, fmtDateTime, useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Table, TableBody, TableCell, TableHead, TableHeaderCell, TableRow } from "@/components/ui/Table";
import { apiClient } from "@/lib/api/client";
import { useApiQuery, withQuery } from "@/lib/api/swr";
import type { ACLEntry, FileItem, Folder, Listing, OrgTree, Perfil, UploadTicket } from "@/lib/nexus/types";
import { mimeOf, putToTicket } from "@/lib/nexus/upload";

const SUBJECT_LABEL: Record<ACLEntry["subject_type"], string> = {
  everyone: "Todos os autenticados",
  user: "Usuário (id)",
  perfil: "Perfil",
  ad_group: "Grupo do AD",
  unidade: "Unidade",
  departamento: "Departamento",
};

function ACLEditor({ folder, onDone }: { folder: Folder; onDone: () => void }) {
  const acl = useApiQuery<ACLEntry[]>(`v1/files/folders/${folder.id}/acl`);
  const perfis = useApiQuery<Perfil[]>("v1/iam/perfis");
  const tree = useApiQuery<OrgTree[]>("v1/iam/org-tree");
  const { run, pending } = useAction();
  const [entries, setEntries] = useState<ACLEntry[] | null>(null);
  const [draft, setDraft] = useState<ACLEntry>({ subject_type: "perfil", subject: "", can_write: false });
  const list = entries ?? acl.data ?? [];

  const unidades = (tree.data ?? []).flatMap((e) => e.unidades);
  const subjectOptions: Record<string, { value: string; label: string }[]> = {
    perfil: (perfis.data ?? []).map((p) => ({ value: p.slug, label: p.nome })),
    unidade: unidades.map((u) => ({ value: u.id, label: u.nome })),
    departamento: unidades.flatMap((u) => u.departamentos.map((d) => ({ value: d.id, label: `${d.nome} (${u.sigla || u.nome})` }))),
  };
  const labelFor = (e: ACLEntry) =>
    e.subject_type === "everyone" ? "" : subjectOptions[e.subject_type]?.find((o) => o.value === e.subject)?.label ?? e.subject;

  return (
    <div className="flex flex-col gap-4">
      <p className="text-sm text-muted">
        As permissões são herdadas pelas subpastas. Sem nenhuma entrada, só o dono e administradores de arquivos acessam.
      </p>
      <ul className="flex flex-col divide-y divide-surface-border rounded-lg border border-surface-border">
        {list.length === 0 && <li className="p-3 text-sm text-muted">Sem entradas — pasta privada.</li>}
        {list.map((e, i) => (
          <li key={`${e.subject_type}-${e.subject}-${i}`} className="flex items-center justify-between gap-2 px-3 py-2 text-sm">
            <span>
              <Badge>{SUBJECT_LABEL[e.subject_type]}</Badge> {labelFor(e)} · {e.can_write ? "leitura e escrita" : "somente leitura"}
            </span>
            <Button variant="ghost" size="sm" aria-label="Remover entrada" onClick={() => setEntries(list.filter((_, j) => j !== i))}>
              <Trash2 size={14} aria-hidden="true" />
            </Button>
          </li>
        ))}
      </ul>
      <div className="grid gap-3 rounded-lg border border-dashed border-surface-border p-3 sm:grid-cols-[1fr_1fr_auto_auto] sm:items-end">
        <Select
          id="acl-type"
          label="Sujeito"
          value={draft.subject_type}
          onChange={(e) => setDraft({ ...draft, subject_type: e.target.value as ACLEntry["subject_type"], subject: "" })}
          options={Object.entries(SUBJECT_LABEL).map(([value, label]) => ({ value, label }))}
        />
        {subjectOptions[draft.subject_type] ? (
          <Select id="acl-subject" label="Qual" placeholder="Selecione…" value={draft.subject} onChange={(e) => setDraft({ ...draft, subject: e.target.value })} options={subjectOptions[draft.subject_type]!} />
        ) : (
          <Input
            id="acl-subject"
            label="Qual"
            value={draft.subject}
            disabled={draft.subject_type === "everyone"}
            onChange={(e) => setDraft({ ...draft, subject: e.target.value.trim() })}
            placeholder={draft.subject_type === "user" ? "UUID do usuário" : draft.subject_type === "ad_group" ? "GRP_EQUIPE" : ""}
          />
        )}
        <label className="flex items-center gap-2 pb-2 text-sm">
          <input type="checkbox" checked={draft.can_write} onChange={(e) => setDraft({ ...draft, can_write: e.target.checked })} className="h-4 w-4 accent-primary" /> Escrita
        </label>
        <Button
          type="button"
          variant="secondary"
          disabled={draft.subject_type !== "everyone" && !draft.subject}
          onClick={() => {
            setEntries([...list, draft]);
            setDraft({ ...draft, subject: "" });
          }}
        >
          <Plus size={14} aria-hidden="true" className="mr-1" /> Adicionar
        </Button>
      </div>
      <div className="flex justify-end">
        <Button
          loading={pending}
          disabled={entries === null}
          onClick={() => void run(() => apiClient.put(`v1/files/folders/${folder.id}/acl`, { entries: list }), "Permissões salvas").then((ok) => ok !== undefined && onDone())}
        >
          Salvar permissões
        </Button>
      </div>
    </div>
  );
}

/** Com `unidade`, também escolhe a unidade dona da pasta (ADR 013): as
 * unidades em que a pessoa está lotada ou que gerencia, e a atual. */
function NameDialog({
  title,
  initial,
  unidade,
  onSubmit,
  onClose,
}: {
  title: string;
  initial?: string;
  unidade?: { initial?: string | null };
  onSubmit: (name: string, unidadeId: string | null) => Promise<unknown>;
  onClose: () => void;
}) {
  const [busy, setBusy] = useState(false);
  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    const name = String(fd.get("name") ?? "").trim();
    if (!name) return;
    setBusy(true);
    try {
      await onSubmit(name, String(fd.get("unidade_id") ?? "") || null);
    } finally {
      setBusy(false);
    }
  }
  return (
    <Dialog open onClose={onClose} title={title}>
      <form onSubmit={submit} className="flex flex-col gap-3">
        <Input id="name-dialog" name="name" label="Nome" required maxLength={255} defaultValue={initial} autoFocus />
        {unidade && (
          <UnidadeDonaSelect id="folder-unidade" permission="files:manage" lotacao defaultValue={unidade.initial} placeholder="Sem unidade (vale a da pasta acima)" />
        )}
        <div className="flex justify-end">
          <Button type="submit" loading={busy}>
            Salvar
          </Button>
        </div>
      </form>
    </Dialog>
  );
}

function Arquivos() {
  const params = useSearchParams();
  const router = useRouter();
  const folderId = params.get("pasta") ?? undefined;
  const listing = useApiQuery<Listing>(withQuery("v1/files/browse", { folder_id: folderId }));
  const { run } = useAction();
  const fileInput = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState<string | null>(null);
  const [dialog, setDialog] = useState<{ kind: "new-folder" } | { kind: "rename-folder"; folder: Folder } | { kind: "rename-file"; file: FileItem } | { kind: "acl"; folder: Folder } | null>(null);
  const l = listing.data;
  const refresh = () => void listing.mutate();

  async function upload(files: FileList) {
    if (!folderId) return;
    for (const file of Array.from(files)) {
      setUploading(file.name);
      await run(async () => {
        const { data } = await apiClient.post<{ file: FileItem; upload: UploadTicket }>("v1/files/uploads", {
          folder_id: folderId,
          filename: file.name,
          content_type: mimeOf(file),
          size: file.size,
        });
        await putToTicket(data.upload, file);
        await apiClient.post(`v1/files/objects/${data.file.id}/confirm`);
      }, `${file.name} enviado`);
    }
    setUploading(null);
    refresh();
  }

  async function download(f: FileItem) {
    const res = await run(() => apiClient.get<{ url: string }>(`v1/files/objects/${f.id}/download`));
    if (res) window.location.assign(res.data.url);
  }

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        eyebrow="Arquivos"
        title={l?.folder?.name ?? "Pastas"}
        description={!folderId ? "Pastas de trabalho compartilhadas, com permissões por perfil, grupo do AD ou unidade." : undefined}
        actions={
          <>
            {(!folderId || l?.access.write) && (
              <Button variant="secondary" onClick={() => setDialog({ kind: "new-folder" })}>
                <FolderPlus size={16} aria-hidden="true" className="mr-1" /> Nova pasta
              </Button>
            )}
            {folderId && l?.access.write && (
              <>
                <input ref={fileInput} type="file" multiple className="sr-only" id="file-upload" onChange={(e) => e.target.files && void upload(e.target.files)} tabIndex={-1} />
                <Button onClick={() => fileInput.current?.click()} loading={uploading !== null}>
                  <Upload size={16} aria-hidden="true" className="mr-1" /> {uploading ? `Enviando ${uploading}…` : "Enviar arquivos"}
                </Button>
              </>
            )}
            {l?.folder && l.access.manage && (
              <Button variant="ghost" onClick={() => setDialog({ kind: "acl", folder: l.folder! })}>
                <Shield size={16} aria-hidden="true" className="mr-1" /> Permissões
              </Button>
            )}
          </>
        }
      />

      <nav aria-label="Caminho" className="flex flex-wrap items-center gap-1 text-sm text-muted">
        <Link href="/arquivos" className="hover:text-foreground hover:underline">
          Raiz
        </Link>
        {(l?.breadcrumbs ?? []).map((b) => (
          <span key={b.id}>
            {" / "}
            <Link href={`/arquivos?pasta=${b.id}`} className="hover:text-foreground hover:underline">
              {b.name}
            </Link>
          </span>
        ))}
        {l?.folder && <span aria-current="page"> / {l.folder.name}</span>}
      </nav>

      <DataState
        loading={listing.isLoading}
        error={listing.error}
        onRetry={refresh}
        empty={!!l && l.folders.length === 0 && l.files.length === 0}
        emptyTitle="Pasta vazia"
      >
        <Table caption="Conteúdo da pasta">
          <TableHead>
            <TableRow>
              <TableHeaderCell>Nome</TableHeaderCell>
              <TableHeaderCell>Tamanho</TableHeaderCell>
              <TableHeaderCell>Dono</TableHeaderCell>
              <TableHeaderCell>Atualizado</TableHeaderCell>
              <TableHeaderCell>
                <span className="sr-only">Ações</span>
              </TableHeaderCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {(l?.folders ?? []).map((f) => (
              <TableRow key={f.id}>
                <TableCell>
                  <Link href={`/arquivos?pasta=${f.id}`} className="inline-flex items-center gap-2 font-medium hover:underline">
                    <FolderIcon size={16} aria-hidden="true" className="text-primary" /> {f.name}
                  </Link>
                </TableCell>
                <TableCell className="text-muted">—</TableCell>
                <TableCell className="text-sm">{f.owner_name}</TableCell>
                <TableCell className="text-xs text-muted">{fmtDateTime(f.updated_at)}</TableCell>
                <TableCell className="text-right">
                  <span className="inline-flex gap-1">
                    <Button variant="ghost" size="sm" aria-label={`Renomear ${f.name}`} onClick={() => setDialog({ kind: "rename-folder", folder: f })}>
                      <Pencil size={14} aria-hidden="true" />
                    </Button>
                    <ConfirmButton
                      aria-label={`Excluir ${f.name}`}
                      title={`Excluir a pasta "${f.name}"?`}
                      description="A pasta, todas as subpastas e todos os arquivos dentro dela serão apagados definitivamente (inclusive do armazenamento). A exclusão fica registrada na auditoria."
                      confirmLabel="Excluir tudo"
                      onConfirm={() => run(() => apiClient.delete(`v1/files/folders/${f.id}?recursive=true`), "Pasta excluída").then(refresh)}
                    >
                      <Trash2 size={14} aria-hidden="true" />
                    </ConfirmButton>
                  </span>
                </TableCell>
              </TableRow>
            ))}
            {(l?.files ?? []).map((f) => (
              <TableRow key={f.id}>
                <TableCell>
                  <span className="inline-flex items-center gap-2">
                    <FileIcon size={16} aria-hidden="true" className="text-muted" /> {f.name}
                    {f.status === "pending" && <Badge tone="warning">envio pendente</Badge>}
                  </span>
                </TableCell>
                <TableCell className="text-sm">{fmtBytes(f.size_bytes)}</TableCell>
                <TableCell className="text-sm">{f.owner_name}</TableCell>
                <TableCell className="text-xs text-muted">{fmtDateTime(f.updated_at)}</TableCell>
                <TableCell className="text-right">
                  <span className="inline-flex gap-1">
                    {f.status === "ready" && (
                      <Button variant="ghost" size="sm" aria-label={`Baixar ${f.name}`} onClick={() => void download(f)}>
                        <Download size={14} aria-hidden="true" />
                      </Button>
                    )}
                    {l?.access.write && (
                      <>
                        <Button variant="ghost" size="sm" aria-label={`Renomear ${f.name}`} onClick={() => setDialog({ kind: "rename-file", file: f })}>
                          <Pencil size={14} aria-hidden="true" />
                        </Button>
                        <ConfirmButton
                          aria-label={`Excluir ${f.name}`}
                          title={`Excluir "${f.name}"?`}
                          confirmLabel="Excluir"
                          onConfirm={() => run(() => apiClient.delete(`v1/files/objects/${f.id}`), "Arquivo excluído").then(refresh)}
                        >
                          <Trash2 size={14} aria-hidden="true" />
                        </ConfirmButton>
                      </>
                    )}
                  </span>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </DataState>

      {dialog?.kind === "new-folder" && (
        <NameDialog
          title="Nova pasta"
          unidade={{}}
          onClose={() => setDialog(null)}
          onSubmit={(name, unidadeId) =>
            run(() => apiClient.post<Folder>("v1/files/folders", { name, parent_id: folderId ?? null, unidade_id: unidadeId }), "Pasta criada").then((res) => {
              if (!res) return;
              setDialog(null);
              if (!folderId) router.push(`/arquivos?pasta=${res.data.id}`);
              else refresh();
            })
          }
        />
      )}
      {dialog?.kind === "rename-folder" && (
        <NameDialog
          title="Editar pasta"
          initial={dialog.folder.name}
          unidade={{ initial: dialog.folder.unidade_id }}
          onClose={() => setDialog(null)}
          onSubmit={(name, unidadeId) =>
            run(
              () =>
                apiClient.patch(`v1/files/folders/${dialog.folder.id}`, { name, parent_id: dialog.folder.parent_id ?? null, move: false, unidade_id: unidadeId, set_unidade: true }),
              "Pasta salva",
            ).then((ok) => {
              if (ok === undefined) return;
              setDialog(null);
              refresh();
            })
          }
        />
      )}
      {dialog?.kind === "rename-file" && (
        <NameDialog
          title="Renomear arquivo"
          initial={dialog.file.name}
          onClose={() => setDialog(null)}
          onSubmit={(name) =>
            run(() => apiClient.patch(`v1/files/objects/${dialog.file.id}`, { name, folder_id: dialog.file.folder_id }), "Arquivo renomeado").then((ok) => {
              if (ok === undefined) return;
              setDialog(null);
              refresh();
            })
          }
        />
      )}
      {dialog?.kind === "acl" && (
        <Dialog open onClose={() => setDialog(null)} title={`Permissões — ${dialog.folder.name}`} size="xl">
          <ACLEditor folder={dialog.folder} onDone={() => setDialog(null)} />
        </Dialog>
      )}
    </div>
  );
}

export default function ArquivosPage() {
  return (
    <Suspense fallback={null}>
      <Arquivos />
    </Suspense>
  );
}
