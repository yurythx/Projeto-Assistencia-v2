"use client";

import { Plus } from "lucide-react";
import type { FormEvent } from "react";

import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";

interface ExampleItem {
  id: string;
  title: string;
  description: string;
  status: string;
  created_at: string;
}

/**
 * Blueprint de tela de plug-in (par do módulo backend "example"):
 * leitura com useApiQuery + DataState, mutação com useAction (toast de
 * sucesso/erro), ação condicionada à permissão "example:manage" — que a
 * API revalida no middleware (A01).
 */
export default function ExemplosPage() {
  const { can } = useNexus();
  const list = useApiQuery<ExampleItem[]>("v1/examples");
  const { run, pending } = useAction();

  async function create(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget;
    const fd = new FormData(form);
    const ok = await run(
      () => apiClient.post("v1/examples", { title: String(fd.get("title") ?? "").trim(), description: String(fd.get("description") ?? "").trim() }),
      "Item criado",
    );
    if (ok) {
      form.reset();
      void list.mutate();
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader eyebrow="Módulo-modelo" title="Exemplo (blueprint)" description="Referência de como um plug-in consome a API: consulta tipada, estados padrão, mutação auditada e autorização por escopo." />

      {can("example:manage") && (
        <Card>
          <CardHeader>
            <CardTitle as="h2">Novo item</CardTitle>
          </CardHeader>
          <CardContent className="pb-5">
            <form onSubmit={create} className="mt-3 grid gap-3 sm:grid-cols-[1fr_1fr_auto] sm:items-end">
              <Input id="ex-title" name="title" label="Título *" required maxLength={200} autoComplete="off" />
              <Input id="ex-desc" name="description" label="Descrição" maxLength={2000} autoComplete="off" />
              <Button type="submit" loading={pending}>
                <Plus size={16} aria-hidden="true" className="mr-1" /> Criar
              </Button>
            </form>
          </CardContent>
        </Card>
      )}

      <DataState loading={list.isLoading} error={list.error} onRetry={() => void list.mutate()} empty={(list.data ?? []).length === 0} emptyTitle="Nenhum item cadastrado">
        <ul className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border bg-surface">
          {(list.data ?? []).map((item) => (
            <li key={item.id} className="flex items-center justify-between gap-3 p-4">
              <div>
                <p className="font-medium">{item.title}</p>
                {item.description && <p className="text-sm text-muted">{item.description}</p>}
              </div>
              <span className="flex items-center gap-2 text-xs text-muted">
                <Badge>{item.status}</Badge>
                {fmtDateTime(item.created_at)}
              </span>
            </li>
          ))}
        </ul>
      </DataState>
    </div>
  );
}
