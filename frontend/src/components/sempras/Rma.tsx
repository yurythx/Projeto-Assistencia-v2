"use client";

import { useState } from "react";

import { DataState } from "@/components/nexus/DataState";
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

  return (
    <div className="flex flex-col gap-6">
      <Input id="rma-competencia" label="Competência" type="month" value={competencia}
        onChange={(e) => setCompetencia(e.target.value)} className="max-w-[200px]" />

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
