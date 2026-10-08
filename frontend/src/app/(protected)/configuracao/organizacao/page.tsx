"use client";

import { Building2, Pencil, Plus, Trash2 } from "lucide-react";
import { useState, type FormEvent } from "react";

import { ConfirmButton } from "@/components/nexus/ConfirmButton";
import { DataState } from "@/components/nexus/DataState";
import { useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { hasPermission, unidadesGeridas } from "@/lib/nexus/permissions";
import type { Departamento, Entidade, OrgTree, Unidade } from "@/lib/nexus/types";

type Kind = "entidade" | "unidade" | "departamento";

interface Editing {
  kind: Kind;
  /** Registro existente (edição) ou vazio (criação). */
  record?: Partial<Entidade & Unidade & Departamento>;
  /** Pai na criação: entidade_id para unidade, unidade_id para departamento. */
  parentId?: string;
}

const LABEL: Record<Kind, string> = { entidade: "Entidade", unidade: "Unidade", departamento: "Departamento" };
const PATH: Record<Kind, string> = { entidade: "v1/iam/entidades", unidade: "v1/iam/unidades", departamento: "v1/iam/departamentos" };

function OrgForm({ editing, onDone }: { editing: Editing; onDone: () => void }) {
  const { run, pending } = useAction();
  const r = editing.record ?? {};
  const { kind } = editing;

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    const s = (k: string) => String(fd.get(k) ?? "").trim();
    const body: Record<string, unknown> = { nome: s("nome"), sigla: s("sigla"), slug: s("slug") || undefined, ativo: fd.get("ativo") === "on" };
    if (kind === "entidade") body.documento = s("documento");
    if (kind !== "entidade") Object.assign(body, { ad_group: s("ad_group"), email: s("email"), telefone: s("telefone") });
    if (kind === "unidade") Object.assign(body, { entidade_id: r.entidade_id ?? editing.parentId, parent_id: r.parent_id, endereco: s("endereco") });
    if (kind === "departamento") body.unidade_id = r.unidade_id ?? editing.parentId;

    const ok = await run(
      () => (r.id ? apiClient.put(`${PATH[kind]}/${r.id}`, body) : apiClient.post(PATH[kind], body)),
      `${LABEL[kind]} salva`,
    );
    if (ok) onDone();
  }

  const id = (f: string) => `org-${kind}-${f}`;
  return (
    <form id="org-form" onSubmit={submit} className="flex flex-col gap-3">
      <Input id={id("nome")} name="nome" label="Nome *" required maxLength={200} defaultValue={r.nome} />
      <div className="grid gap-3 sm:grid-cols-2">
        <Input id={id("sigla")} name="sigla" label="Sigla" maxLength={30} defaultValue={r.sigla} />
        <Input id={id("slug")} name="slug" label="Slug (URL)" maxLength={80} defaultValue={r.slug} placeholder="gerado a partir do nome" />
      </div>
      {kind === "entidade" && <Input id={id("documento")} name="documento" label="CNPJ / documento" maxLength={40} defaultValue={r.documento} />}
      {kind !== "entidade" && (
        <>
          <Input
            id={id("ad_group")}
            name="ad_group"
            label="Grupo do Active Directory"
            maxLength={200}
            defaultValue={r.ad_group}
            placeholder="CN=GRP_SETOR,OU=Grupos,DC=org,DC=gov,DC=br"
          />
          <div className="grid gap-3 sm:grid-cols-2">
            <Input id={id("email")} name="email" type="email" label="E-mail" maxLength={200} defaultValue={r.email} />
            <Input id={id("telefone")} name="telefone" label="Telefone" maxLength={40} defaultValue={r.telefone} />
          </div>
        </>
      )}
      {kind === "unidade" && <Input id={id("endereco")} name="endereco" label="Endereço" maxLength={300} defaultValue={r.endereco} />}
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" name="ativo" defaultChecked={r.ativo ?? true} className="h-4 w-4 accent-primary" /> Ativo
      </label>
      <div className="flex justify-end">
        <Button type="submit" loading={pending}>
          Salvar
        </Button>
      </div>
    </form>
  );
}

