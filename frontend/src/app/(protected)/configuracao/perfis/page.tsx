"use client";

import { Pencil, Plus, ShieldCheck, Trash2 } from "lucide-react";
import { useState, type FormEvent } from "react";

import { ConfirmButton } from "@/components/nexus/ConfirmButton";
import { DataState } from "@/components/nexus/DataState";
import { useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import type { Perfil, PermissionGroup } from "@/lib/nexus/types";

/** Formulário de Perfil: as permissões vêm do catálogo que cada plug-in
 * declara no seu Manifest (GET /iam/permissions) — nada de decorar
 * strings "recurso:ação". Curingas ("*", "recurso:*") são preservados. */
function PerfilForm({ perfil, groups, onDone }: { perfil?: Perfil; groups: PermissionGroup[]; onDone: () => void }) {
  const { run, pending } = useAction();
  const [selected, setSelected] = useState<Set<string>>(new Set(perfil?.permissoes ?? []));
  const known = new Set(groups.flatMap((g) => g.permissions.map((p) => p.key)));
  const extra = [...selected].filter((p) => !known.has(p));
  const readOnly = perfil?.sistema ?? false;

  function toggle(key: string, on: boolean) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (on) next.add(key);
      else next.delete(key);
      return next;
    });
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    const body = {
      nome: String(fd.get("nome") ?? "").trim(),
      slug: String(fd.get("slug") ?? "").trim() || undefined,
      descricao: String(fd.get("descricao") ?? "").trim(),
      permissoes: [...selected].sort(),
      ativo: fd.get("ativo") === "on",
    };
    const ok = await run(
      () => (perfil ? apiClient.put(`v1/iam/perfis/${perfil.id}`, body) : apiClient.post("v1/iam/perfis", body)),
      "Perfil salvo",
    );
    if (ok) onDone();
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4">
      {readOnly && (
        <p className="rounded-md bg-accent/10 p-3 text-xs text-accent">Perfil de sistema: as permissões podem ser ajustadas, mas o perfil não pode ser excluído.</p>
      )}
      <div className="grid gap-3 sm:grid-cols-2">
        <Input id="perfil-nome" name="nome" label="Nome *" required maxLength={120} defaultValue={perfil?.nome} />
        <Input id="perfil-slug" name="slug" label="Slug" maxLength={80} defaultValue={perfil?.slug} disabled={readOnly} />
      </div>
      <Textarea id="perfil-descricao" name="descricao" label="Descrição" rows={2} maxLength={500} defaultValue={perfil?.descricao} />
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" name="ativo" defaultChecked={perfil?.ativo ?? true} className="h-4 w-4 accent-primary" /> Ativo
      </label>

      <fieldset className="flex flex-col gap-3">
        <legend className="mb-2 text-sm font-semibold">Permissões ({selected.size})</legend>
        {groups.map((g) => (
          <div key={g.module} className="rounded-lg border border-surface-border p-3">
            <div className="mb-2 flex items-center justify-between">
              <p className="text-sm font-medium">{g.name}</p>
              <label className="flex items-center gap-1 text-xs text-muted">
                <input
                  type="checkbox"
                  className="h-3.5 w-3.5 accent-primary"
                  checked={g.permissions.every((p) => selected.has(p.key))}
                  onChange={(e) => g.permissions.forEach((p) => toggle(p.key, e.target.checked))}
                />
                Todas
              </label>
            </div>
            <ul className="grid gap-1 sm:grid-cols-2">
              {g.permissions.map((p) => (
                <li key={p.key}>
                  <label className="flex items-start gap-2 text-xs">
                    <input type="checkbox" className="mt-0.5 h-3.5 w-3.5 accent-primary" checked={selected.has(p.key)} onChange={(e) => toggle(p.key, e.target.checked)} />
                    <span>
                      <code className="font-mono">{p.key}</code>
                      <span className="block text-muted">{p.description}</span>
                    </span>
                  </label>
                </li>
              ))}
            </ul>
          </div>
        ))}
        {extra.length > 0 && (
          <p className="text-xs text-muted">
            Também concedidas (curingas ou de módulos não carregados):{" "}
            {extra.map((p) => (
              <button key={p} type="button" onClick={() => toggle(p, false)} className="mr-1 rounded bg-surface-hover px-1.5 font-mono" aria-label={`Remover ${p}`}>
                {p} ×
              </button>
            ))}
          </p>
        )}
      </fieldset>

      <div className="flex justify-end">
        <Button type="submit" loading={pending}>
          Salvar perfil
        </Button>
      </div>
    </form>
  );
}

