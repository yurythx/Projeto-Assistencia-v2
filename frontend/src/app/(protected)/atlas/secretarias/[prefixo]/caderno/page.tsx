"use client";

import { ArrowLeft, Printer } from "lucide-react";
import Link from "next/link";
import { useParams, useSearchParams } from "next/navigation";
import { Suspense } from "react";

import { FichaValidacao } from "@/components/atlas/FichaValidacao";
import {
  funcaoDaSecretaria,
  porFuncao,
  SITUACOES,
  useFuncoes,
  useProcedimentosDaSecretaria,
} from "@/components/atlas/secretaria";
import { DataState } from "@/components/nexus/DataState";
import { Button } from "@/components/ui/Button";
import { useApiQuery, withQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { ProcedimentosOrgao, Workflow } from "@/lib/nexus/types";

/** Uma ficha do caderno: carrega o procedimento completo (com as etapas). */
function Ficha({ id }: { id: string }) {
  const detail = useApiQuery<Workflow>(`v1/atlas/admin/workflows/${encodeURIComponent(id)}`);
  return (
    <div className="break-before-page pt-6">
      <DataState
        loading={detail.isLoading}
        error={detail.error}
        onRetry={() => void detail.mutate()}
        empty={!detail.data}
      >
        {detail.data && <FichaValidacao wf={detail.data} aninhada />}
      </DataState>
    </div>
  );
}

/** Caderno da entrevista (ADR 028): capa e sumário da secretaria e a ficha de
 * validação de cada procedimento, uma por página, para imprimir de uma vez. */
function Caderno() {
  const { prefixo } = useParams<{ prefixo: string }>();
  const params = useSearchParams();
  const situacao = params.get("situacao") ?? "";
  const funcao = funcaoDaSecretaria(prefixo, params.get("funcao"));
  const nomeFuncao = useFuncoes(prefixo).find((f) => f.codigo === funcao)?.nome;
  const resumo = useApiQuery<ProcedimentosOrgao[]>("v1/atlas/admin/workflows/secretarias");
  const orgao = resumo.data?.find((o) => o.prefixo === prefixo);
  const lista = useProcedimentosDaSecretaria(prefixo, true, situacao, funcao);
  const items = lista.data?.items ?? [];
  const grupos = porFuncao(items);
  const filtro = SITUACOES.find((s) => s.value === situacao && s.value);

  return (
    <div className="mx-auto flex max-w-4xl flex-col gap-5 text-foreground">
      <div className="flex items-center justify-between gap-2 print:hidden">
        <Link
          href={withQuery(`/atlas/secretarias/${encodeURIComponent(prefixo)}`, {
            situacao,
            funcao,
          })}
          className="inline-flex items-center gap-1 text-sm text-muted hover:text-foreground"
        >
          <ArrowLeft size={14} aria-hidden="true" /> Voltar à secretaria
        </Link>
        <Button size="sm" onClick={() => window.print()} disabled={items.length === 0}>
          <Printer size={14} aria-hidden="true" className="mr-1" /> Imprimir caderno
        </Button>
      </div>
      <DataState
        loading={lista.isLoading}
        error={lista.error}
        onRetry={() => void lista.mutate()}
        empty={items.length === 0}
        emptyTitle="Nenhum procedimento para o caderno"
      >
        <header className="flex flex-col gap-2 border-b-2 border-foreground pb-3">
          <p className="text-xs uppercase tracking-wide">Caderno de validação dos procedimentos</p>
          <h1 className="text-2xl font-bold">{orgao?.nome ?? `Órgão ${prefixo}`}</h1>
          {funcao && (
            <p className="text-lg font-semibold">
              Função {funcao}
              {nomeFuncao && ` — ${nomeFuncao}`}
            </p>
          )}
          <p className="text-sm">
            TTDD {prefixo} · {items.length} {items.length === 1 ? "procedimento" : "procedimentos"}
            {filtro && ` · situação: ${filtro.label.toLowerCase()}`}
          </p>
          <p className="text-sm">Data da entrevista: ____/____/________</p>
        </header>
        <section aria-labelledby="caderno-sumario" className="flex flex-col gap-2">
          <h2 id="caderno-sumario" className="font-semibold">
            Sumário
          </h2>
          {grupos.map((g) => (
            <div key={g.codigo || "outros"} className="text-sm">
              <p className="font-medium">
                {g.codigo && <span className="mr-1 font-mono">{g.codigo}</span>}
                {g.nome}
              </p>
              <ul className="ml-4 list-disc">
                {g.itens.map((wf) => (
                  <li key={wf.id}>
                    <span className="font-mono">{wf.codigo_processual}</span> — {wf.titulo}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </section>
        {grupos
          .flatMap((g) => g.itens)
          .map((wf) => (
            <Ficha key={wf.id} id={wf.id} />
          ))}
      </DataState>
    </div>
  );
}

export default function CadernoPage() {
  const { can } = useNexus();
  if (!can("atlas:manage")) {
    return (
      <p className="text-sm text-muted">
        O caderno da entrevista é da gestão do Atlas (permissão atlas:manage).
      </p>
    );
  }
  return (
    <Suspense>
      <Caderno />
    </Suspense>
  );
}
