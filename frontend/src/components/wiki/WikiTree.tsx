"use client";

import Link from "next/link";
import { FileText } from "lucide-react";

import type { WikiPage } from "@/lib/nexus/types";

interface Node extends WikiPage {
  children: Node[];
}

export function buildTree(pages: WikiPage[]): Node[] {
  const byId = new Map<string, Node>(pages.map((p) => [p.id, { ...p, children: [] }]));
  const roots: Node[] = [];
  for (const n of byId.values()) {
    const parent = n.parent_id ? byId.get(n.parent_id) : undefined;
    if (parent) parent.children.push(n);
    else roots.push(n);
  }
  return roots;
}

function Branch({ nodes, current }: { nodes: Node[]; current?: string }) {
  return (
    <ul className="flex flex-col gap-0.5">
      {nodes.map((n) => (
        <li key={n.id}>
          <Link
            href={`/wiki/${n.slug}`}
            aria-current={n.slug === current ? "page" : undefined}
            className={`flex items-center gap-2 rounded-md px-2 py-1 text-sm ${n.slug === current ? "bg-primary/10 font-medium text-primary" : "hover:bg-surface-hover"}`}
          >
            <FileText size={14} aria-hidden="true" className="shrink-0 text-muted" />
            <span className="truncate">{n.title}</span>
          </Link>
          {n.children.length > 0 && (
            <div className="ml-3 border-l border-surface-border pl-2">
              <Branch nodes={n.children} current={current} />
            </div>
          )}
        </li>
      ))}
    </ul>
  );
}

/** Navegação em árvore da base de conhecimento (GET /wiki/tree). */
export function WikiTree({ pages, current }: { pages: WikiPage[]; current?: string }) {
  const tree = buildTree(pages);
  if (tree.length === 0) return <p className="px-2 text-sm text-muted">Nenhuma página ainda.</p>;
  return (
    <nav aria-label="Páginas da wiki">
      <Branch nodes={tree} current={current} />
    </nav>
  );
}