export default function PerfisPage() {
  const perfis = useApiQuery<Perfil[]>("v1/iam/perfis");
  const groups = useApiQuery<PermissionGroup[]>("v1/iam/permissions");
  const [editing, setEditing] = useState<Perfil | "new" | null>(null);
  const { run } = useAction();

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="max-w-2xl text-sm text-muted">
          Um Perfil é um conjunto de permissões <code>recurso:ação</code>. Usuários recebem perfis por lotação manual ou pelo mapeamento
          de grupos do AD, sempre num escopo (Entidade, Unidade ou Departamento).
        </p>
        <Button onClick={() => setEditing("new")}>
          <Plus size={16} aria-hidden="true" className="mr-1" /> Novo perfil
        </Button>
      </div>

      <DataState loading={perfis.isLoading} error={perfis.error} onRetry={() => void perfis.mutate()} empty={(perfis.data ?? []).length === 0} emptyTitle="Nenhum perfil">
        <ul className="grid gap-3 md:grid-cols-2">
          {(perfis.data ?? []).map((p) => (
            <li key={p.id}>
              <Card className="flex h-full flex-col gap-2 p-4">
                <div className="flex items-start justify-between gap-2">
                  <div className="flex items-center gap-2">
                    <ShieldCheck size={18} aria-hidden="true" className="text-primary" />
                    <h2 className="font-semibold">{p.nome}</h2>
                    <code className="text-xs text-muted">{p.slug}</code>
                  </div>
                  <span className="flex items-center gap-1">
                    <Button variant="ghost" size="sm" aria-label={`Editar ${p.nome}`} onClick={() => setEditing(p)}>
                      <Pencil size={14} aria-hidden="true" />
                    </Button>
                    {!p.sistema && (
                      <ConfirmButton
                        aria-label={`Excluir ${p.nome}`}
                        title={`Excluir o perfil "${p.nome}"?`}
                        description="Lotações e mapeamentos AD que usam este perfil impedem a exclusão."
                        confirmLabel="Excluir"
                        onConfirm={() => run(() => apiClient.delete(`v1/iam/perfis/${p.id}`), "Perfil excluído").then(() => perfis.mutate())}
                      >
                        <Trash2 size={14} aria-hidden="true" />
                      </ConfirmButton>
                    )}
                  </span>
                </div>
                {p.descricao && <p className="text-sm text-muted">{p.descricao}</p>}
                <div className="mt-auto flex flex-wrap gap-1">
                  {p.sistema && <Badge tone="info">Sistema</Badge>}
                  {!p.ativo && <Badge tone="warning">Inativo</Badge>}
                  <Badge>{p.permissoes.includes("*") ? "Acesso total (*)" : `${p.permissoes.length} permissões`}</Badge>
                </div>
              </Card>
            </li>
          ))}
        </ul>
      </DataState>

      <Dialog open={editing !== null} onClose={() => setEditing(null)} size="xl" title={editing === "new" ? "Novo perfil" : `Editar perfil${editing ? ` — ${editing.nome}` : ""}`}>
        {editing && (
          <PerfilForm
            key={editing === "new" ? "new" : editing.id}
            perfil={editing === "new" ? undefined : editing}
            groups={groups.data ?? []}
            onDone={() => {
              setEditing(null);
              void perfis.mutate();
            }}
          />
        )}
      </Dialog>
    </div>
  );
}
