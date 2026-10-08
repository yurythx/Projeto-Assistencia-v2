"use client";

import { DataState } from "@/components/nexus/DataState";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { useApiQuery } from "@/lib/api/swr";
import type { PainelUnidade } from "@/lib/sempras/types";

/** Números do dia de todas as unidades geridas (sempras:gestao). */
export function Painel() {
  const painel = useApiQuery<PainelUnidade[]>("v1/sempras/painel", { refreshInterval: 30_000 });
  const linhas = painel.data ?? [];
  return (
    <Card>
      <CardHeader>
        <CardTitle as="h2">Painel das unidades (hoje)</CardTitle>
      </CardHeader>
      <CardContent className="pb-5">
        <DataState loading={painel.isLoading} error={painel.error} onRetry={() => void painel.mutate()} empty={linhas.length === 0} emptyTitle="Nenhuma unidade">
          <div className="mt-3 overflow-x-auto">
            <table className="w-full text-sm">
              <caption className="sr-only">Atendimentos do dia por unidade</caption>
              <thead>
                <tr className="text-left text-muted">
                  <th scope="col" className="py-2">Unidade</th>
                  <th scope="col">Na fila</th>
                  <th scope="col">Em atendimento</th>
                  <th scope="col">Concluídos</th>
                  <th scope="col">Não compareceram</th>
                  <th scope="col">Espera média</th>
                </tr>
              </thead>
              <tbody>
                {linhas.map((l) => (
                  <tr key={l.unidade.id} className="border-t border-surface-border">
                    <th scope="row" className="py-2 text-left font-medium">{l.unidade.nome}</th>
                    <td>{l.aguardando}</td>
                    <td>{l.em_atendimento}</td>
                    <td>{l.concluidos}</td>
                    <td>{l.nao_compareceram}</td>
                    <td>{Math.round(l.espera_media_min)} min</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </DataState>
      </CardContent>
    </Card>
  );
}
