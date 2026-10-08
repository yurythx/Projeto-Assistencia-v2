"use client";

import { ArrowLeft, Printer } from "lucide-react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense } from "react";

import { OrganogramaBlocos } from "@/components/atlas/organograma/OrganogramaBlocos";
import { OrganogramaCaixas } from "@/components/atlas/organograma/OrganogramaCaixas";
import { OrganogramaFichas } from "@/components/atlas/organograma/OrganogramaFichas";
import { OrganogramaTopicos } from "@/components/atlas/organograma/OrganogramaTopicos";
import {
  detalheProcedimentos,
  subfuncoesDe,
  VISOES,
  visaoValida,
  type Visao,
} from "@/components/atlas/organograma/organograma";
import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { Button } from "@/components/ui/Button";
import { Select } from "@/components/ui/Select";
import { useApiQuery, withQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { ContagemOrganograma, OrgaoOrganograma } from "@/lib/nexus/types";

/** Orientação da página impressa de cada visualização. */
const PAPEL: Record<Visao, "landscape" | "portrait"> = {
  caixas: "landscape",
  topicos: "portrait",
  blocos: "landscape",
  fichas: "landscape",
};

function Organograma() {
  const params = useSearchParams();
  const router = useRouter();
  const { can } = useNexus();
  const gestao = can("atlas:manage");
  const visao = visaoValida(params.get("visao"));
  const orgao = params.get("orgao") ?? "";

  const dados = useApiQuery<OrgaoOrganograma[]>(`v1/atlas/${gestao ? "admin/" : ""}organograma`);
  const todos = dados.data ?? [];
  const orgaos = orgao ? todos.filter((o) => o.prefixo === orgao) : todos;
  const total = orgaos.reduce(
    (t, o) => ({
      series: t.series + o.series,
      publicados: t.publicados + o.publicados,
      em_validacao: t.em_validacao + o.em_validacao,
      rascunhos: t.rascunhos + o.rascunhos,
      lacunas: t.lacunas + o.lacunas,
    }),
    { series: 0, publicados: 0, em_validacao: 0, rascunhos: 0, lacunas: 0 } as ContagemOrganograma,
  );
  const ir = (mudanca: { visao?: string; orgao?: string }) =>
    router.replace(withQuery("/atlas/organograma", { visao, orgao, ...mudanca }));

  return (
    <div className="flex flex-col gap-5">
      {/* Orientação do papel conforme a visualização. */}
      <style>{`@media print { @page { size: A4 ${PAPEL[visao]}; margin: 10mm; } }`}</style>
      <Link
        href="/atlas"
        className="inline-flex items-center gap-1 text-sm text-muted hover:text-foreground print:hidden"
      >
        <ArrowLeft size={14} aria-hidden="true" /> Atlas
      </Link>
      <PageHeader
        eyebrow="Atlas · Tabela de Temporalidade"
        title="Organograma da TTDD"
        description="As secretarias, as funções (em geral, os departamentos) e as subfunções da Tabela de Temporalidade, com as séries documentais e os procedimentos de cada uma. Clique numa função para ver os procedimentos dela ou numa subfunção para ver as séries."
        actions={
          <Button
            className="print:hidden"
            onClick={() => window.print()}
            disabled={orgaos.length === 0}
          >
            <Printer size={16} aria-hidden="true" className="mr-1" /> Imprimir
          </Button>
        }
      />

      <div className="flex flex-wrap items-end gap-4 print:hidden">
        <div
          role="group"
          aria-label="Visualização"
          className="flex flex-wrap gap-1 rounded-lg border border-surface-border p-1"
        >
          {VISOES.map((v) => (
            <button
              key={v.id}
              type="button"
              aria-pressed={visao === v.id}
              onClick={() => ir({ visao: v.id })}
              className={`rounded-md px-3 py-1.5 text-sm transition-colors ${
                visao === v.id
                  ? "bg-primary text-primary-foreground"
                  : "text-foreground hover:bg-surface-hover"
              }`}
            >
              {v.label}
            </button>
          ))}
        </div>
        <div className="w-full max-w-sm">
          <Select
            label="Secretaria"
            placeholder="Todas as secretarias"
            options={todos.map((o) => ({ value: o.prefixo, label: `${o.prefixo} · ${o.nome}` }))}
            value={orgao}
            onChange={(e) => ir({ orgao: e.target.value })}
          />
        </div>
      </div>

      <DataState
        loading={dados.isLoading}
        error={dados.error}
        onRetry={() => void dados.mutate()}
        empty={orgaos.length === 0}
        emptyTitle="Nenhuma secretaria na TTDD"
      >
        <p className="text-sm text-muted">
          {orgaos.length} {orgaos.length === 1 ? "secretaria" : "secretarias"} ·{" "}
          {orgaos.reduce((n, o) => n + o.funcoes.length, 0)} funções ·{" "}
          {orgaos.reduce((n, o) => n + subfuncoesDe(o).length, 0)} subfunções · {total.series}{" "}
          séries · procedimentos: {detalheProcedimentos(total, gestao)}
          {total.lacunas > 0 &&
            ` · ${total.lacunas} ${total.lacunas === 1 ? "subfunção" : "subfunções"} com processo sem fluxo${gestao ? "" : " publicado"}`}
        </p>
        {/* A chave remonta a visão ao trocar a secretaria (a árvore reabre). */}
        <div key={`${visao}-${orgao}`}>
          {visao === "caixas" && <OrganogramaCaixas orgaos={orgaos} gestao={gestao} />}
          {visao === "topicos" && <OrganogramaTopicos orgaos={orgaos} gestao={gestao} />}
          {visao === "blocos" && <OrganogramaBlocos orgaos={orgaos} gestao={gestao} />}
          {visao === "fichas" && <OrganogramaFichas orgaos={orgaos} gestao={gestao} />}
        </div>
      </DataState>
    </div>
  );
}

export default function OrganogramaPage() {
  return (
    <Suspense>
      <Organograma />
    </Suspense>
  );
}
