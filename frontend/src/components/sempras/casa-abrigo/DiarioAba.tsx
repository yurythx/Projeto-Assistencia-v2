"use client";

import { useState, type FormEvent } from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";

import { DataState } from "@/components/nexus/DataState";
import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import { dataISO, diaDe, fmtDia, hojeISO, type AcolhidoResumo, type AnotacaoDiario } from "@/lib/sempras/types";

import { SeletorAcolhido } from "./campos";

const MESES = ["Janeiro", "Fevereiro", "Março", "Abril", "Maio", "Junho", "Julho", "Agosto", "Setembro", "Outubro", "Novembro", "Dezembro"];
const SEMANA = ["D", "S", "T", "Q", "Q", "S", "S"];

/** O mês ("AAAA-MM") deslocado de n meses. */
function somarMes(mes: string, n: number): string {
  const [a, m] = mes.split("-").map(Number);
  const d = new Date(a!, m! - 1 + n, 1);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}

/**
 * Diário de bordo: calendário do mês, anotação para o dia escolhido (geral
 * ou de um acolhido; pode ser futura, como agenda) e a lista do mês com
 * busca. Só de inclusão — as ações do sistema já ficam na auditoria.
 */
export function DiarioAba({ unidadeId, acolhidos }: { unidadeId: string; acolhidos: AcolhidoResumo[] }) {
  const [dia, setDia] = useState(hojeISO());
  const [mes, setMes] = useState(hojeISO().slice(0, 7));
  const lista = useApiQuery<AnotacaoDiario[]>(`v1/sempras/unidades/${unidadeId}/casa-abrigo/diario?mes=${mes}`);
  const [acolhimentoId, setAcolhimentoId] = useState("");
  const [texto, setTexto] = useState("");
  const [filtro, setFiltro] = useState("");
  const { run, pending } = useAction();

  const comAnotacao = new Set((lista.data ?? []).map((a) => diaDe(a.data_referencia)));
  const [ano, m] = mes.split("-").map(Number);
  const primeiro = new Date(ano!, m! - 1, 1).getDay();
  const diasNoMes = new Date(ano!, m!, 0).getDate();
  const celulas: (string | null)[] = [
    ...Array.from({ length: primeiro }, () => null),
    ...Array.from({ length: diasNoMes }, (_, i) => `${mes}-${String(i + 1).padStart(2, "0")}`),
  ];

  async function salvar(e: FormEvent) {
    e.preventDefault();
    const ok = await run(
      () => apiClient.post(`v1/sempras/unidades/${unidadeId}/casa-abrigo/diario`, {
        acolhimento_id: acolhimentoId || null, data_referencia: dataISO(dia), texto: texto.trim(),
      }),
      "Anotação salva no diário",
    );
    if (ok) {
      setTexto("");
      if (dia.slice(0, 7) !== mes) setMes(dia.slice(0, 7));
      else void lista.mutate();
    }
  }

  const termo = filtro.trim().toLowerCase();
  const visiveis = (lista.data ?? []).filter((a) =>
    `${a.texto} ${a.acolhido_nome ?? ""} ${a.autor_nome} ${fmtDia(a.data_referencia)}`.toLowerCase().includes(termo),
  );

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap gap-6">
        <div className="w-full max-w-xs rounded-xl border border-surface-border p-3">
          <div className="mb-2 flex items-center justify-between">
            <Button type="button" size="sm" variant="ghost" aria-label="Mês anterior" onClick={() => setMes(somarMes(mes, -1))}>
              <ChevronLeft size={16} aria-hidden="true" />
            </Button>
            <span className="text-sm font-semibold">
              {MESES[m! - 1]} {ano}
            </span>
            <Button type="button" size="sm" variant="ghost" aria-label="Próximo mês" onClick={() => setMes(somarMes(mes, 1))}>
              <ChevronRight size={16} aria-hidden="true" />
            </Button>
          </div>
          <div role="group" aria-label={`Calendário de ${MESES[m! - 1]} de ${ano}`} className="grid grid-cols-7 gap-0.5 text-center text-xs">
            {SEMANA.map((d, i) => (
              <span key={`${d}-${i}`} className="py-1 text-muted" aria-hidden="true">
                {d}
              </span>
            ))}
            {celulas.map((c, i) =>
              c ? (
                <button
                  key={c}
                  type="button"
                  aria-pressed={c === dia}
                  aria-label={`${fmtDia(c)}${comAnotacao.has(c) ? " (com anotação)" : ""}`}
                  onClick={() => setDia(c)}
                  className={`rounded py-1.5 ${c === dia ? "bg-primary text-primary-foreground" : comAnotacao.has(c) ? "font-bold text-primary" : "hover:bg-surface-border/50"}`}
                >
                  {Number(c.slice(8))}
                </button>
              ) : (
                <span key={`vazio-${i}`} />
              ),
            )}
          </div>
        </div>

        <form onSubmit={salvar} className="flex min-w-72 flex-1 flex-col gap-3 rounded-xl border border-surface-border p-4">
          <h2 className="text-base font-semibold">Nova anotação para {fmtDia(dia)}</h2>
          <SeletorAcolhido id="diario-acolhido" acolhidos={acolhidos} valor={acolhimentoId} onChange={setAcolhimentoId}
            label="Referente a qual acolhido?" vazio="Anotação geral (rotina da casa)" />
          <Textarea id="diario-texto" label="Anotação / ocorrência *" rows={4} value={texto} onChange={(e) => setTexto(e.target.value)}
            placeholder="O que vai acontecer ou aconteceu neste dia…" />
          <div>
            <Button type="submit" loading={pending} disabled={texto.trim().length < 3}>
              Salvar no diário
            </Button>
          </div>
        </form>
      </div>

      <section aria-label="Anotações do mês" className="flex flex-col gap-3">
        <div className="flex flex-wrap items-end justify-between gap-3">
          <h2 className="text-base font-semibold">
            Diário de {MESES[m! - 1]?.toLowerCase()} de {ano}
          </h2>
          <div className="min-w-64">
            <Input id="diario-filtro" label="Buscar por texto, acolhido ou data" value={filtro} onChange={(e) => setFiltro(e.target.value)} />
          </div>
        </div>
        <DataState loading={lista.isLoading} error={lista.error} onRetry={() => void lista.mutate()} empty={visiveis.length === 0}
          emptyTitle="Nenhuma anotação neste mês">
          <ul aria-label="Anotações do diário" className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
            {visiveis.map((a) => (
              <li key={a.id} className="flex flex-col gap-1 p-3 text-sm">
                <span className="flex flex-wrap items-center justify-between gap-2">
                  <strong>
                    {fmtDia(a.data_referencia)} · {a.acolhido_nome || "Geral (rotina da casa)"}
                  </strong>
                  <span className="text-xs text-muted">
                    {a.autor_nome} · {fmtDateTime(a.created_at)}
                  </span>
                </span>
                <span className="whitespace-pre-line">{a.texto}</span>
              </li>
            ))}
          </ul>
        </DataState>
      </section>
    </div>
  );
}
