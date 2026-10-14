"use client";

import { useState } from "react";
import { useSWRConfig } from "swr";

import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { EncaminhamentosRecebidos } from "@/components/sempras/EncaminhamentosRecebidos";
import { Fila } from "@/components/sempras/Fila";
import { Recepcao } from "@/components/sempras/Recepcao";
import { Select } from "@/components/ui/Select";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { P, type Unidade } from "@/lib/sempras/types";

/**
 * SEMPRAS (ADR 030): a jornada do munícipe na unidade — recepção (busca,
 * cadastro e inclusão na fila) e fila do dia em tempo real. O painel das
 * unidades, para a gestão, tem rota própria (/sempras/painel).
 */
export default function SemprasPage() {
  const { can } = useNexus();
  const unidades = useApiQuery<Unidade[]>("v1/sempras/unidades");
  const [escolhida, setEscolhida] = useState("");
  const lista = unidades.data ?? [];
  const unidadeId = escolhida || lista[0]?.id || "";
  const { mutate } = useSWRConfig();

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Assistência Social"
        title="SEMPRAS — Atendimento"
        description="Busque o munícipe pelo CPF, NIS ou nome, cadastre quem ainda não existe e acompanhe a fila da unidade em tempo real."
      />

      <DataState loading={unidades.isLoading} error={unidades.error} onRetry={() => void unidades.mutate()} empty={lista.length === 0}
        emptyTitle="Você não está lotado em nenhuma unidade da SEMPRAS"
        emptyDescription="Peça à TI a inclusão no grupo do AD da sua unidade (ex.: SEMPRAS_CRAS_CENTRAL_TECNICO).">
        <div className="max-w-md">
          <Select id="sempras-unidade" label="Unidade" value={unidadeId} onChange={(e) => setEscolhida(e.target.value)}
            options={lista.map((u) => ({ value: u.id, label: u.nome }))} />
        </div>
        {unidadeId && (
          <>
            <div className="mt-6 grid gap-6 xl:grid-cols-2">
              {can(P.recepcao) && <Recepcao unidadeId={unidadeId} onIncluido={() => void mutate(`v1/sempras/unidades/${unidadeId}/fila`)} />}
              <Fila unidadeId={unidadeId} />
            </div>
            {can(P.atendimento) && (
              <div className="mt-6">
                <EncaminhamentosRecebidos unidadeId={unidadeId} />
              </div>
            )}
          </>
        )}
      </DataState>
    </div>
  );
}
