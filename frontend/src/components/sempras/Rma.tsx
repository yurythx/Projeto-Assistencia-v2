"use client";

import { Download, Printer } from "lucide-react";
import { useState } from "react";

import { DataState } from "@/components/nexus/DataState";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { useApiQuery } from "@/lib/api/swr";
import { VIOLACOES_PAEFI, rotuloDemanda, type RmaUnidade, type TipoUnidade, type ViolacaoPaefi } from "@/lib/sempras/types";

function Numero({ valor, rotulo }: { valor: string | number; rotulo: string }) {
  return (
    <li className="rounded-lg border border-surface-border bg-surface p-4">
      <span className="block text-2xl font-bold text-foreground">{valor}</span>
      <span className="block text-sm text-muted">{rotulo}</span>
    </li>
  );
}

function competenciaAtual(): string {
  const agora = new Date();
  return `${agora.getFullYear()}-${String(agora.getMonth() + 1).padStart(2, "0")}`;
}

interface Coluna {
  rotulo: string;
  valor: (l: RmaUnidade) => number;
}

const col = (rotulo: string, valor: (l: RmaUnidade) => number): Coluna => ({ rotulo, valor });

/** Formulários do RMA (ADR 031, fase E): cada um lista só as unidades do
 * tipo e os indicadores que o tipo produz. "Geral" é o consolidado da gestão. */
