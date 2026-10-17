"use client";

import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { useState } from "react";

import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardContent } from "@/components/ui/Card";
import { Select } from "@/components/ui/Select";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { P, TIPOS_UNIDADE, type TipoUnidade, type Unidade } from "@/lib/sempras/types";

// Painel de chamada com nome: o padrão do ADR 031 §3 para cada tipo.
const PAINEL_PADRAO: Record<TipoUnidade, boolean> = {
  cras: true, scfv: true, centro_pop: true,
  creas: false, casa_mulher: false, casa_abrigo: false, conselho_tutelar: false,
};

function LinhaUnidade({ unidade, onSalvo }: { unidade: Unidade; onSalvo: () => void }) {
  const [tipo, setTipo] = useState<string>(unidade.tipo ?? "");
  const [painel, setPainel] = useState(unidade.painel_exibe_nome);
  const { run, pending } = useAction();
  const mudou = tipo !== (unidade.tipo ?? "") || painel !== unidade.painel_exibe_nome;

  async function salvar() {
    const ok = await run(
      () => apiClient.put(`v1/sempras/unidades/${unidade.id}/configuracao`, { tipo, painel_exibe_nome: tipo !== "" && painel }),
      tipo ? "Unidade configurada" : "A unidade deixou de atender",
    );
    if (ok) onSalvo();
  }

  return (
    <tr className="border-t border-surface-border align-middle">
      <th scope="row" className="py-3 pr-3 text-left font-medium">
        {unidade.nome}
        {!unidade.tipo && <Badge tone="warning" className="ml-2">Não atende</Badge>}
      </th>
      <td className="pr-3">
        <Select
          id={`tipo-${unidade.id}`}
          aria-label={`Tipo de ${unidade.nome}`}
          value={tipo}
          onChange={(e) => {
            const novo = e.target.value;
            setTipo(novo);
            if (novo) setPainel(PAINEL_PADRAO[novo as TipoUnidade]);
          }}
          options={[{ value: "", label: "Sem tipo (não atende)" },
            ...Object.entries(TIPOS_UNIDADE).map(([value, t]) => ({ value, label: t.label }))]}
        />
      </td>
      <td className="pr-3">
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={painel} disabled={!tipo} onChange={(e) => setPainel(e.target.checked)}
            aria-label={`Painel de ${unidade.nome} mostra o nome`} />
          Mostra o nome
        </label>
      </td>
      <td>
        <Button size="sm" onClick={() => void salvar()} loading={pending} disabled={!mudou}>
          Salvar
        </Button>
      </td>
    </tr>
  );
}

/** Configuração das unidades (ADR 031, sempras:gestao): o tipo decide os
 * serviços e a recepção de cada unidade; sem tipo, a unidade não atende.
 * Unidade nova da estrutura (um CRAS que abriu) entra aqui. */
export default function UnidadesPage() {
  const { can } = useNexus();
  const unidades = useApiQuery<Unidade[]>(can(P.gestao) ? "v1/sempras/unidades/configuracao" : null);

  if (!can(P.gestao)) {
    return <p className="text-sm text-muted">A configuração das unidades é da gestão do SEMPRAS (permissão {P.gestao}).</p>;
  }
  const lista = unidades.data ?? [];
  return (
    <div className="flex flex-col gap-6">
      <Link href="/sempras" className="inline-flex items-center gap-1 text-sm text-primary underline">
        <ArrowLeft size={14} aria-hidden="true" /> Voltar ao atendimento
      </Link>
      <PageHeader
        eyebrow="Assistência Social"
        title="SEMPRAS — Unidades"
        description="O tipo de cada unidade decide os serviços, a recepção e o painel de chamada. Unidade sem tipo não atende."
      />
      <DataState loading={unidades.isLoading} error={unidades.error} onRetry={() => void unidades.mutate()}
        empty={lista.length === 0} emptyTitle="Nenhuma unidade da SEMPRAS na sua área">
        <Card>
          <CardContent className="overflow-x-auto pb-5">
            <table className="mt-3 w-full text-sm">
              <caption className="sr-only">Tipo e painel de chamada de cada unidade</caption>
              <thead>
                <tr className="text-left text-muted">
                  <th scope="col" className="py-2">Unidade</th>
                  <th scope="col">Tipo</th>
                  <th scope="col">Painel de chamada</th>
                  <th scope="col"><span className="sr-only">Ações</span></th>
                </tr>
              </thead>
              <tbody>
                {lista.map((u) => (
                  <LinhaUnidade key={`${u.id}-${u.tipo}-${u.painel_exibe_nome}`} unidade={u} onSalvo={() => void unidades.mutate()} />
                ))}
              </tbody>
            </table>
          </CardContent>
        </Card>
      </DataState>
    </div>
  );
}
