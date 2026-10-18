"use client";

import { Printer } from "lucide-react";

import { DataState } from "@/components/nexus/DataState";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useApiQuery } from "@/lib/api/swr";
import { EVENTOS_DE_DIA, fmtDia, type AcolhidoResumo, type EventoLinhaDoTempo } from "@/lib/sempras/types";

/** Dia do evento: a data "pura" lida em UTC; o instante, no fuso local. */
export function diaDoEvento(e: EventoLinhaDoTempo): string {
  if (EVENTOS_DE_DIA.has(e.tipo)) return fmtDia(e.data);
  const d = new Date(e.data);
  return Number.isNaN(d.getTime()) ? "—" : d.toLocaleDateString("pt-BR");
}

function Eventos({ eventos }: { eventos: EventoLinhaDoTempo[] }) {
  return (
    <ol className="relative ml-2 border-l border-border pl-5">
      {eventos.map((e, i) => (
        <li key={`${e.tipo}-${e.data}-${i}`} className="mb-4 break-inside-avoid">
          <span className="absolute -left-1.5 mt-1.5 h-3 w-3 rounded-full border border-surface bg-primary" aria-hidden="true" />
          <p className="text-xs text-muted">{diaDoEvento(e)}</p>
          <p className="font-medium">{e.titulo}</p>
          {e.detalhe && <p className="whitespace-pre-wrap text-sm text-muted">{e.detalhe}</p>}
        </li>
      ))}
    </ol>
  );
}

/**
 * Linha do tempo do acolhido: entrada, evoluções, P.I.A., visitas, termos,
 * diário, relatórios, documentos e desligamento, do mais recente ao mais
 * antigo — para a audiência concentrada e a reavaliação. A leitura é
 * auditada no servidor; o psicológico vem sem o texto para quem não tem o
 * sigilo.
 */
export function LinhaDoTempo({ acolhido, onFechar }: { acolhido: AcolhidoResumo; onFechar: () => void }) {
  const nome = acolhido.nome_social || acolhido.nome;
  const linha = useApiQuery<EventoLinhaDoTempo[]>(`v1/sempras/casa-abrigo/${acolhido.acolhimento_id}/linha-do-tempo`);
  const eventos = linha.data ?? [];
  return (
    <Dialog open onClose={onFechar} title={`Linha do tempo — ${nome}`} description={`Acolhido(a) desde ${fmtDia(acolhido.data_entrada)}`}
      size="3xl"
      footer={
        <Button type="button" size="sm" variant="secondary" onClick={() => window.print()} disabled={eventos.length === 0}>
          <Printer size={14} className="mr-1" aria-hidden="true" /> Imprimir / PDF
        </Button>
      }>
      <DataState loading={linha.isLoading} error={linha.error} onRetry={() => void linha.mutate()} empty={eventos.length === 0}
        emptyTitle="Nada registrado ainda">
        <Eventos eventos={eventos} />
        <div className="folha-impressao hidden p-8 font-sans text-black print:block">
          <h1 className="border-b-2 border-black pb-2 text-xl font-bold">Linha do tempo — {nome}</h1>
          <p className="mb-4 text-sm">Acolhido(a) desde {fmtDia(acolhido.data_entrada)}</p>
          <Eventos eventos={eventos} />
        </div>
      </DataState>
    </Dialog>
  );
}
