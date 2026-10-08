"use client";

import { Select } from "@/components/ui/Select";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { unidadesGeridas, unidadesLotadas } from "@/lib/nexus/permissions";
import type { OrgTree } from "@/lib/nexus/types";

/** Unidade dona de um conteúdo (ADR 013): oferece só as unidades em que a
 * pessoa tem a permissão de gestão; "institucional" (sem dono) só para a
 * gestão global — para os demais o campo é obrigatório. A API confere de
 * novo.
 *
 * Com `lotacao` (Wiki: qualquer autenticado cria e edita), oferece também
 * as unidades em que a pessoa está lotada e a dona atual; o campo é
 * opcional e o vazio tem o sentido dado por `placeholder`. Com `optional`,
 * só o campo fica opcional (e a atual sempre aparece), sem as lotadas. */
export function UnidadeDonaSelect({
  id,
  name = "unidade_id",
  permission,
  defaultValue,
  label = "Unidade dona",
  lotacao = false,
  optional = false,
  placeholder,
}: {
  id: string;
  name?: string;
  permission: string;
  defaultValue?: string | null;
  label?: string;
  lotacao?: boolean;
  optional?: boolean;
  placeholder?: string;
}) {
  const { me } = useNexus();
  const tree = useApiQuery<OrgTree[]>("v1/iam/org-tree");
  const geridas = unidadesGeridas(me, tree.data, permission);
  const oferecidas = new Set(geridas.unidades);
  if (lotacao) for (const u of unidadesLotadas(me, tree.data)) oferecidas.add(u);
  if ((lotacao || optional) && defaultValue) oferecidas.add(defaultValue);
  const opcional = geridas.global || lotacao || optional;
  const options = (tree.data ?? []).flatMap((e) =>
    e.unidades.filter((u) => oferecidas.has(u.id)).map((u) => ({ value: u.id, label: u.sigla ? `${u.sigla} — ${u.nome}` : u.nome })),
  );
  return (
    <Select
      // key: select não controlado — remonta com a dona marcada quando a
      // árvore chega depois de abrir o formulário.
      key={`${id}-${options.length}`}
      id={id}
      name={name}
      label={opcional ? label : `${label} *`}
      required={!opcional}
      placeholder={placeholder ?? (opcional ? "Institucional (sem unidade)" : "Selecione…")}
      defaultValue={defaultValue ?? ""}
      options={options}
    />
  );
}
