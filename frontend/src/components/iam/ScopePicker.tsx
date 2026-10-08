"use client";

import { useMemo } from "react";

import { Select } from "@/components/ui/Select";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { departamentosGeridos, hasPermission, unidadesGeridas } from "@/lib/nexus/permissions";
import type { OrgTree, ScopeRef } from "@/lib/nexus/types";

/** Escopo organizacional em cascata (Entidade → Unidade → Departamento)
 * a partir de GET /iam/org-tree. Campo vazio = escopo mais amplo.
 *
 * Com `permission` (administração delegada, ADR 013), oferece só o que
 * essa permissão cobre: sem concessão global não há escopo global; sem
 * concessão na entidade, a unidade é obrigatória; sem a unidade inteira,
 * o departamento é obrigatório. A API confere de novo. */
export function ScopePicker({
  value,
  onChange,
  idPrefix,
  required = false,
  permission,
}: {
  value: ScopeRef;
  onChange: (v: ScopeRef) => void;
  idPrefix: string;
  /** Exige ao menos a Entidade. */
  required?: boolean;
  permission?: string;
}) {
  const { me } = useNexus();
  const { data: tree } = useApiQuery<OrgTree[]>("v1/iam/org-tree");

  const alcance = useMemo(() => {
    if (!permission) return null;
    const unidades = unidadesGeridas(me, tree, permission);
    if (unidades.global) return null;
    const departamentos = departamentosGeridos(me, tree, permission).departamentos;
    const entidades = new Set(
      (me?.scopes ?? [])
        .filter((s) => s.entidade_id && !s.unidade_id && !s.departamento_id && hasPermission(s.permissions, permission))
        .map((s) => s.entidade_id!),
    );
    return { entidades, unidades: unidades.unidades, departamentos };
  }, [me, tree, permission]);

  const alcancaUnidade = (u: OrgTree["unidades"][number]) =>
    !alcance || alcance.unidades.has(u.id) || u.departamentos.some((d) => alcance.departamentos.has(d.id));
  const entidades = (tree ?? []).filter((e) => !alcance || alcance.entidades.has(e.id) || e.unidades.some(alcancaUnidade));
  const entidade = useMemo(() => tree?.find((e) => e.id === value.entidade_id), [tree, value.entidade_id]);
  const unidade = useMemo(() => entidade?.unidades.find((u) => u.id === value.unidade_id), [entidade, value.unidade_id]);
  const unidadeObrigatoria = !!alcance && !!entidade && !alcance.entidades.has(entidade.id);
  const departamentoObrigatorio = !!alcance && !!unidade && !alcance.unidades.has(unidade.id);
  const entidadeObrigatoria = required || !!alcance;

  return (
    <fieldset className="grid gap-3 sm:grid-cols-3">
      <legend className="sr-only">Escopo organizacional</legend>
      <Select
        id={`${idPrefix}-entidade`}
        label={entidadeObrigatoria ? "Entidade *" : "Entidade"}
        placeholder={entidadeObrigatoria ? "Selecione…" : "Todas (global)"}
        required={entidadeObrigatoria}
        value={value.entidade_id ?? ""}
        onChange={(e) => onChange({ entidade_id: e.target.value || undefined })}
        options={entidades.map((e) => ({ value: e.id, label: e.sigla ? `${e.sigla} — ${e.nome}` : e.nome }))}
      />
      <Select
        id={`${idPrefix}-unidade`}
        label={unidadeObrigatoria ? "Unidade *" : "Unidade"}
        placeholder={unidadeObrigatoria ? "Selecione…" : "Todas da entidade"}
        required={unidadeObrigatoria}
        disabled={!entidade}
        value={value.unidade_id ?? ""}
        onChange={(e) => onChange({ entidade_id: value.entidade_id, unidade_id: e.target.value || undefined })}
        options={(entidade?.unidades ?? []).filter(alcancaUnidade).map((u) => ({ value: u.id, label: u.sigla ? `${u.sigla} — ${u.nome}` : u.nome }))}
      />
      <Select
        id={`${idPrefix}-departamento`}
        label={departamentoObrigatorio ? "Departamento *" : "Departamento"}
        placeholder={departamentoObrigatorio ? "Selecione…" : "Todos da unidade"}
        required={departamentoObrigatorio}
        disabled={!unidade}
        value={value.departamento_id ?? ""}
        onChange={(e) =>
          onChange({ entidade_id: value.entidade_id, unidade_id: value.unidade_id, departamento_id: e.target.value || undefined })
        }
        options={(unidade?.departamentos ?? [])
          .filter((d) => !alcance || alcance.unidades.has(unidade!.id) || alcance.departamentos.has(d.id))
          .map((d) => ({ value: d.id, label: d.sigla ? `${d.sigla} — ${d.nome}` : d.nome }))}
      />
    </fieldset>
  );
}
