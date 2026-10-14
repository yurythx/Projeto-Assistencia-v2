"use client";

import Link from "next/link";
import { ArrowLeft } from "lucide-react";

import { PageHeader } from "@/components/nexus/PageHeader";
import { Rma } from "@/components/sempras/Rma";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { P } from "@/lib/sempras/types";

/** RMA/MDS do SEMPRAS (sempras:gestao — ADR 030): consolida, por
 * competência, os indicadores que o SUAS exige reportar. Rota própria,
 * mesmo padrão de /sempras/painel. */
export default function RmaPage() {
  const { can } = useNexus();
  if (!can(P.gestao)) {
    return (
      <p className="text-sm text-muted">
        O RMA é da gestão do SEMPRAS (permissão {P.gestao}).
      </p>
    );
  }
  return (
    <div className="flex flex-col gap-6">
      <Link href="/sempras" className="inline-flex items-center gap-1 text-sm text-primary underline">
        <ArrowLeft size={14} aria-hidden="true" /> Voltar ao atendimento
      </Link>
      <PageHeader
        eyebrow="Assistência Social"
        title="SEMPRAS — RMA/MDS"
        description="Relatório Mensal de Atendimento: indicadores do SUAS por unidade, numa competência."
      />
      <Rma />
    </div>
  );
}
