"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { ArrowLeft } from "lucide-react";
import { Suspense, useState } from "react";

import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { AtendimentosColetivos } from "@/components/sempras/AtendimentosColetivos";
import { Select } from "@/components/ui/Select";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { P, oferta, type Unidade } from "@/lib/sempras/types";

/** Atendimentos Coletivos (SCFV) — sempras:atendimento, ADR 030 Fase 8:
 * evento de grupo da unidade, alimenta o Bloco III do RMA/MDS. Só CRAS e
 * unidades de convivência ofertam o serviço (ADR 031 §2). */
function ColetivosPageConteudo() {
  const { can } = useNexus();
  const params = useSearchParams();
  const unidades = useApiQuery<Unidade[]>("v1/sempras/unidades");
  const [escolhida, setEscolhida] = useState(params.get("unidade") ?? "");
  const lista = (unidades.data ?? []).filter((u) => oferta(u, "coletivo"));
  const unidadeId = (lista.find((u) => u.id === escolhida) ?? lista[0])?.id ?? "";

  if (!can(P.atendimento)) {
    return (
      <p className="text-sm text-muted">
        Os atendimentos coletivos são de quem atende a unidade do SEMPRAS (permissão {P.atendimento}).
      </p>
    );
  }
  return (
    <div className="flex flex-col gap-6">
      <Link href={unidadeId ? `/sempras/u/${unidadeId}` : "/sempras"} className="inline-flex items-center gap-1 text-sm text-primary underline">
        <ArrowLeft size={14} aria-hidden="true" /> Voltar ao atendimento
      </Link>
      <PageHeader
        eyebrow="Assistência Social"
        title="SEMPRAS — Atendimentos coletivos"
        description="Registre eventos de grupo do SCFV (Serviço de Convivência e Fortalecimento de Vínculos) da sua unidade."
      />
      <DataState loading={unidades.isLoading} error={unidades.error} onRetry={() => void unidades.mutate()} empty={lista.length === 0}
        emptyTitle="Nenhuma das suas unidades oferta atendimentos coletivos"
        emptyDescription="Os atendimentos coletivos são do CRAS e das unidades de convivência (SCFV).">
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

export default function ColetivosPage() {
  return (
    <Suspense fallback={null}>
      <ColetivosPageConteudo />
    </Suspense>
  );
}
