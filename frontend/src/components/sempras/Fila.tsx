"use client";

import Link from "next/link";
import { BellRing, CheckCheck, Play, UserX, X } from "lucide-react";
import { useState, type FormEvent } from "react";

import { DataState } from "@/components/nexus/DataState";
import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import { useTopic } from "@/components/realtime/RealtimeProvider";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { P, STATUS, rotuloPrioridade, type FilaItem } from "@/lib/sempras/types";

type Acao = "chamar" | "iniciar" | "concluir" | "ausente" | "cancelar";

const ACOES: Record<Acao, { label: string; icon: typeof Play; ok: string }> = {
  chamar: { label: "Chamar", icon: BellRing, ok: "Munícipe chamado" },
  iniciar: { label: "Iniciar", icon: Play, ok: "Atendimento iniciado" },
  concluir: { label: "Concluir", icon: CheckCheck, ok: "Atendimento concluído" },
  ausente: { label: "Não compareceu", icon: UserX, ok: "Ausência registrada" },
  cancelar: { label: "Cancelar", icon: X, ok: "Entrada cancelada" },
};

/** Ações possíveis por estado (o servidor revalida — ADR 030 §4). */
function acoesDe(item: FilaItem, pode: (perm: string) => boolean): Acao[] {
  const atende = item.tipo === "psicologico" ? pode(P.psicologia) : pode(P.atendimento);
  const cancela = pode(P.recepcao) || pode(P.atendimento) || pode(P.gestao);
  switch (item.status) {
    case "aguardando":
      return [...(atende ? (["chamar"] as Acao[]) : []), ...(cancela ? (["cancelar"] as Acao[]) : [])];
    case "chamado":
      return atende ? ["iniciar", "chamar", "ausente"] : cancela ? ["cancelar"] : [];
    case "em_atendimento":
      return atende ? ["concluir"] : [];
    default:
      return [];
  }
}

/** Fila do dia da unidade, atualizada em tempo real (tópico
 * sempras:fila:<unidade>; o frame não traz dado pessoal — a lista é
 * recarregada pela API). */
export function Fila({ unidadeId }: { unidadeId: string }) {
  const { can } = useNexus();
  const { run, pending } = useAction();
  const [local, setLocal] = useState("");
  const fila = useApiQuery<FilaItem[]>(`v1/sempras/unidades/${unidadeId}/fila`);
  useTopic(`sempras:fila:${unidadeId}`, () => void fila.mutate());

  async function agir(item: FilaItem, acao: Acao) {
    const ok = await run(() => apiClient.post(`v1/sempras/atendimentos/${item.id}/${acao}`, { local }), ACOES[acao].ok);
    if (ok) void fila.mutate();
  }

  async function chamarProximo(e: FormEvent) {
    e.preventDefault();
    const out = await run(() => apiClient.post<FilaItem>(`v1/sempras/unidades/${unidadeId}/fila/proximo`, { local }));
    if (out) void fila.mutate();
  }

  const itens = fila.data ?? [];
  const ativos = itens.filter((i) => ["aguardando", "chamado", "em_atendimento"].includes(i.status));
  return (
    <Card>
      <CardHeader>
        <CardTitle as="h2">Fila de hoje</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-4 pb-5">
        {can(P.atendimento) && (
          <form onSubmit={chamarProximo} className="mt-3 grid gap-3 sm:grid-cols-[1fr_auto] sm:items-end">
            <Input id="sempras-local" label="Sala ou guichê" value={local} onChange={(e) => setLocal(e.target.value)} maxLength={60} autoComplete="off" />
            <Button type="submit" loading={pending}>
              <BellRing size={16} aria-hidden="true" className="mr-1" /> Chamar próximo
            </Button>
          </form>
        )}
        <p className="text-sm text-muted" aria-live="polite">
          {ativos.filter((i) => i.status === "aguardando").length} aguardando · {itens.length} entrada(s) hoje
        </p>
        <DataState loading={fila.isLoading} error={fila.error} onRetry={() => void fila.mutate()} empty={itens.length === 0} emptyTitle="Ninguém na fila hoje">
          <ul aria-label="Fila" className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
            {itens.map((item) => (
              <li key={item.id} className="flex flex-wrap items-center justify-between gap-3 p-3">
                <div className="flex items-center gap-3">
                  <span className="font-mono text-lg font-semibold">{item.senha_exibida}</span>
                  <div>
                    <p className="font-medium">
                      {can(P.atendimento) ? <Link href={`/sempras/cidadaos/${item.cidadao_id}`} className="underline">{item.nome}</Link> : item.nome}
                    </p>
                    <p className="text-xs text-muted">
                      {[item.tipo === "psicologico" ? "Psicológico" : "Social", item.prioridade && rotuloPrioridade(item.prioridade),
                        item.local, fmtDateTime(item.created_at)].filter(Boolean).join(" · ")}
                    </p>
                  </div>
                </div>
                <span className="flex flex-wrap items-center gap-2">
                  <Badge tone={STATUS[item.status].tone}>{STATUS[item.status].label}</Badge>
                  {acoesDe(item, can).map((acao) => {
                    const Icon = ACOES[acao].icon;
                    return (
                      <Button key={acao} size="sm" variant={acao === "cancelar" || acao === "ausente" ? "secondary" : "primary"}
                        onClick={() => void agir(item, acao)} disabled={pending} aria-label={`${ACOES[acao].label} ${item.senha_exibida}`}>
                        <Icon size={14} aria-hidden="true" className="mr-1" /> {ACOES[acao].label}
                      </Button>
                    );
                  })}
                </span>
              </li>
            ))}
          </ul>
        </DataState>
      </CardContent>
    </Card>
  );
}
