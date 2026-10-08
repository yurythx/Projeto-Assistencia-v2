"use client";

import { ArrowLeft, Printer } from "lucide-react";
import Link from "next/link";
import { useParams } from "next/navigation";

import { FichaValidacao } from "@/components/atlas/FichaValidacao";
import { DataState } from "@/components/nexus/DataState";
import { Button } from "@/components/ui/Button";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { Workflow } from "@/lib/nexus/types";

/** Ficha de validação para imprimir e levar à entrevista no departamento
 * (ADR 028): o fluxo proposto, item a item, com espaço para confirmar ou
 * corrigir e para padronizar cada documento. */
export default function FichaValidacaoPage() {
  const { id } = useParams<{ id: string }>();
  const { can } = useNexus();
  const gestao = can("atlas:manage");
  const detail = useApiQuery<Workflow>(
    `v1/atlas/${gestao ? "admin/" : ""}workflows/${encodeURIComponent(id)}`,
  );
  const wf = detail.data;

  return (
    <div className="mx-auto flex max-w-4xl flex-col gap-5 text-foreground">
      <div className="flex items-center justify-between gap-2 print:hidden">
        <Link
          href={`/atlas/procedimentos/${encodeURIComponent(id)}`}
          className="inline-flex items-center gap-1 text-sm text-muted hover:text-foreground"
        >
          <ArrowLeft size={14} aria-hidden="true" /> Voltar ao procedimento
        </Link>
        <Button size="sm" onClick={() => window.print()}>
          <Printer size={14} aria-hidden="true" className="mr-1" /> Imprimir
        </Button>
      </div>
      <DataState
        loading={detail.isLoading}
        error={detail.error}
        onRetry={() => void detail.mutate()}
        empty={!wf}
      >
        {wf && <FichaValidacao wf={wf} />}
      </DataState>
    </div>
  );
}
