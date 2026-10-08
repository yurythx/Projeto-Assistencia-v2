"use client";

import { Pencil } from "lucide-react";
import { useState, type FormEvent } from "react";

import { useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { departamentosGeridos } from "@/lib/nexus/permissions";
import type { ChatRoom, OrgTree } from "@/lib/nexus/types";

/** Canais globais e salas departamentais (mapeadas a um departamento e/ou
 * grupo do AD — os membros são resolvidos pelo IAM). */
export function RoomsAdmin({ rooms, onChanged }: { rooms: ChatRoom[]; onChanged: () => void }) {
  const tree = useApiQuery<OrgTree[]>("v1/iam/org-tree");
  const { run, pending } = useAction();
  const [edit, setEdit] = useState<ChatRoom | null>(null);
  const [kind, setKind] = useState<"global" | "department">("global");
  const { me } = useNexus();
  // Moderação com escopo (ADR 013): só os departamentos cobertos; canal
  // global (e sala só por grupo) é da moderação global.
  const geridos = departamentosGeridos(me, tree.data, "mercurio:manage");
  const tipo = geridos.global ? kind : "department";
  const deptos = (tree.data ?? []).flatMap((e) =>
    e.unidades.flatMap((u) =>
      u.departamentos.filter((d) => geridos.departamentos.has(d.id)).map((d) => ({ value: d.id, label: `${d.nome} (${u.sigla || u.nome})` })),
    ),
  );
  const editavel = (r: ChatRoom) => geridos.global || (!!r.departamento_id && geridos.departamentos.has(r.departamento_id));
  const managed = rooms.filter((r) => r.kind !== "direct");

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget;
    const fd = new FormData(form);
    const payload = {
      kind: tipo,
      name: String(fd.get("name") ?? "").trim(),
      description: String(fd.get("description") ?? "").trim(),
      ad_group: String(fd.get("ad_group") ?? "").trim(),
      departamento_id: tipo === "department" ? String(fd.get("departamento_id") ?? "") || null : null,
      archived: fd.get("archived") === "on",
    };
    const ok = await run(() => (edit ? apiClient.put(`v1/mercurio/rooms/${edit.id}`, payload) : apiClient.post("v1/mercurio/rooms", payload)), "Sala salva");
    if (ok) {
      setEdit(null);
      form.reset();
      onChanged();
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <ul className="flex flex-col divide-y divide-surface-border rounded-lg border border-surface-border">
        {managed.map((r) => (
          <li key={r.id} className="flex items-center justify-between gap-2 px-3 py-2 text-sm">
            <span>
              <strong>{r.name}</strong> <Badge className="ml-1">{r.kind === "global" ? "Canal" : "Departamento"}</Badge>
              {r.archived && <Badge tone="warning" className="ml-1">Arquivada</Badge>}
              {r.ad_group && <span className="block font-mono text-[11px] text-muted">{r.ad_group}</span>}
            </span>
            {editavel(r) && (
              <Button
                variant="ghost"
                size="sm"
                aria-label={`Editar ${r.name}`}
                onClick={() => {
                  setEdit(r);
                  setKind(r.kind === "department" ? "department" : "global");
                }}
              >
                <Pencil size={14} aria-hidden="true" />
              </Button>
            )}
          </li>
        ))}
      </ul>
      <form key={edit?.id ?? "new"} onSubmit={submit} className="flex flex-col gap-3 rounded-lg border border-dashed border-surface-border p-3">
        <p className="text-sm font-medium">{edit ? `Editar ${edit.name}` : "Nova sala"}</p>
        <div className="grid gap-3 sm:grid-cols-2">
          <Select
            id="room-kind"
            label="Tipo"
            value={tipo}
            onChange={(e) => setKind(e.target.value as "global" | "department")}
            options={[...(geridos.global ? [{ value: "global", label: "Canal global" }] : []), { value: "department", label: "Sala departamental" }]}
          />
          <Input id="room-name" name="name" label="Nome *" required maxLength={120} defaultValue={edit?.name} />
        </div>
        <Input id="room-desc" name="description" label="Descrição" maxLength={500} defaultValue={edit?.description} />
        {tipo === "department" && (
          <div className="grid gap-3 sm:grid-cols-2">
            <Select
              id="room-depto"
              name="departamento_id"
              label={geridos.global ? "Departamento" : "Departamento *"}
              required={!geridos.global}
              placeholder="—"
              defaultValue={edit?.departamento_id ?? ""}
              options={deptos}
            />
            <Input id="room-ad" name="ad_group" label="Grupo do AD" maxLength={200} defaultValue={edit?.ad_group} />
          </div>
        )}
        {edit && (
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" name="archived" defaultChecked={edit.archived} className="h-4 w-4 accent-primary" /> Arquivada (somente leitura)
          </label>
        )}
        <div className="flex justify-end gap-2">
          {edit && (
            <Button type="button" variant="ghost" onClick={() => setEdit(null)}>
              Cancelar
            </Button>
          )}
          <Button type="submit" loading={pending}>
            Salvar sala
          </Button>
        </div>
      </form>
    </div>
  );
}