const FORMULARIOS: Record<string, { label: string; tipos: TipoUnidade[] | null; colunas: Coluna[]; destaques: Coluna[] }> = {
  geral: {
    label: "Consolidado (todas as unidades)",
    tipos: null,
    colunas: [
      col("Atendimentos", (l) => l.atendimentos),
      col("PAIF ativos", (l) => l.paif_ativos),
      col("Benefícios concedidos", (l) => l.beneficios_concedidos_no_mes),
      col("PAEFI ativos", (l) => l.paefi_casos_ativos),
      col("MSE ativas", (l) => l.mse_ativas),
      col("Conselho Tutelar", (l) => l.conselho_atendimentos_no_mes),
      col("Casa Abrigo — acolhidos", (l) => l.casa_abrigo_acolhidos_ativos),
      col("Casa da Mulher — casos ativos", (l) => l.casa_mulher_casos_ativos),
      col("Encaminhamentos enviados", (l) => l.encaminhamentos_enviados_no_mes),
    ],
    destaques: [
      col("Atendimentos", (l) => l.atendimentos),
      col("PAIF ativos", (l) => l.paif_ativos),
      col("PAEFI ativos", (l) => l.paefi_casos_ativos),
      col("Casa Abrigo — acolhidos", (l) => l.casa_abrigo_acolhidos_ativos),
    ],
  },
  cras: {
    label: "CRAS — Proteção Social Básica",
    tipos: ["cras", "scfv"],
    colunas: [
      col("Atendimentos", (l) => l.atendimentos),
      col("PAIF ativos", (l) => l.paif_ativos),
      col("PAIF — extrema pobreza", (l) => l.paif_extrema_pobreza),
      col("PAIF — Bolsa Família", (l) => l.paif_bolsa_familia),
      col("PAIF — BPC", (l) => l.paif_bpc),
      col("SCFV — participantes (PAIF)", (l) => l.paif_scfv_participantes),
      col("Benefícios concedidos", (l) => l.beneficios_concedidos_no_mes),
      col("Coletivo — famílias PAIF", (l) => l.coletivo_paif_familias_no_mes),
      col("SCFV 0–6", (l) => l.coletivo_scfv_0_6_no_mes),
      col("SCFV 7–14", (l) => l.coletivo_scfv_7_14_no_mes),
      col("SCFV 15–17", (l) => l.coletivo_scfv_15_17_no_mes),
      col("SCFV 18–59", (l) => l.coletivo_scfv_18_59_no_mes),
      col("SCFV idosos", (l) => l.coletivo_scfv_idosos_no_mes),
      col("Palestras", (l) => l.coletivo_palestras_no_mes),
      col("PcD", (l) => l.coletivo_pcd_no_mes),
      col("Ofícios recebidos", (l) => l.oficios_cras_recebidos_no_mes),
      col("Ofícios respondidos", (l) => l.oficios_cras_respondidos_no_mes),
      col("Encaminhamentos enviados", (l) => l.encaminhamentos_enviados_no_mes),
    ],
    destaques: [
      col("Atendimentos", (l) => l.atendimentos),
      col("PAIF ativos", (l) => l.paif_ativos),
      col("Benefícios concedidos", (l) => l.beneficios_concedidos_no_mes),
      col("Participantes em coletivos", (l) => l.coletivo_total_no_mes),
    ],
  },
  creas: {
    label: "CREAS — PAEFI e MSE em meio aberto",
    tipos: ["creas"],
    colunas: [
      col("PAEFI no início do mês", (l) => l.paefi_casos_inicio_mes),
      col("PAEFI novos no mês", (l) => l.paefi_casos_abertos_no_mes),
      col("PAEFI desligados no mês", (l) => l.paefi_desligados_no_mes),
      ...(Object.keys(VIOLACOES_PAEFI) as ViolacaoPaefi[]).map((v) =>
        col(`Novos — ${VIOLACOES_PAEFI[v]}`, (l) => l.paefi_novos_por_violacao?.[v] ?? 0),
      ),
      col("LA em cumprimento", (l) => l.mse_la_ativas),
      col("PSC em cumprimento", (l) => l.mse_psc_ativas),
      col("LA novas no mês", (l) => l.mse_novas_la_no_mes),
      col("PSC novas no mês", (l) => l.mse_novas_psc_no_mes),
      col("MSE concluídas no mês", (l) => l.mse_concluidas_no_mes),
      col("Notificações recebidas", (l) => l.creas_notificacoes_recebidas_no_mes),
      col("Notificações respondidas", (l) => l.creas_notificacoes_respondidas_no_mes),
      col("Atendimentos individualizados", (l) => l.atendimentos),
    ],
    destaques: [
      col("PAEFI no início do mês", (l) => l.paefi_casos_inicio_mes),
      col("PAEFI novos no mês", (l) => l.paefi_casos_abertos_no_mes),
      col("Adolescentes em MSE", (l) => l.mse_la_ativas + l.mse_psc_ativas),
      col("Notificações recebidas", (l) => l.creas_notificacoes_recebidas_no_mes),
    ],
  },
  centro_pop: {
    label: "Centro POP — população em situação de rua",
    tipos: ["centro_pop"],
    colunas: [
      col("Pessoas atendidas no mês", (l) => l.pop_pessoas_atendidas_no_mes),
      col("Pessoas atendidas pela 1ª vez", (l) => l.pop_pessoas_novas_no_mes),
      col("Usuárias de crack/outras drogas", (l) => l.pop_pessoas_uso_drogas),
      col("Com doença ou transtorno", (l) => l.pop_pessoas_com_doenca),
      col("Trecheiras / migrantes", (l) => l.pop_pessoas_trecheiras),
      col("Atendimentos realizados", (l) => l.atendimentos),
      col("Encaminhamentos para a rede", (l) => l.encaminhamentos_externos_no_mes),
      col("Benefícios concedidos", (l) => l.beneficios_concedidos_no_mes),
    ],
    destaques: [
      col("Pessoas atendidas", (l) => l.pop_pessoas_atendidas_no_mes),
      col("Pela 1ª vez", (l) => l.pop_pessoas_novas_no_mes),
      col("Atendimentos realizados", (l) => l.atendimentos),
      col("Encaminhamentos para a rede", (l) => l.encaminhamentos_externos_no_mes),
    ],
  },
  casa_mulher: {
    label: "Casa da Mulher (indicadores municipais)",
    tipos: ["casa_mulher"],
    colunas: [
      col("Casos ativos", (l) => l.casa_mulher_casos_ativos),
      col("Casos novos no mês", (l) => l.casa_mulher_casos_novos_no_mes),
      col("Risco alto ou extremo", (l) => l.casa_mulher_risco_alto_extremo),
      col("Abrigadas", (l) => l.casa_mulher_abrigadas),
      col("Medidas protetivas deferidas", (l) => l.casa_mulher_medidas_deferidas),
      col("Atendimentos", (l) => l.atendimentos),
    ],
    destaques: [
      col("Casos ativos", (l) => l.casa_mulher_casos_ativos),
      col("Risco alto ou extremo", (l) => l.casa_mulher_risco_alto_extremo),
      col("Abrigadas", (l) => l.casa_mulher_abrigadas),
      col("Medidas deferidas", (l) => l.casa_mulher_medidas_deferidas),
    ],
  },
};

/** RMA/MDS (sempras:gestao): consolida, por unidade e por competência, os
 * indicadores que o SUAS exige reportar — reais (booleans, enums, datas),
 * não inferidos de texto livre como no legado de referência. Um formulário
 * por tipo de unidade (CRAS, CREAS, Centro POP), mais a Casa da Mulher. */
