"use client";

import { Lock } from "lucide-react";
import { useId, useMemo, useState, type KeyboardEvent } from "react";

import { Badge } from "@/components/ui/Badge";
import { Toggle } from "@/components/ui/Toggle";
import { layoutModuleGraph, NODE_H, NODE_W, relatedModules, type NodeStatus } from "@/lib/modules/graph";
import { moduleNames } from "@/lib/modules/plan";
import type { ModuleStatus } from "@/lib/nexus/types";

const STATUS: Record<NodeStatus, { label: string; tone: "info" | "success" | "warning" | "neutral"; box: string; dot: string }> = {
  core: { label: "Núcleo", tone: "info", box: "fill-primary/10 stroke-primary", dot: "fill-primary" },
  active: { label: "Ativo", tone: "success", box: "fill-success/10 stroke-success", dot: "fill-success" },
  waiting: { label: "Aguardando dependência", tone: "warning", box: "fill-warning/10 stroke-warning", dot: "fill-warning" },
  inactive: { label: "Inativo", tone: "neutral", box: "fill-surface stroke-surface-border", dot: "fill-muted" },
};

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;

export interface ModuleGraphProps {
  modules: ModuleStatus[];
  canManage: boolean;
  busy: boolean;
  onToggle: (m: ModuleStatus, enabled: boolean) => void;
}

/**
 * Grafo de dependências do Kernel (ADR 008): colunas da esquerda para a
 * direita, cada aresta vai da dependência ao dependente. Selecionar um
 * módulo (clique, Enter ou espaço) destaca tudo de que ele depende e tudo
 * que depende dele, e abre o painel com o interruptor — a mesma operação,
 * com a mesma confirmação de cascata, da visão em cartões.
 */
export function ModuleGraph({ modules, canManage, busy, onToggle }: ModuleGraphProps) {
  const arrowId = useId();
  const layout = useMemo(() => layoutModuleGraph(modules), [modules]);
  const [selected, setSelected] = useState<string | null>(null);
  const current = modules.find((m) => m.key === selected) ?? null;
  const related = useMemo(() => (selected ? relatedModules(modules, selected) : null), [modules, selected]);

  const lit = (key: string) => !related || key === selected || related.up.has(key) || related.down.has(key);
  const select = (key: string) => setSelected((s) => (s === key ? null : key));
  const onKey = (e: KeyboardEvent, key: string) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      select(key);
    } else if (e.key === "Escape") {
      setSelected(null);
    }
  };

  return (
    <div className="flex flex-col gap-4">
      <figure className="overflow-x-auto rounded-xl border border-surface-border bg-surface p-3">
        <svg
          role="group"
          aria-label={`Grafo de dependências: ${plural(modules.length, "módulo", "módulos")}, ${plural(layout.edges.length, "dependência", "dependências")}. Selecione um módulo para destacar as ligações.`}
          viewBox={`0 0 ${layout.width} ${layout.height}`}
          width={layout.width}
          height={layout.height}
          className="mx-auto block"
        >
          <defs>
            <marker id={arrowId} viewBox="0 0 8 8" refX="8" refY="4" markerWidth="8" markerHeight="8" orient="auto-start-reverse">
              <path d="M0,0 L8,4 L0,8 z" className="fill-muted" />
            </marker>
          </defs>
          {layout.edges.map((e) => {
            const on = lit(e.from) && lit(e.to);
            return (
              <path
                key={`${e.from}-${e.to}`}
                d={e.path}
                data-edge={`${e.from}->${e.to}`}
                markerEnd={`url(#${arrowId})`}
                className={`fill-none transition-opacity ${e.active ? "stroke-muted" : "stroke-surface-border [stroke-dasharray:4_3]"} ${on ? "opacity-100" : "opacity-15"} ${related && on ? "stroke-2" : "stroke-[1.5]"}`}
              />
            );
          })}
          {layout.isolatedLabelY !== null && (
            <text x={8} y={layout.isolatedLabelY + 14} className="fill-muted text-xs font-medium uppercase tracking-wide">
              Sem dependências
            </text>
          )}
          {layout.nodes.map((n) => {
            const s = STATUS[n.status];
            const m = modules.find((x) => x.key === n.key)!;
            const on = lit(n.key);
            const label = [
              m.name,
              s.label,
              m.depends_on.length > 0 ? `depende de ${moduleNames(modules, m.depends_on)}` : "",
              m.dependents.length > 0 ? `necessário para ${moduleNames(modules, m.dependents)}` : "",
            ].filter(Boolean).join("; ");
            return (
              <g
                key={n.key}
                role="button"
                tabIndex={0}
                aria-pressed={selected === n.key}
                aria-label={label}
                data-node={n.key}
                transform={`translate(${n.x},${n.y})`}
                onClick={() => select(n.key)}
                onKeyDown={(e) => onKey(e, n.key)}
                className={`cursor-pointer outline-none transition-opacity focus-visible:[&>rect]:stroke-[3] ${on ? "opacity-100" : "opacity-30"}`}
              >
                <rect width={NODE_W} height={NODE_H} rx={8} className={`${s.box} ${selected === n.key ? "stroke-[3]" : "stroke-[1.5]"}`} />
                <circle cx={14} cy={NODE_H / 2} r={4} className={s.dot} />
                <text x={26} y={NODE_H / 2} dominantBaseline="central" className="fill-foreground text-[13px] font-medium">
                  {n.name.length > 20 ? `${n.name.slice(0, 19)}…` : n.name}
                </text>
              </g>
            );
          })}
        </svg>
        <figcaption className="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted">
          {(Object.keys(STATUS) as NodeStatus[]).map((k) => (
            <span key={k} className="inline-flex items-center gap-1.5">
              <svg width="10" height="10" aria-hidden="true">
                <circle cx="5" cy="5" r="4" className={STATUS[k].dot} />
              </svg>
              {STATUS[k].label}
            </span>
          ))}
          <span>Seta: da dependência para o módulo que depende dela. Tracejado: ligação com um lado inativo.</span>
        </figcaption>
      </figure>

      {current && (
        <section aria-label={`Detalhes de ${current.name}`} className="flex flex-col gap-2 rounded-xl border border-surface-border bg-surface p-4 text-sm">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-base font-semibold">{current.name}</h2>
            <code className="text-xs text-muted">{current.key}</code>
            <Badge tone={STATUS[layout.nodes.find((n) => n.key === current.key)!.status].tone}>
              {STATUS[layout.nodes.find((n) => n.key === current.key)!.status].label}
            </Badge>
            <span className="ml-auto">
              {current.core ? (
                <span className="inline-flex items-center gap-1 text-xs text-muted">
                  <Lock size={14} aria-hidden="true" /> Sempre ativo
                </span>
              ) : (
                <Toggle
                  label={`${current.configured ? "Desativar" : "Ativar"} ${current.name}`}
                  checked={current.configured}
                  disabled={!canManage || busy}
                  onChange={(v) => onToggle(current, v)}
                />
              )}
            </span>
          </div>
          <p className="text-muted">{current.description}</p>
          <p>
            <span className="font-medium">Depende de:</span>{" "}
            {related && related.up.size > 0 ? moduleNames(modules, [...related.up]) : "nenhum módulo"}
          </p>
          <p>
            <span className="font-medium">Necessário para:</span>{" "}
            {related && related.down.size > 0 ? moduleNames(modules, [...related.down]) : "nenhum módulo"}
          </p>
        </section>
      )}
    </div>
  );
}
