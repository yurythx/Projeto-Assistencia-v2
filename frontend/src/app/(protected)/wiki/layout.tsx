"use client";

import { useParams, useRouter } from "next/navigation";
import { Plus } from "lucide-react";
import { useState, type ReactNode } from "react";

import { WikiEditor } from "@/components/wiki/WikiEditor";
import { WikiTree } from "@/components/wiki/WikiTree";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useApiQuery } from "@/lib/api/swr";
import type { WikiPage } from "@/lib/nexus/types";

/** Layout da Wiki: árvore de páginas à esquerda, conteúdo à direita. */
export default function WikiLayout({ children }: { children: ReactNode }) {
  const params = useParams<{ ref?: string }>();
  const router = useRouter();
  const tree = useApiQuery<WikiPage[]>("v1/wiki/tree");
  const [creating, setCreating] = useState(false);

  return (
    <div className="grid gap-6 lg:grid-cols-[16rem_1fr]">
      <aside className="flex flex-col gap-3 lg:sticky lg:top-[calc(var(--topbar-h)+1.5rem)] lg:max-h-[calc(100dvh-var(--topbar-h)-3rem)] lg:overflow-y-auto">
        <div className="flex items-center justify-between">
          <p className="dateline">Base de conhecimento</p>
          <Button size="sm" variant="ghost" onClick={() => setCreating(true)} aria-label="Nova página">
            <Plus size={16} aria-hidden="true" />
          </Button>
        </div>
        <WikiTree pages={tree.data ?? []} current={params.ref} />
      </aside>
      <div className="min-w-0">{children}</div>
      <Dialog open={creating} onClose={() => setCreating(false)} title="Nova página" size="xl">
        {creating && (
          <WikiEditor
            pages={tree.data ?? []}
            defaultParent={tree.data?.find((p) => p.slug === params.ref)?.id}
            onSaved={(p) => {
              setCreating(false);
              void tree.mutate();
              router.push(`/wiki/${p.slug}`);
            }}
          />
        )}
      </Dialog>
    </div>
  );
}
