"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { ArrowLeft } from "lucide-react";
import { Suspense, useState } from "react";

import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { PainelChamada } from "@/components/sempras/PainelChamada";
import { Select } from "@/components/ui/Select";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { P, oferta, type Unidade } from "@/lib/sempras/types";

/** Painel de chamada para TV da sala de espera (ADR 030) — exige
 * sempras:atendimento, mesmo nível de acesso da Fila. Só unidades com fila;
 * o nome do munícipe aparece apenas onde a unidade permite (ADR 031 §3).
 * Deixe aberto numa TV/monitor da recepção. */
function PainelChamadaPageConteudo() {
  const { can } = useNexus();
  const params = useSearchParams();
  const unidades = useApiQuery<Unidade[]>("v1/sempras/unidades");
  const [escolhida, setEscolhida] = useState(params.get("unidade") ?? "");
  const lista = (unidades.data ?? []).filter((u) => oferta(u, "fila"));
  const unidade = lista.find((u) => u.id === escolhida) ?? lista[0];
  const unidadeId = unidade?.id ?? "";

  if (!can(P.atendimento)) {
    return (
      <p className="text-sm text-muted">
        O painel de chamada é de quem atende a fila do SEMPRAS (permissão {P.atendimento}).
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
        title="SEMPRAS — Painel de chamada"
        description="Deixe esta tela aberta numa TV ou monitor da sala de espera: mostra as últimas senhas chamadas e quantas pessoas aguardam."
      />
      <DataState loading={unidades.isLoading} error={unidades.error} onRetry={() => void unidades.mutate()} empty={lista.length === 0}
        emptyTitle="Você não está lotado em nenhuma unidade da SEMPRAS">
        <div className="max-w-md">
          <Select id="painel-chamada-unidade" label="Unidade" value={unidadeId} onChange={(e) => setEscolhida(e.target.value)}
            options={lista.map((u) => ({ value: u.id, label: u.nome }))} />
        </div>
        {unidade && (
          <div className="mt-8">
            <PainelChamada unidade={unidade} />
          </div>
        )}
      </DataState>
    </div>
  );
}

export default function PainelChamadaPage() {
  return (
    <Suspense fallback={null}>
      <PainelChamadaPageConteudo />
    </Suspense>
  );
}
