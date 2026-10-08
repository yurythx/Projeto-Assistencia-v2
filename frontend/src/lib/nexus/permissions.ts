import type { Me, OrgTree } from "@/lib/nexus/types";

// Mesma regra de curingas do backend (internal/platform/auth/rbac.go):
// "*" concede tudo, "recurso:*" concede toda ação do recurso. No frontend
// isto só decide o que MOSTRAR — a autorização efetiva é sempre do
// backend (middleware RequirePermission, A01).
export function hasPermission(granted: readonly string[] | undefined, want: string): boolean {
  if (!granted) return false;
  const [resource] = want.split(":");
  return granted.some((g) => g === "*" || g === want || g === `${resource}:*`);
}

/** Onde uma permissão com escopo vale (ADR 013) — espelho de
 * auth.Scope.Covers no backend, só para decidir o que OFERECER nos
 * formulários (a API confere de novo):
 *   - concessão global (ou o papel nexus-admin): todas as unidades e o
 *     "institucional" (sem dono);
 *   - entidade: todas as unidades dela;
 *   - unidade: ela e as subunidades (árvore parent_id);
 *   - departamento: nenhuma unidade inteira. */
export function unidadesGeridas(
  me: Pick<Me, "roles" | "scopes"> | undefined,
  tree: readonly OrgTree[] | undefined,
  permission: string,
): { global: boolean; unidades: Set<string> } {
  const all = (tree ?? []).flatMap((e) => e.unidades);
  if (me?.roles?.includes("nexus-admin"))
    return { global: true, unidades: new Set(all.map((u) => u.id)) };
  const filhas = new Map<string, string[]>();
  for (const u of all)
    if (u.parent_id) filhas.set(u.parent_id, [...(filhas.get(u.parent_id) ?? []), u.id]);
  const unidades = new Set<string>();
  const arvore = (id: string) => {
    if (unidades.has(id)) return;
    unidades.add(id);
    for (const f of filhas.get(id) ?? []) arvore(f);
  };
  let global = false;
  for (const s of me?.scopes ?? []) {
    if (!hasPermission(s.permissions, permission)) continue;
    if (!s.entidade_id && !s.unidade_id && !s.departamento_id) global = true;
    else if (s.departamento_id) continue;
    else if (s.unidade_id) arvore(s.unidade_id);
    else for (const u of all.filter((x) => x.entidade_id === s.entidade_id)) unidades.add(u.id);
  }
  return { global, unidades: global ? new Set(all.map((u) => u.id)) : unidades };
}

/** Departamentos em que uma permissão com escopo vale: os das unidades
 * cobertas (unidadesGeridas) e os concedidos diretamente. */
export function departamentosGeridos(
  me: Pick<Me, "roles" | "scopes"> | undefined,
  tree: readonly OrgTree[] | undefined,
  permission: string,
): { global: boolean; departamentos: Set<string> } {
  const { global, unidades } = unidadesGeridas(me, tree, permission);
  const departamentos = new Set<string>();
  for (const u of (tree ?? []).flatMap((e) => e.unidades)) {
    if (unidades.has(u.id)) for (const d of u.departamentos) departamentos.add(d.id);
  }
  for (const s of me?.scopes ?? []) {
    if (s.departamento_id && hasPermission(s.permissions, permission))
      departamentos.add(s.departamento_id);
  }
  return { global, departamentos };
}

/** Unidades em que a pessoa está lotada (concessão de qualquer perfil) —
 * espelho de auth.LotadoEm: a unidade da concessão e as subunidades; na
 * entidade, todas as unidades dela; no departamento, só a unidade dele.
 * Concessão global não é lotação. */
export function unidadesLotadas(
  me: Pick<Me, "scopes"> | undefined,
  tree: readonly OrgTree[] | undefined,
): Set<string> {
  const all = (tree ?? []).flatMap((e) => e.unidades);
  const lotadas = new Set<string>();
  for (const s of me?.scopes ?? []) {
    if (s.departamento_id) {
      const dona = all.find((u) => u.departamentos.some((d) => d.id === s.departamento_id));
      if (dona) lotadas.add(dona.id);
    } else if (s.unidade_id || s.entidade_id) {
      const sozinho = { roles: [], scopes: [{ ...s, permissions: ["*"] }] };
      for (const id of unidadesGeridas(sozinho, tree, "*").unidades) lotadas.add(id);
    }
  }
  return lotadas;
}
