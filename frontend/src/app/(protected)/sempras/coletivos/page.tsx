"use client";

import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { useState } from "react";

import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { AtendimentosColetivos } from "@/components/sempras/AtendimentosColetivos";
import { Select } from "@/components/ui/Select";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { P, type Unidade } from "@/lib/sempras/types";

/** Atendimentos Coletivos (SCFV) — sempras:atendimento, ADR 030 Fase 8:
 * evento de grupo da unidade, alimenta o Bloco III do RMA/MDS. */
export default function ColetivosPage() {
  const { can } = useNexus();
  const unidades = useApiQuery<Unidade[]>("v1/sempras/unidades");
  const [escolhida, setEscolhida] = useState("");
  const lista = unidades.data ?? [];
  const unidadeId = escolhida || lista[0]?.id || "";

  if (!can(P.atendimento)) {
    return (
      <p className="text-sm text-muted">
        Os atendimentos coletivos são de quem atende a unidade do SEMPRAS (permissão {P.atendimento}).
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
        title="SEMPRAS — Atendimentos coletivos"
        description="Registre eventos de grupo do SCFV (Serviço de Convivência e Fortalecimento de Vínculos) da sua unidade."
      />
      <DataState loading={unidades.isLoading} error={unidades.error} onRetry={() => void unidades.mutate()} empty={lista.length === 0}
        emptyTitle="Você não está lotado em nenhuma unidade da SEMPRAS">
        <div className="max-w-md">
          <Select id="coletivos-unidade" label="Unidade" value={unidadeId} onChange={(e) => setEscolhida(e.target.value)}
            options={lista.map((u) => ({ value: u.id, label: u.nome }))} />
        </div>
        {unidadeId && (
          <div className="mt-6">
            <AtendimentosColetivos unidadeId={unidadeId} />
          </div>
        )}
      </DataState>
    </div>
  );
}