export default function OrganizacaoPage() {
  const { data: tree, error, isLoading, mutate } = useApiQuery<OrgTree[]>("v1/iam/org-tree");
  const [editing, setEditing] = useState<Editing | null>(null);
  const { run } = useAction();
  const { me } = useNexus();
  // Administração delegada (ADR 013): a global faz tudo; a delegada, só o
  // que cobre — entidades só a global. A API confere de novo.
  const geridas = unidadesGeridas(me, tree, "iam:manage");
  const naEntidade = (entidadeId: string) =>
    geridas.global ||
    (me?.scopes ?? []).some((s) => s.entidade_id === entidadeId && !s.unidade_id && !s.departamento_id && hasPermission(s.permissions, "iam:manage"));
  const naUnidade = (unidadeId?: string) => !!unidadeId && geridas.unidades.has(unidadeId);

  const remove = (kind: Kind, id: string) => run(() => apiClient.delete(`${PATH[kind]}/${id}`), `${LABEL[kind]} removida`).then(() => mutate());

  const actions = (kind: Kind, record: Editing["record"] & { id: string; nome: string }, pode: { editar: boolean; excluir: boolean }) => (
    <span className="flex shrink-0 items-center gap-1">
      {pode.editar && (
        <Button variant="ghost" size="sm" aria-label={`Editar ${record.nome}`} onClick={() => setEditing({ kind, record })}>
          <Pencil size={14} aria-hidden="true" />
        </Button>
      )}
      {pode.excluir && (
        <ConfirmButton
          aria-label={`Excluir ${record.nome}`}
          title={`Excluir ${LABEL[kind].toLowerCase()} "${record.nome}"?`}
          description="A exclusão é recusada se houver itens vinculados (lotações, mapeamentos AD ou estrutura filha)."
          confirmLabel="Excluir"
          onConfirm={() => remove(kind, record.id)}
        >
          <Trash2 size={14} aria-hidden="true" />
        </ConfirmButton>
      )}
    </span>
  );

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="max-w-2xl text-sm text-muted">
          Escopos organizacionais do RBAC multi-tenant. Unidades e departamentos podem ser vinculados a um grupo do AD, usado na
          sincronização do Diretório e nas salas departamentais do Mercúrio.
        </p>
        {geridas.global && (
          <Button onClick={() => setEditing({ kind: "entidade" })}>
            <Plus size={16} aria-hidden="true" className="mr-1" /> Nova entidade
          </Button>
        )}
      </div>

      <DataState
        loading={isLoading}
        error={error}
        onRetry={() => void mutate()}
        empty={(tree ?? []).length === 0}
        emptyTitle="Nenhuma entidade cadastrada"
        emptyDescription="Comece cadastrando a entidade (órgão ou empresa) raiz."
      >
        <ul className="flex flex-col gap-4">
          {(tree ?? []).map((ent) => (
            <li key={ent.id}>
              <Card className="p-5">
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-center gap-3">
                    <Building2 size={20} aria-hidden="true" className="text-primary" />
                    <div>
                      <h2 className="font-semibold">
                        {ent.nome} {ent.sigla && <span className="text-muted">({ent.sigla})</span>}
                      </h2>
                      <p className="text-xs text-muted">
                        {ent.documento || "sem documento"} · {ent.unidades.length} unidade(s)
                      </p>
                    </div>
                    {!ent.ativo && <Badge tone="warning">Inativa</Badge>}
                  </div>
                  <span className="flex items-center gap-1">
                    {naEntidade(ent.id) && (
                      <Button variant="secondary" size="sm" onClick={() => setEditing({ kind: "unidade", parentId: ent.id })}>
                        <Plus size={14} aria-hidden="true" className="mr-1" /> Unidade
                      </Button>
                    )}
                    {actions("entidade", ent, { editar: geridas.global, excluir: geridas.global })}
                  </span>
                </div>

                {ent.unidades.length > 0 && (
                  <ul className="mt-4 flex flex-col gap-2 border-l-2 border-surface-border pl-4">
                    {ent.unidades.map((un) => (
                      <li key={un.id} className="flex flex-col gap-2">
                        <div className="flex items-center justify-between gap-2 rounded-md bg-surface-hover/60 px-3 py-2">
                          <div className="min-w-0">
                            <p className="text-sm font-medium">
                              {un.nome} {un.sigla && <span className="text-muted">({un.sigla})</span>}
                              {!un.ativo && <Badge tone="warning" className="ml-2">Inativa</Badge>}
                            </p>
                            {un.ad_group && <p className="truncate font-mono text-[11px] text-muted">{un.ad_group}</p>}
                          </div>
                          <span className="flex items-center gap-1">
                            {naUnidade(un.id) && (
                              <>
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  aria-label={`Nova subunidade de ${un.nome}`}
                                  onClick={() => setEditing({ kind: "unidade", parentId: ent.id, record: { parent_id: un.id } })}
                                >
                                  <Plus size={14} aria-hidden="true" className="mr-1" /> Sub.
                                </Button>
                                <Button variant="ghost" size="sm" onClick={() => setEditing({ kind: "departamento", parentId: un.id })}>
                                  <Plus size={14} aria-hidden="true" className="mr-1" /> Depto.
                                </Button>
                              </>
                            )}
                            {actions("unidade", un, { editar: naUnidade(un.id), excluir: un.parent_id ? naUnidade(un.parent_id) : naEntidade(ent.id) })}
                          </span>
                        </div>
                        {un.departamentos.length > 0 && (
                          <ul className="ml-4 flex flex-col gap-1">
                            {un.departamentos.map((d) => (
                              <li key={d.id} className="flex items-center justify-between gap-2 px-3 py-1 text-sm">
                                <span className="min-w-0">
                                  {d.nome} {d.sigla && <span className="text-muted">({d.sigla})</span>}
                                  {d.ad_group && <span className="ml-2 font-mono text-[11px] text-muted">{d.ad_group}</span>}
                                </span>
                                {actions("departamento", d, { editar: naUnidade(un.id), excluir: naUnidade(un.id) })}
                              </li>
                            ))}
                          </ul>
                        )}
                      </li>
                    ))}
                  </ul>
                )}
              </Card>
            </li>
          ))}
        </ul>
      </DataState>

      <Dialog
        open={editing !== null}
        onClose={() => setEditing(null)}
        size="lg"
        title={editing ? `${editing.record?.id ? "Editar" : "Nova"} ${LABEL[editing.kind].toLowerCase()}` : ""}
      >
        {editing && (
          <OrgForm
            key={`${editing.kind}-${editing.record?.id ?? "new"}`}
            editing={editing}
            onDone={() => {
              setEditing(null);
              void mutate();
            }}
          />
        )}
      </Dialog>
    </div>
  );
}
