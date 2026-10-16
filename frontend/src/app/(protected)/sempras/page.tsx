"use client";

import { useState } from "react";
import { useSWRConfig } from "swr";
import { Building2, User } from "lucide-react";

import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { EncaminhamentosRecebidos } from "@/components/sempras/EncaminhamentosRecebidos";
import { Fila } from "@/components/sempras/Fila";
import { FichaModal } from "@/components/sempras/FichaModal";
import { Recepcao } from "@/components/sempras/Recepcao";
import { Badge } from "@/components/ui/Badge";
import { Card, CardContent } from "@/components/ui/Card";
import { Select } from "@/components/ui/Select";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { P, type Unidade } from "@/lib/sempras/types";

/**
 * SEMPRAS (ADR 030): a jornada do munícipe na unidade — recepção (busca,
 * pré-cadastro e inclusão na fila) e fila do dia em tempo real posicionada abaixo.
 * Apenas equipe técnica e psicologia chamam para atendimento e acessam o prontuário.
 */
export default function SemprasPage() {
  const { me, can } = useNexus();
  const unidades = useApiQuery<Unidade[]>("v1/sempras/unidades");
  const [escolhida, setEscolhida] = useState("");
  const [fichaCidadaoId, setFichaCidadaoId] = useState<string | null>(null);
  const lista = unidades.data ?? [];
  const unidadeId = escolhida || lista[0]?.id || "";
  const { mutate } = useSWRConfig();

  // Identificação do perfil profissional do atendente
  const papelAtendente = can(P.psicologia)
    ? { cargo: "Psicóloga(o)", tone: "info" as const }
    : can(P.atendimento)
    ? { cargo: "Assistente Social / Técnica(o)", tone: "info" as const }
    : can(P.gestao)
    ? { cargo: "Gestão / Coordenação", tone: "neutral" as const }
    : { cargo: "Recepção / Pré-cadastro", tone: "neutral" as const };

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Assistência Social"
        title="SEMPRAS — Atendimento"
        description="Busque o munícipe pelo CPF, NIS ou nome, cadastre quem ainda não existe e acompanhe a fila da unidade em tempo real."
      />

      <DataState
        loading={unidades.isLoading}
        error={unidades.error}
        onRetry={() => void unidades.mutate()}
        empty={lista.length === 0}
        emptyTitle="Você não está lotado em nenhuma unidade da SEMPRAS"
        emptyDescription="Peça à TI a inclusão no grupo do AD da sua unidade (ex.: SEMPRAS_CRAS_CENTRAL_TECNICO)."
      >
        {/* Informações do Atendente e da Unidade no topo */}
        <Card className="border-surface-border bg-surface shadow-xs">
          <CardContent className="grid gap-4 p-5 sm:grid-cols-2 sm:items-center">
            <div className="flex items-center gap-3">
              <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                <User size={22} aria-hidden="true" />
              </div>
              <div className="flex flex-col">
                <span className="text-xs font-semibold uppercase tracking-wider text-muted">Atendente</span>
                <span className="text-base font-bold text-foreground">{me?.name || me?.username || "Atendente"}</span>
                <div className="mt-1 flex items-center gap-1.5">
                  <Badge tone={papelAtendente.tone}>{papelAtendente.cargo}</Badge>
                </div>
              </div>
            </div>

            <div className="flex w-full flex-col sm:max-w-xs sm:justify-self-end">
              <Select
                id="sempras-unidade"
                label="Unidade"
                value={unidadeId}
                onChange={(e) => setEscolhida(e.target.value)}
                options={lista.map((u) => ({ value: u.id, label: u.nome }))}
              />
            </div>
          </CardContent>
        </Card>

        {unidadeId && (
          <div className="flex flex-col gap-6">
            {/* Bloco de Recepção (Pré-cadastro e inclusão na fila) */}
            {can(P.recepcao) && (
              <Recepcao
                unidadeId={unidadeId}
                unidadeNome={lista.find((u) => u.id === unidadeId)?.nome}
                onIncluido={() => void mutate(`v1/sempras/unidades/${unidadeId}/fila`)}
                onAbrirFicha={(id) => setFichaCidadaoId(id)}
              />
            )}

            {/* Bloco da Fila — posicionado abaixo da recepção */}
            <Fila
              unidadeId={unidadeId}
              onAbrirFicha={(id) => setFichaCidadaoId(id)}
            />

            {/* Encaminhamentos recebidos — restrito à equipe técnica/psicologia */}
            {(can(P.atendimento) || can(P.psicologia)) && (
              <div className="mt-2">
                <EncaminhamentosRecebidos unidadeId={unidadeId} />
              </div>
            )}

            {/* Modal dinâmico da Ficha do Munícipe */}
            <FichaModal
              cidadaoId={fichaCidadaoId}
              unidadeId={unidadeId}
              unidadeNome={lista.find((u) => u.id === unidadeId)?.nome}
              onClose={() => setFichaCidadaoId(null)}
              onCidadaoUpdated={() => {
                void mutate(`v1/sempras/unidades/${unidadeId}/fila`);
                void mutate(`v1/sempras/cidadaos?q=`);
              }}
            />
          </div>
        )}
      </DataState>
    </div>
  );
}
