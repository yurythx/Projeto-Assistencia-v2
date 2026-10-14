"use client";

import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { useState } from "react";

import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { PainelChamada } from "@/components/sempras/PainelChamada";
import { Select } from "@/components/ui/Select";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { P, type Unidade } from "@/lib/sempras/types";

/** Painel de chamada para TV da sala de espera (ADR 030) — exige
 * sempras:atendimento, mesmo nível de acesso da Fila (mostra nome do
 * munícipe). Deixe aberto numa TV/monitor da recepção. */
export default function PainelChamadaPage() {
  const { can } = useNexus();
  const unidades = useApiQuery<Unidade[]>("v1/sempras/unidades");
  const [escolhida, setEscolhida] = useState("");
  const lista = unidades.data ?? [];
  const unidadeId = escolhida || lista[0]?.id || "";

  if (!can(P.atendimento)) {
    return (
      <p className="text-sm text-muted">
        O painel de chamada é de quem atende a fila do SEMPRAS (permissão {P.atendimento}).
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
        title="SEMPRAS — Painel de chamada"
        description="Deixe esta tela aberta numa TV ou monitor da sala de espera: mostra as últimas senhas chamadas e quantas pessoas aguardam."
      />
      <DataState loading={unidades.isLoading} error={unidades.error} onRetry={() => void unidades.mutate()} empty={lista.length === 0}
        emptyTitle="Você não está lotado em nenhuma unidade da SEMPRAS">
        <div className="max-w-md">
          <Select id="painel-chamada-unidade" label="Unidade" value={unidadeId} onChange={(e) => setEscolhida(e.target.value)}
            options={lista.map((u) => ({ value: u.id, label: u.nome }))} />
        </div>
        {unidadeId && (
          <div className="mt-8">
            <PainelChamada unidadeId={unidadeId} />
          </div>
        )}
      </DataState>
    </div>
  );
}
