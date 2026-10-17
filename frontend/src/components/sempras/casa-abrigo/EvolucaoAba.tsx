"use client";

import { useState, type FormEvent } from "react";
import { Lock } from "lucide-react";

import { DataState } from "@/components/nexus/DataState";
import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { P, type AcolhidoResumo, type EvolucaoAcolhido } from "@/lib/sempras/types";

import { SeletorAcolhido } from "./campos";

/**
 * Evolução técnica: registra no prontuário do acolhido (o mesmo da ficha,
 * só de inclusão) e lista as evoluções de todos os acolhidos da unidade,
 * com busca por nome. O psicológico só tem texto para a psicologia.
 */
export function EvolucaoAba({ unidadeId, acolhidos }: { unidadeId: string; acolhidos: AcolhidoResumo[] }) {
  const { can } = useNexus();
  const lista = useApiQuery<EvolucaoAcolhido[]>(`v1/sempras/unidades/${unidadeId}/casa-abrigo/evolucoes`);
  const [acolhimentoId, setAcolhimentoId] = useState("");
  const [tipo, setTipo] = useState("social");
  const [texto, setTexto] = useState("");
  const [filtro, setFiltro] = useState("");
  const { run, pending } = useAction();
  const escolhido = acolhidos.find((a) => a.acolhimento_id === acolhimentoId);

  async function salvar(e: FormEvent) {
    e.preventDefault();
    if (!escolhido) return;
    const ok = await run(
      () => apiClient.post(`v1/sempras/cidadaos/${escolhido.cidadao_id}/registros`, { unidade_id: unidadeId, tipo, texto: texto.trim() }),
      "Evolução registrada",
    );
    if (ok) {
      setTexto("");
      void lista.mutate();
    }
  }

  const visiveis = (lista.data ?? []).filter((e) => e.acolhido_nome.toLowerCase().includes(filtro.trim().toLowerCase()));

  return (
    <div className="flex flex-col gap-6">
      <form onSubmit={salvar} className="flex flex-col gap-3 rounded-xl border border-surface-border p-4">
        <h2 className="text-base font-semibold">Registro de evolução técnica</h2>
        <div className="grid gap-3 sm:grid-cols-2">
          <SeletorAcolhido id="evolucao-acolhido" acolhidos={acolhidos} valor={acolhimentoId} onChange={setAcolhimentoId} />
          <Select id="evolucao-tipo" label="Tipo" value={tipo} onChange={(e) => setTipo(e.target.value)}
            options={[{ value: "social", label: "Social" }, ...(can(P.psicologia) ? [{ value: "psicologico", label: "Psicológico (sigiloso)" }] : [])]} />
        </div>
        <Textarea id="evolucao-texto" label="Relato da evolução *" rows={6} value={texto} onChange={(e) => setTexto(e.target.value)}
          placeholder="Descreva o atendimento, o comportamento ou a ocorrência…" />
        <div>
          <Button type="submit" loading={pending} disabled={!escolhido || !texto.trim()}>
            Salvar evolução
          </Button>
        </div>
      </form>

      <section aria-label="Histórico de evoluções" className="flex flex-col gap-3">
        <div className="flex flex-wrap items-end justify-between gap-3">
          <h2 className="text-base font-semibold">Histórico recente de evoluções</h2>
          <div className="min-w-64">
            <Input id="evolucao-filtro" label="Buscar por nome do acolhido" value={filtro} onChange={(e) => setFiltro(e.target.value)} />
          </div>
        </div>
        <DataState loading={lista.isLoading} error={lista.error} onRetry={() => void lista.mutate()} empty={visiveis.length === 0}
          emptyTitle="Nenhuma evolução" emptyDescription="As evoluções registradas para os acolhidos aparecem aqui.">
          <ul aria-label="Evoluções dos acolhidos" className="flex flex-col gap-3">
            {visiveis.map((e) => (
              <li key={e.id} className="rounded-lg border-l-4 border-primary bg-surface p-3 text-sm">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <strong>{e.acolhido_nome}</strong>
                  <span className="text-xs text-muted">{fmtDateTime(e.created_at)}</span>
                </div>
                {e.sigiloso ? (
                  <p className="mt-1 flex items-center gap-1 text-muted">
                    <Lock size={12} aria-hidden="true" /> Registro sigiloso da psicologia
                  </p>
                ) : (
                  <p className="mt-1 whitespace-pre-line">{e.texto}</p>
                )}
                <p className="mt-1 text-right text-xs text-primary">
                  {e.tipo === "psicologico" && <Badge tone="info" className="mr-1">Psicológico</Badge>}
                  {e.autor_nome || "Técnico responsável"}
                </p>
              </li>
            ))}
          </ul>
        </DataState>
      </section>
    </div>
  );
}