export function Rma() {
  const [competencia, setCompetencia] = useState(competenciaAtual);
  const [formulario, setFormulario] = useState("geral");
  const rma = useApiQuery<RmaUnidade[]>(`v1/sempras/relatorios/rma?competencia=${competencia}`);
  const form = FORMULARIOS[formulario]!;
  const linhas = (rma.data ?? []).filter((l) => !form.tipos || form.tipos.includes(l.unidade.tipo));
  const total = (c: Coluna) => linhas.reduce((soma, l) => soma + c.valor(l), 0);

  // Demandas da recepção (ADR 031 §3), somadas nas unidades do formulário.
  const demandas = new Map<string, number>();
  for (const l of linhas) {
    for (const [d, n] of Object.entries(l.atendimentos_por_demanda ?? {})) demandas.set(d, (demandas.get(d) ?? 0) + n);
  }
  const demandasOrdenadas = [...demandas.entries()].sort((a, b) => b[1] - a[1]);

  function exportarCsv() {
    if (linhas.length === 0) return;
    const escapar = (s: string) => `"${s.replace(/"/g, '""')}"`;
    const cabecalho = ["Unidade", ...form.colunas.map((c) => c.rotulo)].map(escapar);
    const rows = linhas.map((l) => [escapar(l.unidade.nome), ...form.colunas.map((c) => c.valor(l))]);
    const csvContent = "﻿" + [cabecalho.join(";"), ...rows.map((r) => r.join(";"))].join("\r\n");
    const blob = new Blob([csvContent], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.setAttribute("href", url);
    link.setAttribute("download", `RMA_SEMPRAS_${formulario}_${competencia}.csv`);
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div className="flex flex-wrap items-end gap-3">
          <Input
            id="rma-competencia"
            label="Competência"
            type="month"
            value={competencia}
            onChange={(e) => setCompetencia(e.target.value)}
            className="max-w-[200px]"
          />
          <Select
            id="rma-formulario"
            label="Formulário"
            value={formulario}
            onChange={(e) => setFormulario(e.target.value)}
            options={Object.entries(FORMULARIOS).map(([value, f]) => ({ value, label: f.label }))}
          />
        </div>
        <div className="flex items-center gap-2">
          <Button size="sm" variant="secondary" onClick={exportarCsv} disabled={linhas.length === 0}>
            <Download size={15} className="mr-1.5" /> Exportar CSV (Excel)
          </Button>
          <Button size="sm" variant="secondary" onClick={() => window.print()} disabled={linhas.length === 0}>
            <Printer size={15} className="mr-1.5" /> Imprimir Relatório
          </Button>
        </div>
      </div>

      <DataState loading={rma.isLoading} error={rma.error} onRetry={() => void rma.mutate()} empty={linhas.length === 0}
        emptyTitle="Nenhuma unidade deste tipo">
        <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4" aria-label="Totais da competência">
          {form.destaques.map((c) => (
            <Numero key={c.rotulo} valor={total(c)} rotulo={c.rotulo} />
          ))}
        </ul>

        <Card className="mt-6">
          <CardHeader>
            <CardTitle as="h2">Por unidade</CardTitle>
          </CardHeader>
          <CardContent className="pb-5">
            <div className="mt-3 overflow-x-auto">
              <table className="w-full text-sm">
                <caption className="sr-only">{form.label} por unidade na competência selecionada</caption>
                <thead>
                  <tr className="text-left text-muted">
                    <th scope="col" className="py-2">Unidade</th>
                    {form.colunas.map((c) => (
                      <th key={c.rotulo} scope="col">{c.rotulo}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {linhas.map((l) => (
                    <tr key={l.unidade.id} className="border-t border-surface-border">
                      <th scope="row" className="py-2 text-left font-medium">{l.unidade.nome}</th>
                      {form.colunas.map((c) => (
                        <td key={c.rotulo}>{c.valor(l)}</td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>

        {demandasOrdenadas.length > 0 && (
          <Card className="mt-6">
            <CardHeader>
              <CardTitle as="h2">Demandas da recepção</CardTitle>
            </CardHeader>
            <CardContent className="pb-5">
              <ul aria-label="Demandas da recepção" className="flex flex-col divide-y divide-surface-border text-sm">
                {demandasOrdenadas.map(([d, n]) => (
                  <li key={d} className="flex justify-between py-2">
                    <span>{rotuloDemanda(d)}</span>
                    <span className="font-semibold">{n}</span>
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>
        )}
      </DataState>
    </div>
  );
}
