"use client";

import { ArrowLeft } from "lucide-react";
import Link from "next/link";

import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { situacaoGuarda } from "@/components/tramite/AtlasDoProcesso";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { useApiPage, useApiQuery, withQuery } from "@/lib/api/swr";
import type { ClassificacaoTTDD, Processo } from "@/lib/nexus/types";

/** Os processos de uma série, com a situação da guarda de cada um. */
function GrupoSerie({ codigo, processos }: { codigo: string; processos: Processo[] }) {
  const serie = useApiQuery<ClassificacaoTTDD>(`v1/atlas/ttdd/${encodeURIComponent(codigo)}`);
  const c = serie.data;
  return (
    <Card>
      <CardHeader>
        <CardTitle as="h2" className="text-base">
          <Link
            href={`/atlas/ttdd/${encodeURIComponent(codigo)}`}
            className="hover:text-primary hover:underline"
          >
            <span className="font-mono">{codigo}</span>
            {c && ` — ${c.descritor}`}
          </Link>
        </CardTitle>
      </CardHeader>
      <CardContent className="pb-4 pt-2">
        <ul
          className="flex flex-col divide-y divide-surface-border text-sm"
          aria-label={`Processos da série ${codigo}`}
        >
          {processos.map((p) => (
            <li key={p.id} className="flex flex-wrap items-start justify-between gap-2 py-2">
              <Link href={`/tramite/${p.id}`} className="hover:underline">
                <span className="font-mono text-xs text-muted">{p.numero}</span> {p.assunto}
              </Link>
              <span className="text-xs text-foreground">
                {c
                  ? situacaoGuarda(c, p.concluido_at)
                  : serie.error
                    ? "Série não encontrada no Atlas."
                    : "…"}
              </span>
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  );
}

/** Guarda documental (ADR 026): processos encerrados (concluídos ou
 * arquivados) por série da TTDD, com o que fazer com cada um — continuar
 * guardando, eliminar (com a aprovação da CCPAD) ou recolher ao permanente. */
export default function GuardaPage() {
  const concluidos = useApiPage<Processo>(
    withQuery("v1/tramite/processos", { status: "concluido", page_size: 100 }),
  );
  const arquivados = useApiPage<Processo>(
    withQuery("v1/tramite/processos", { status: "arquivado", page_size: 100 }),
  );
  const todos = [...(concluidos.data?.items ?? []), ...(arquivados.data?.items ?? [])];
  const porSerie = new Map<string, Processo[]>();
  const semSerie: Processo[] = [];
  for (const p of todos) {
    if (p.codigo_ttdd) porSerie.set(p.codigo_ttdd, [...(porSerie.get(p.codigo_ttdd) ?? []), p]);
    else semSerie.push(p);
  }
  const series = [...porSerie.keys()].sort();

  return (
    <div className="flex flex-col gap-6">
      <Link
        href="/tramite"
        className="inline-flex items-center gap-1 text-sm text-muted hover:text-foreground"
      >
        <ArrowLeft size={14} aria-hidden="true" /> Processos
      </Link>
      <PageHeader
        eyebrow="Trâmite"
        title="Guarda documental"
        description="Processos encerrados pela série da TTDD: até quando ficam na fase corrente e no arquivo intermediário, e quais já podem ser eliminados (com a aprovação da CCPAD) ou devem ir ao arquivo permanente. A contagem começa no encerramento do processo."
      />
      <DataState
        loading={concluidos.isLoading || arquivados.isLoading}
        error={concluidos.error ?? arquivados.error}
        empty={todos.length === 0}
        emptyTitle="Nenhum processo encerrado"
      >
        <div className="flex flex-col gap-4">
          {series.map((codigo) => (
            <GrupoSerie key={codigo} codigo={codigo} processos={porSerie.get(codigo)!} />
          ))}
          {semSerie.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle as="h2" className="text-base">
                  Sem série da TTDD ({semSerie.length})
                </CardTitle>
              </CardHeader>
              <CardContent className="pb-4 pt-2 text-sm">
                <p className="mb-2 text-muted">
                  Classifique estes processos (cartão Atlas, no processo) para calcular a guarda.
                </p>
                <ul className="flex flex-col gap-1" aria-label="Processos sem série">
                  {semSerie.map((p) => (
                    <li key={p.id}>
                      <Link href={`/tramite/${p.id}`} className="hover:underline">
                        <span className="font-mono text-xs text-muted">{p.numero}</span> {p.assunto}
                      </Link>
                    </li>
                  ))}
                </ul>
              </CardContent>
            </Card>
          )}
        </div>
      </DataState>
    </div>
  );
}
