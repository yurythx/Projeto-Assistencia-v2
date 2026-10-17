"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useState } from "react";
import { useSWRConfig } from "swr";
import { ArrowLeftRight, MonitorPlay, User, Users2 } from "lucide-react";

import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { AbordagemSocial } from "@/components/sempras/AbordagemSocial";
import { CasaAbrigoWorkspace } from "@/components/sempras/casa-abrigo/CasaAbrigoWorkspace";
import { EncaminhamentosRecebidos } from "@/components/sempras/EncaminhamentosRecebidos";
import { FichaModal } from "@/components/sempras/FichaModal";
import { Fila } from "@/components/sempras/Fila";
import { Recepcao } from "@/components/sempras/Recepcao";
import { Badge } from "@/components/ui/Badge";
import { Card, CardContent } from "@/components/ui/Card";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { P, TIPOS_UNIDADE, oferta, type Unidade } from "@/lib/sempras/types";

/**
 * Espaço de atendimento de UMA unidade (ADR 031 §4): recepção, fila e ficha
 * montadas pelo tipo e pelos serviços da unidade — o CRAS não vê o que é do
 * CREAS, a Casa Abrigo não tem fila, o Centro POP acolhe sem documento.
 */
export default function UnidadeAtendimentoPage() {
  const { unidade: id } = useParams<{ unidade: string }>();
  const { me, can } = useNexus();
  const { mutate } = useSWRConfig();
  const unidades = useApiQuery<Unidade[]>("v1/sempras/unidades");
  const [fichaCidadaoId, setFichaCidadaoId] = useState<string | null>(null);
  const lista = unidades.data ?? [];
  const unidade = lista.find((u) => u.id === id);
  const tipo = unidade ? TIPOS_UNIDADE[unidade.tipo] : undefined;
  const temFila = oferta(unidade, "fila");
  const casaAbrigo = oferta(unidade, "acolhimento");
  const equipeTecnica = can(P.atendimento) || can(P.psicologia);

  const papel = can(P.psicologia)
    ? "Psicóloga(o)"
    : can(P.atendimento)
    ? "Assistente Social / Técnica(o)"
    : can(P.gestao)
    ? "Gestão / Coordenação"
    : "Recepção / Pré-cadastro";

  const atualizarFila = () => void mutate(`v1/sempras/unidades/${id}/fila`);

  return (
    <div className="flex flex-col gap-6">
      <DataState
        loading={unidades.isLoading}
        error={unidades.error}
        onRetry={() => void unidades.mutate()}
        empty={!unidade}
        emptyTitle="Unidade não encontrada"
        emptyDescription="A unidade não existe, não atende público ou não está na sua lotação."
      >
        {unidade && (
          <>
            <PageHeader
              eyebrow={tipo ? `${tipo.label} · ${tipo.protecao}` : "Assistência Social"}
              title={unidade.nome}
              description={
                temFila
                  ? "Busque o munícipe, inclua na fila com a demanda e acompanhe o atendimento em tempo real."
                  : "Busque o munícipe e abra a ficha para registrar o acompanhamento."
              }
            />

            <Card className="border-surface-border bg-surface shadow-xs">
              <CardContent className="flex flex-wrap items-center justify-between gap-4 p-5">
                <div className="flex items-center gap-3">
                  <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
                    <User size={22} aria-hidden="true" />
                  </div>
                  <div className="flex flex-col">
                    <span className="text-xs font-semibold uppercase tracking-wider text-muted">Atendente</span>
                    <span className="text-base font-bold text-foreground">{me?.name || me?.username || "Atendente"}</span>
                    <Badge tone="info" className="mt-1 w-fit">{papel}</Badge>
                  </div>
                </div>
                <nav aria-label="Atalhos da unidade" className="flex flex-wrap items-center gap-3 text-sm">
                  {temFila && can(P.atendimento) && (
                    <Link href={`/sempras/painel-chamada?unidade=${unidade.id}`} className="inline-flex items-center gap-1 text-primary underline">
                      <MonitorPlay size={14} aria-hidden="true" /> Painel de chamada
                    </Link>
                  )}
                  {oferta(unidade, "coletivo") && can(P.atendimento) && (
                    <Link href={`/sempras/coletivos?unidade=${unidade.id}`} className="inline-flex items-center gap-1 text-primary underline">
                      <Users2 size={14} aria-hidden="true" /> Atendimentos coletivos
                    </Link>
                  )}
                  {lista.length > 1 && (
                    <Link href="/sempras" className="inline-flex items-center gap-1 text-primary underline">
                      <ArrowLeftRight size={14} aria-hidden="true" /> Trocar de unidade
                    </Link>
                  )}
                </nav>
              </CardContent>
            </Card>

            {/* Casa Abrigo: as telas do sistema da unidade em abas (ADR 032). */}
            {casaAbrigo && equipeTecnica && (
              <CasaAbrigoWorkspace
                unidadeId={unidade.id}
                extras={
                  <>
                    <Recepcao unidade={unidade} onIncluido={atualizarFila} onAbrirFicha={setFichaCidadaoId} />
                    {oferta(unidade, "encaminhamento") && <EncaminhamentosRecebidos unidadeId={unidade.id} />}
                  </>
                }
              />
            )}

            {/* Sem fila, a equipe técnica também busca o munícipe. */}
            {!casaAbrigo && (can(P.recepcao) || (!temFila && equipeTecnica)) && (
              <Recepcao unidade={unidade} onIncluido={atualizarFila} onAbrirFicha={setFichaCidadaoId} />
            )}

            {temFila && <Fila unidadeId={unidade.id} onAbrirFicha={setFichaCidadaoId} />}

            {equipeTecnica && oferta(unidade, "centro_pop") && <AbordagemSocial unidadeId={unidade.id} />}

            {!casaAbrigo && equipeTecnica && oferta(unidade, "encaminhamento") && (
              <EncaminhamentosRecebidos unidadeId={unidade.id} />
            )}

            <FichaModal
              cidadaoId={fichaCidadaoId}
              unidade={unidade}
              onClose={() => setFichaCidadaoId(null)}
              onCidadaoUpdated={() => {
                atualizarFila();
                void mutate(`v1/sempras/cidadaos?q=`);
              }}
            />
          </>
        )}
      </DataState>
    </div>
  );
}
