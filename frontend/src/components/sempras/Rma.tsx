"use client";

import { Download, Printer } from "lucide-react";
import { useState } from "react";

import { DataState } from "@/components/nexus/DataState";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { useApiQuery } from "@/lib/api/swr";
import type { RmaUnidade } from "@/lib/sempras/types";

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

/** RMA/MDS (sempras:gestao): consolida, por unidade e por competência, os
 * indicadores que o SUAS exige reportar — reais (booleans, enums, datas),
 * não inferidos de texto livre como no legado de referência. */
export function Rma() {
  const [competencia, setCompetencia] = useState(competenciaAtual);
  const rma = useApiQuery<RmaUnidade[]>(`v1/sempras/relatorios/rma?competencia=${competencia}`);
  const linhas = rma.data ?? [];
  const total = (campo: keyof Omit<RmaUnidade, "unidade">) => linhas.reduce((soma, l) => soma + l[campo], 0);

  function exportarCsv() {
    if (linhas.length === 0) return;
    const colunas = [
      "Unidade",
      "Atendimentos",
      "PAIF Ativos",
      "Beneficios Concedidos",
      "PAEFI Ativos",
      "PAEFI Abertos no Mes",
      "MSE Ativas",
      "MSE Concluidas no Mes",
      "Conselho Tutelar",
      "Notificacoes CREAS",
      "Casa Abrigo Ativos",
      "Casa Abrigo Entradas no Mes",
      "Encaminhamentos Enviados",
      "Oficios CRAS Recebidos",
      "Oficios CRAS Respondidos",
      "Coletivo Participantes",
    ];

    const rows = linhas.map((l) => [
      `"${l.unidade.nome.replace(/"/g, '""')}"`,
      l.atendimentos,
      l.paif_ativos,
      l.beneficios_concedidos_no_mes,
      l.paefi_casos_ativos,
      l.paefi_casos_abertos_no_mes,
      l.mse_ativas,
      l.mse_concluidas_no_mes,
      l.conselho_atendimentos_no_mes,
      l.creas_notificacoes_recebidas_no_mes,
      l.casa_abrigo_acolhidos_ativos,
      l.casa_abrigo_entradas_no_mes,
      l.encaminhamentos_enviados_no_mes,
      l.oficios_cras_recebidos_no_mes,
      l.oficios_cras_respondidos_no_mes,
      l.coletivo_total_no_mes,
    ]);

    const csvContent = "\uFEFF" + [colunas.join(";"), ...rows.map((r) => r.join(";"))].join("\r\n");
    const blob = new Blob([csvContent], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.setAttribute("href", url);
    link.setAttribute("download", `RMA_MDS_SEMPRAS_${competencia}.csv`);
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <Input
          id="rma-competencia"
          label="Competência"
          type="month"
          value={competencia}
          onChange={(e) => setCompetencia(e.target.value)}
          className="max-w-[200px]"
        />
        <div className="flex items-center gap-2">
          <Button size="sm" variant="secondary" onClick={exportarCsv} disabled={linhas.length === 0}>
            <Download size={15} className="mr-1.5" /> Exportar CSV (Excel)
          </Button>
          <Button size="sm" variant="secondary" onClick={() => window.print()} disabled={linhas.length === 0}>
            <Printer size={15} className="mr-1.5" /> Imprimir Relatório
          </Button>
        </div>
      </div>

      <DataState loading={rma.isLoading} error={rma.error} onRetry={() => void rma.mutate()} empty={linhas.length === 0} emptyTitle="Nenhuma unidade">
        <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4" aria-label="Totais da competência">
          <Numero valor={total("atendimentos")} rotulo="Atendimentos" />
          <Numero valor={total("paif_ativos")} rotulo="PAIF ativos" />
          <Numero valor={total("paefi_casos_ativos")} rotulo="PAEFI ativos" />
          <Numero valor={total("casa_abrigo_acolhidos_ativos")} rotulo="Casa Abrigo — acolhidos" />
        </ul>

        <Card className="mt-6">
          <CardHeader>
            <CardTitle as="h2">Por unidade</CardTitle>
          </CardHeader>
          <CardContent className="pb-5">
            <div className="mt-3 overflow-x-auto">
              <table className="w-full text-sm">
                <caption className="sr-only">RMA/MDS por unidade na competência selecionada</caption>
                <thead>
                  <tr className="text-left text-muted">
                    <th scope="col" className="py-2">Unidade</th>
                    <th scope="col">Atendimentos</th>
                    <th scope="col">PAIF ativos</th>
                    <th scope="col">Benefícios concedidos</th>
                    <th scope="col">PAEFI ativos</th>
                    <th scope="col">PAEFI abertos</th>
                    <th scope="col">MSE ativas</th>
                    <th scope="col">MSE concluídas</th>
                    <th scope="col">Conselho Tutelar</th>
                    <th scope="col">Notificações CREAS</th>
                    <th scope="col">Casa Abrigo — ativos</th>
                    <th scope="col">Casa Abrigo — entradas</th>
                    <th scope="col">Encaminhamentos</th>
                    <th scope="col">Ofícios CRAS (recebidos/respondidos)</th>
                    <th scope="col">Atend. coletivos (participantes)</th>
                  </tr>
                </thead>
                <tbody>
                  {linhas.map((l) => (
                    <tr key={l.unidade.id} className="border-t border-surface-border">
                      <th scope="row" className="py-2 text-left font-medium">{l.unidade.nome}</th>
                      <td>{l.atendimentos}</td>
                      <td>{l.paif_ativos}</td>
                      <td>{l.beneficios_concedidos_no_mes}</td>
                      <td>{l.paefi_casos_ativos}</td>
                      <td>{l.paefi_casos_abertos_no_mes}</td>
                      <td>{l.mse_ativas}</td>
                      <td>{l.mse_concluidas_no_mes}</td>
                      <td>{l.conselho_atendimentos_no_mes}</td>
                      <td>{l.creas_notificacoes_recebidas_no_mes}</td>
                      <td>{l.casa_abrigo_acolhidos_ativos}</td>
                      <td>{l.casa_abrigo_entradas_no_mes}</td>
                      <td>{l.encaminhamentos_enviados_no_mes}</td>
                      <td>{l.oficios_cras_recebidos_no_mes}/{l.oficios_cras_respondidos_no_mes}</td>
                      <td>{l.coletivo_total_no_mes}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>
      </DataState>
    </div>
  );
}
