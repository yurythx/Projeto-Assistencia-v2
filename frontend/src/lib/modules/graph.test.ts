import { describe, expect, it } from "vitest";

import { layoutModuleGraph, NODE_H, NODE_W, nodeStatus, relatedModules } from "./graph";
import type { ModuleStatus } from "@/lib/nexus/types";

function mod(key: string, extra: Partial<ModuleStatus> = {}): ModuleStatus {
  return {
    key, name: key.toUpperCase(), description: "", core: false, default_enabled: true, depends_on: [], permissions: [],
    public: false, icon: "box", route: `/${key}`, enabled: true, configured: true, dependents: [], blocked_by: [], ...extra,
  };
}

// iam (núcleo) ← files ← signum ← tramite; blog solto; wiki depende de files e de blog
const MODULES = [
  mod("tramite", { depends_on: ["signum", "files"] }),
  mod("signum", { depends_on: ["files"], dependents: ["tramite"] }),
  mod("files", { depends_on: ["iam"], dependents: ["signum", "tramite", "wiki"] }),
  mod("iam", { core: true, dependents: ["files"] }),
  mod("blog", { dependents: ["wiki"], enabled: false, configured: false }),
  mod("wiki", { depends_on: ["files", "blog"], enabled: false }),
];

describe("layoutModuleGraph", () => {
  const g = layoutModuleGraph(MODULES);
  const node = (k: string) => g.nodes.find((n) => n.key === k)!;

  it("põe cada módulo uma coluna à direita da dependência mais profunda", () => {
    expect(node("iam").layer).toBe(0);
    expect(node("blog").layer).toBe(0);
    expect(node("files").layer).toBe(1);
    expect(node("signum").layer).toBe(2);
    expect(node("wiki").layer).toBe(2);
    expect(node("tramite").layer).toBe(3); // a mais profunda (signum), não files
  });

  it("ordena a primeira coluna com o núcleo antes e as demais pelo baricentro das dependências", () => {
    expect([node("iam").row, node("blog").row]).toEqual([0, 1]);
    // signum (baricentro 0) antes de wiki (média de files=0 e blog=1 → 0,5)
    expect(node("signum").row).toBe(0);
    expect(node("wiki").row).toBe(1);
  });

  it("posiciona em grade e centraliza as colunas mais curtas", () => {
    expect(node("files").x).toBeGreaterThan(node("iam").x + NODE_W);
    expect(node("files").y).toBeGreaterThan(node("iam").y); // 1 nó numa coluna de 2: no meio
    expect(g.width).toBeGreaterThanOrEqual(4 * NODE_W);
    expect(g.height).toBeGreaterThanOrEqual(2 * NODE_H);
  });

  it("liga dependência → dependente, marcando as ligações com um lado inativo", () => {
    expect(g.edges.map((e) => `${e.from}>${e.to}`).sort()).toEqual(
      ["blog>wiki", "files>signum", "files>tramite", "files>wiki", "iam>files", "signum>tramite"].sort(),
    );
    const e = g.edges.find((x) => x.from === "iam")!;
    expect(e.active).toBe(true);
    expect(e.path).toMatch(new RegExp(`^M${node("iam").x + NODE_W},${node("iam").y + NODE_H / 2} C`));
    expect(g.edges.find((x) => x.to === "wiki")!.active).toBe(false);
  });

  it("classifica o estado de cada nó", () => {
    expect(node("iam").status).toBe("core");
    expect(node("files").status).toBe("active");
    expect(node("wiki").status).toBe("waiting"); // ligado, mas blog está inativo
    expect(node("blog").status).toBe("inactive");
    expect(nodeStatus(mod("x", { enabled: false, configured: false }))).toBe("inactive");
  });

  it("ignora dependências desconhecidas, laços e ciclos sem travar", () => {
    const odd = layoutModuleGraph([
      mod("a", { depends_on: ["fantasma", "a", "b"] }),
      mod("b", { depends_on: ["a"] }),
    ]);
    expect(odd.nodes).toHaveLength(2);
    expect(odd.edges.every((e) => e.from !== "fantasma" && e.from !== e.to)).toBe(true);
  });

  it("grafo vazio não tem dimensão", () => {
    expect(layoutModuleGraph([])).toEqual({ nodes: [], edges: [], width: 0, height: 0, isolatedLabelY: null });
  });

  it("módulos sem ligação vão para uma grade abaixo das colunas, o núcleo primeiro", () => {
    const loose = ["c", "b", "a", "d", "e"].map((k) => mod(k));
    const g2 = layoutModuleGraph([mod("x", { dependents: ["y"] }), mod("y", { depends_on: ["x"] }), ...loose, mod("z", { core: true })]);
    const at = (k: string) => g2.nodes.find((n) => n.key === k)!;
    const flowBottom = at("x").y + NODE_H;
    expect(g2.isolatedLabelY).toBeGreaterThanOrEqual(flowBottom);
    expect(at("z").layer).toBe(-1);
    expect([at("z").x, at("a").x]).toEqual([8, 8 + NODE_W + 16]); // z (núcleo), depois a, b, c...
    expect(at("z").y).toBeGreaterThan(g2.isolatedLabelY!);
    expect(at("e").row).toBe(1); // 4 por linha: e quebra
    expect(g2.width).toBeGreaterThanOrEqual(4 * NODE_W);
    expect(g2.height).toBeGreaterThan(at("e").y + NODE_H);

    // só módulos soltos: a grade começa no topo, sem colunas de fluxo
    const only = layoutModuleGraph([mod("a"), mod("b")]);
    expect(only.isolatedLabelY).toBe(8);
    expect(only.nodes.map((n) => n.row)).toEqual([0, 0]);
    expect(only.width).toBe(8 * 2 + 2 * NODE_W + 16);
  });
});

describe("relatedModules", () => {
  it("devolve as dependências e os dependentes transitivos", () => {
    const r = relatedModules(MODULES, "signum");
    expect([...r.up].sort()).toEqual(["files", "iam"]);
    expect([...r.down]).toEqual(["tramite"]);
    const f = relatedModules(MODULES, "files");
    expect([...f.down].sort()).toEqual(["signum", "tramite", "wiki"]);
  });

  it("tolera chave desconhecida, referências órfãs e ciclos", () => {
    expect(relatedModules(MODULES, "nada")).toEqual({ up: new Set(), down: new Set() });
    const cyc = [mod("a", { depends_on: ["b", "fantasma"] }), mod("b", { depends_on: ["a"] })];
    expect([...relatedModules(cyc, "a").up]).toEqual(["b"]);
  });
});
