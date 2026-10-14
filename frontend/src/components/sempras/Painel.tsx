"use client";

import { DataState } from "@/components/nexus/DataState";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { useApiQuery } from "@/lib/api/swr";
import type { PainelUnidade } from "@/lib/sempras/types";

function Numero({ valor, rotulo }: { valor: string | number; rotulo: string }) {
  return (
    <li className="rounded-lg border border-surface-border bg-surface p-4">
      <span className="block text-2xl font-bold text-foreground">{valor}</span>
      <span className="block text-sm text-muted">{rotulo}</span>
    </li>
  );
}

/** Números do dia de todas as unidades geridas (sempras:gestao): resumo
 * (totais) seguido do detalhe por unidade — mesmo padrão de "Cobertura do
 * Atlas" (atlas/cobertura). */
export function Painel() {
  const painel = useApiQuery<PainelUnidade[]>("v1/sempras/painel", { refreshInterval: 30_000 });
  const linhas = painel.data ?? [];
  const total = (campo: "aguardando" | "em_atendimento" | "concluidos" | "nao_compareceram") =>
    linhas.reduce((soma, l) => soma + l[campo], 0);
  const esperaMedia = linhas.length
    ? Math.round(linhas.reduce((soma, l) => soma + l.espera_media_min, 0) / linhas.length)
    : 0;

  return (
    <DataState loading={painel.isLoading} error={painel.error} onRetry={() => void painel.mutate()} empty={linhas.length === 0} emptyTitle="Nenhuma unidade">
      <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-5" aria-label="Totais do dia">
        <Numero valor={linhas.length} rotulo="Unidades" />
        <Numero valor={total("aguardando")} rotulo="Na fila" />
        <Numero valor={total("em_atendimento")} rotulo="Em atendimento" />
        <Numero valor={total("concluidos")} rotulo="Concluídos" />
        <Numero valor={total("nao_compareceram")} rotulo="Não compareceram" />
      </ul>

      <Card className="mt-6">
        <CardHeader>
          <CardTitle as="h2">Por unidade (espera média geral: {esperaMedia} min)</CardTitle>
        </CardHeader>
        <CardContent className="pb-5">
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
        </CardContent>
      </Card>
    </DataState>
  );
}
