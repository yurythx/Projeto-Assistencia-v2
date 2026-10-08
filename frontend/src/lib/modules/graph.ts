import type { ModuleStatus } from "@/lib/nexus/types";

export type NodeStatus = "core" | "active" | "waiting" | "inactive";

export interface GraphNode {
  key: string;
  name: string;
  layer: number;
  row: number;
  x: number;
  y: number;
  status: NodeStatus;
}

export interface GraphEdge {
  /** dependência (à esquerda) */
  from: string;
  /** dependente (à direita) */
  to: string;
  path: string;
  /** as duas pontas estão ativas */
  active: boolean;
}

export interface GraphLayout {
  nodes: GraphNode[];
  edges: GraphEdge[];
  width: number;
  height: number;
  /** y do rótulo da grade de módulos sem ligação alguma (null se não há) */
  isolatedLabelY: number | null;
}

export const NODE_W = 172;
export const NODE_H = 44;
const GAP_X = 64;
const GAP_Y = 16;
const PAD = 8;
const GRID_GAP = 16;
const LABEL_H = 28;
const MIN_GRID_COLS = 4;

export function nodeStatus(m: ModuleStatus): NodeStatus {
  if (m.core) return "core";
  if (m.enabled) return "active";
  if (m.configured) return "waiting"; // ligado, mas bloqueado por dependência inativa
  return "inactive";
}

const byName = (a: { name: string }, b: { name: string }) => a.name.localeCompare(b.name, "pt-BR");

/**
 * Desenho em camadas do grafo do Kernel: cada módulo fica uma coluna à
 * direita da sua dependência mais profunda (quem não depende de ninguém fica
 * na primeira), e as arestas vão da dependência para o dependente. Dentro de
 * cada coluna, a ordem segue a média das linhas das dependências
 * (baricentro), o que reduz os cruzamentos. Módulos sem ligação nenhuma
 * não entram nas colunas (seriam uma pilha que esconde as cadeias): vão para
 * uma grade compacta abaixo. Dependências desconhecidas são ignoradas; um
 * ciclo (que o Kernel recusa no registro) não trava o cálculo.
 */
export function layoutModuleGraph(all: ModuleStatus[]): GraphLayout {
  const byKey = new Map(all.map((m) => [m.key, m]));
  const deps = (m: ModuleStatus) => m.depends_on.filter((d) => byKey.has(d) && d !== m.key);
  const linked = new Set<string>();
  for (const m of all) {
    for (const d of deps(m)) {
      linked.add(d);
      linked.add(m.key);
    }
  }
  const modules = all.filter((m) => linked.has(m.key));
  const isolated = all.filter((m) => !linked.has(m.key)).sort((a, b) => Number(b.core) - Number(a.core) || byName(a, b));

  const layer = new Map<string, number>();
  const visiting = new Set<string>();
  const layerOf = (m: ModuleStatus): number => {
    const known = layer.get(m.key);
    if (known !== undefined) return known;
    if (visiting.has(m.key)) return 0;
    visiting.add(m.key);
    const l = Math.max(-1, ...deps(m).map((d) => layerOf(byKey.get(d)!))) + 1;
    visiting.delete(m.key);
    layer.set(m.key, l);
    return l;
  };
  modules.forEach(layerOf);

  const columns: ModuleStatus[][] = [];
  for (const m of modules) (columns[layer.get(m.key)!] ??= []).push(m);

  const row = new Map<string, number>();
  columns.forEach((col, i) => {
    if (i === 0) {
      col.sort((a, b) => Number(b.core) - Number(a.core) || byName(a, b));
    } else {
      const bary = (m: ModuleStatus) => {
        const rows = deps(m).map((d) => row.get(d)!);
        return rows.reduce((s, r) => s + r, 0) / rows.length;
      };
      col.sort((a, b) => bary(a) - bary(b) || byName(a, b));
    }
    col.forEach((m, r) => row.set(m.key, r));
  });

  const maxRows = Math.max(0, ...columns.map((c) => c.length));
  const flowHeight = maxRows === 0 ? 0 : PAD * 2 + maxRows * NODE_H + (maxRows - 1) * GAP_Y;
  const flowWidth = columns.length === 0 ? 0 : PAD * 2 + columns.length * NODE_W + (columns.length - 1) * GAP_X;

  const nodes: GraphNode[] = [];
  const pos = new Map<string, GraphNode>();
  columns.forEach((col, l) => {
    const offset = ((maxRows - col.length) * (NODE_H + GAP_Y)) / 2; // coluna centralizada
    col.forEach((m, r) => {
      const n: GraphNode = {
        key: m.key, name: m.name, layer: l, row: r, status: nodeStatus(m),
        x: PAD + l * (NODE_W + GAP_X), y: PAD + offset + r * (NODE_H + GAP_Y),
      };
      nodes.push(n);
      pos.set(m.key, n);
    });
  });

  const edges: GraphEdge[] = [];
  for (const m of modules) {
    for (const d of deps(m)) {
      const a = pos.get(d)!;
      const b = pos.get(m.key)!;
      const x1 = a.x + NODE_W;
      const y1 = a.y + NODE_H / 2;
      const x2 = b.x;
      const y2 = b.y + NODE_H / 2;
      const dx = Math.max(24, (x2 - x1) / 2);
      edges.push({
        from: d, to: m.key, active: byKey.get(d)!.enabled && m.enabled,
        path: `M${x1},${y1} C${x1 + dx},${y1} ${x2 - dx},${y2} ${x2},${y2}`,
      });
    }
  }
  if (isolated.length === 0) {
    return { nodes, edges, width: flowWidth, height: flowHeight, isolatedLabelY: null };
  }
  // grade: tão larga quanto as colunas do fluxo, com um mínimo de colunas
  const cols = Math.min(isolated.length, Math.max(MIN_GRID_COLS, Math.floor((flowWidth - PAD * 2 + GRID_GAP) / (NODE_W + GRID_GAP))));
  const labelY = flowHeight === 0 ? PAD : flowHeight + PAD;
  const top = labelY + LABEL_H;
  isolated.forEach((m, i) => {
    const r = Math.floor(i / cols);
    nodes.push({
      key: m.key, name: m.name, layer: -1, row: r, status: nodeStatus(m),
      x: PAD + (i % cols) * (NODE_W + GRID_GAP), y: top + r * (NODE_H + GRID_GAP),
    });
  });
  const rows = Math.ceil(isolated.length / cols);
  return {
    nodes,
    edges,
    width: Math.max(flowWidth, PAD * 2 + cols * NODE_W + (cols - 1) * GRID_GAP),
    height: top + rows * NODE_H + (rows - 1) * GRID_GAP + PAD,
    isolatedLabelY: labelY,
  };
}

/** Dependências (acima) e dependentes (abaixo) transitivos de key. */
export function relatedModules(modules: ModuleStatus[], key: string): { up: Set<string>; down: Set<string> } {
  const byKey = new Map(modules.map((m) => [m.key, m]));
  const walk = (start: string, next: (m: ModuleStatus) => string[]) => {
    const seen = new Set<string>();
    const stack = [...(byKey.get(start) ? next(byKey.get(start)!) : [])];
    while (stack.length > 0) {
      const k = stack.pop()!;
      const m = byKey.get(k);
      if (seen.has(k) || k === start || !m) continue;
      seen.add(k);
      stack.push(...next(m));
    }
    return seen;
  };
  return { up: walk(key, (m) => m.depends_on), down: walk(key, (m) => m.dependents) };
}
