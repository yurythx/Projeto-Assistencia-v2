"use client";

import { useState, type FormEvent } from "react";

import { DataState } from "@/components/nexus/DataState";
import { fmtDate, useAction } from "@/components/nexus/useAction";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import { CATEGORIAS_COLETIVO, type AtendimentoColetivo, type CategoriaColetivo } from "@/lib/sempras/types";

function competenciaAtual(): string {
  const agora = new Date();
  return `${agora.getFullYear()}-${String(agora.getMonth() + 1).padStart(2, "0")}`;
}

function hoje(): string {
  return new Date().toISOString().slice(0, 10);
}

/** Atendimentos Coletivos (SCFV): evento de grupo da unidade, não é por
 * munícipe — alimenta o Bloco III do RMA/MDS (ADR 030, Fase 8). */
export function AtendimentosColetivos({ unidadeId }: { unidadeId: string }) {
  const [mes, setMes] = useState(competenciaAtual);
  const lista = useApiQuery<AtendimentoColetivo[]>(`v1/sempras/unidades/${unidadeId}/coletivos?mes=${mes}`);
  const [dataEvento, setDataEvento] = useState(hoje);
  const [categoria, setCategoria] = useState<CategoriaColetivo>("scfv_0_6");
  const [participantes, setParticipantes] = useState("1");
  const [descricao, setDescricao] = useState("");
  const { run, pending } = useAction();

  async function registrar(e: FormEvent) {
    e.preventDefault();
    const ok = await run(
      () => apiClient.post(`v1/sempras/unidades/${unidadeId}/coletivos`, {
        data_evento: new Date(dataEvento).toISOString(), categoria, participantes: Number(participantes) || 0,
        descricao: descricao.trim(),
      }),
      "Atendimento coletivo registrado",
    );
    if (ok) {
      setDescricao("");
      setParticipantes("1");
      void lista.mutate();
    }
  }

  const itens = lista.data ?? [];
  return (
    <Card>
      <CardHeader>
        <CardTitle as="h2">Atendimentos coletivos (SCFV)</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-4 pb-5">
        <form onSubmit={registrar} className="mt-3 grid gap-3 sm:grid-cols-2">
          <Input id="coletivo-data" label="Data do evento *" type="date" value={dataEvento}
            onChange={(e) => setDataEvento(e.target.value)} required />
          <Select id="coletivo-categoria" label="Categoria *" value={categoria}
            onChange={(e) => setCategoria(e.target.value as CategoriaColetivo)}
            options={Object.entries(CATEGORIAS_COLETIVO).map(([value, label]) => ({ value, label }))} />
          <Input id="coletivo-participantes" label="Participantes *" type="number" min={0} max={1000} value={participantes}
            onChange={(e) => setParticipantes(e.target.value)} />
          <Textarea id="coletivo-descricao" label="Descrição" rows={2} maxLength={2000} value={descricao}
            onChange={(e) => setDescricao(e.target.value)} className="sm:col-span-2" />
          <div>
            <Button type="submit" loading={pending}>
              Registrar
            </Button>
          </div>
        </form>

        <Input id="coletivo-mes" label="Mês" type="month" value={mes} onChange={(e) => setMes(e.target.value)} className="max-w-[200px]" />

        <DataState loading={lista.isLoading} error={lista.error} onRetry={() => void lista.mutate()}
          empty={itens.length === 0} emptyTitle="Nenhum atendimento coletivo neste mês">
          <ul aria-label="Atendimentos coletivos" className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
            {itens.map((a) => (
              <li key={a.id} className="flex flex-wrap items-center justify-between gap-3 p-3 text-sm">
                <div>
                  <p className="font-medium">{CATEGORIAS_COLETIVO[a.categoria]}</p>
                  <p className="text-xs text-muted">
                    {fmtDate(a.data_evento)} · {a.participantes} participante{a.participantes === 1 ? "" : "s"}
                    {a.descricao && ` · ${a.descricao}`}
                  </p>
                </div>
              </li>
            ))}
          </ul>
        </DataState>
      </CardContent>
    </Card>
  );
}
