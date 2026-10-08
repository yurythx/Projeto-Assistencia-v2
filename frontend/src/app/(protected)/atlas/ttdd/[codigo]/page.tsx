"use client";

import { AlertTriangle, Archive, Clock, FolderClock, History } from "lucide-react";
import Link from "next/link";
import { useParams } from "next/navigation";

import { AcoesPagina } from "@/components/atlas/AcoesPagina";
import { CalculadoraTemporalidade } from "@/components/atlas/CalculadoraTemporalidade";
import { ModelosDaSerie } from "@/components/atlas/ModelosDaSerie";
import { SeloVigencia } from "@/components/atlas/SeloVigencia";
import { Trilha } from "@/components/atlas/Trilha";
import { WorkflowList } from "@/components/atlas/WorkflowList";
import { destinacao, fase, revogacao } from "@/components/atlas/labels";
import { DataState } from "@/components/nexus/DataState";
import { Badge } from "@/components/ui/Badge";
import { useApiPage, useApiQuery, withQuery } from "@/lib/api/swr";
import type { ClassificacaoTTDD, HistoricoTTDD, Workflow } from "@/lib/nexus/types";

function Prazo({
  titulo,
  icone,
  children,
}: {
  titulo: string;
  icone: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="rounded-lg border border-surface-border bg-surface p-4">
      <dt className="flex items-center gap-1.5 text-xs font-semibold text-muted">
        {icone} {titulo}
      </dt>
      <dd className="mt-1 text-lg font-semibold text-foreground">{children}</dd>
    </div>
  );
}

/** Procedimentos enquadrados nesta série (filtro codigo_ttdd). */
function Procedimentos({ codigo }: { codigo: string }) {
  const list = useApiPage<Workflow>(
    withQuery("v1/atlas/workflows", { codigo_ttdd: codigo, page_size: 20 }),
  );
  const items = list.data?.items ?? [];
  return (
    <section aria-labelledby="atlas-serie-procs">
      <h2 id="atlas-serie-procs" className="mb-3 text-lg font-bold text-foreground">
        Procedimentos que geram esta série
      </h2>
      <DataState
        loading={list.isLoading}
        error={list.error}
        onRetry={() => void list.mutate()}
        empty={items.length === 0}
        emptyTitle="Nenhum procedimento cadastrado nesta série"
      >
        <WorkflowList items={items} />
      </DataState>
    </section>
  );
}

/** Página da série documental: prazos, destinação, recomendação, fonte
 * oficial, calculadora e os procedimentos que a produzem. */
const EVENTO: Record<HistoricoTTDD["evento"], string> = {
  ALTERADA: "Prazos ou descrição alterados",
  REVOGADA: "Série revogada",
  RESTABELECIDA: "Série restabelecida",
};

/** Histórico da série: cada mudança com os valores que valiam ANTES dela e
 * a publicação que a trouxe (documentos antigos seguem a regra da época). */
function Historico({ codigo }: { codigo: string }) {
  const hist = useApiQuery<HistoricoTTDD[]>(
    `v1/atlas/ttdd/${encodeURIComponent(codigo)}/historico`,
  );
  const itens = hist.data ?? [];
  return (
    <section aria-labelledby="atlas-serie-hist">
      <h2
        id="atlas-serie-hist"
        className="mb-3 flex items-center gap-2 text-lg font-bold text-foreground"
      >
        <History size={18} aria-hidden="true" className="text-primary" /> Histórico
      </h2>
      <DataState
        loading={hist.isLoading}
        error={hist.error}
        onRetry={() => void hist.mutate()}
        empty={itens.length === 0}
        emptyTitle="Sem alterações registradas"
        emptyDescription="Os prazos não mudaram desde a carga da TTDD no sistema."
      >
        <ol className="flex flex-col gap-3">
          {itens.map((h, i) => {
            const a = h.anterior;
            return (
              <li
                key={i}
                className="rounded-lg border border-surface-border bg-surface p-3 text-sm"
              >
                <p className="flex flex-wrap items-baseline justify-between gap-2">
                  <strong className="text-foreground">{EVENTO[h.evento]}</strong>
                  <span className="text-xs text-muted">
                    {new Date(h.registrado_em).toLocaleDateString("pt-BR")}
                    {h.edicao_diario && ` · Diário Oficial nº ${h.edicao_diario}`}
                  </span>
                </p>
                <p className="mt-1 text-muted">
                  Antes: corrente{" "}
                  {fase(a.fase_corrente_anos, a.fase_corrente_condicao, true).toLowerCase()} ·
                  intermediária{" "}
                  {fase(a.fase_interm_anos, a.fase_interm_condicao, false).toLowerCase()} ·{" "}
                  {destinacao(a.destinacao_final).label.toLowerCase()}
                </p>
              </li>
            );
          })}
        </ol>
      </DataState>
    </section>
  );
}

export default function SeriePage() {
  const { codigo: bruto } = useParams<{ codigo: string }>();
  const codigo = decodeURIComponent(bruto);
  const serie = useApiQuery<ClassificacaoTTDD>(`v1/atlas/ttdd/${encodeURIComponent(codigo)}`);
  const c = serie.data;
  const sub = c?.subfuncao;
  const dest = c && destinacao(c.destinacao_final);
  const total =
    c && c.fase_corrente_anos !== null && c.fase_interm_anos !== null
      ? c.fase_corrente_anos + c.fase_interm_anos
      : null;

  return (
    <div className="flex flex-col gap-6">
      <Trilha
        itens={[
          { label: "Atlas", href: "/atlas" },
          { label: "Tabela de Temporalidade", href: "/atlas/ttdd" },
          ...(sub
            ? [
                {
                  label: sub.funcao.orgao.nome,
                  href: `/atlas/ttdd?codigo=${sub.funcao.orgao.prefixo}`,
                },
                { label: sub.funcao.nome, href: `/atlas/ttdd?codigo=${sub.funcao.codigo}` },
                { label: sub.nome, href: `/atlas/ttdd?codigo=${sub.codigo}` },
              ]
            : []),
          { label: codigo },
        ]}
      />
      <DataState
        loading={serie.isLoading}
        error={serie.error}
        empty={!c}
        emptyTitle="Série não encontrada na TTDD"
        onRetry={() => void serie.mutate()}
      >
        {c && dest && (
          <>
            <header className="flex flex-col gap-3">
              <span className="font-mono text-sm font-bold text-primary">{c.codigo}</span>
              <h1 className="text-2xl font-bold text-foreground">{c.descritor}</h1>
              {sub && (
                <p className="text-sm text-muted">
                  {sub.funcao.nome} › {sub.nome}
                </p>
              )}
              {revogacao(c) ? (
                <p
                  role="status"
                  className="flex max-w-3xl items-start gap-2 rounded-md border border-danger/30 bg-danger/10 px-3 py-2 text-sm text-foreground"
                >
                  <AlertTriangle
                    size={16}
                    aria-hidden="true"
                    className="mt-0.5 shrink-0 text-danger"
                  />
                  <span>
                    Série {revogacao(c)}: não está na TTDD em vigor. Os prazos abaixo valem para os
                    documentos produzidos enquanto ela vigorava; para documentos novos, use a série
                    vigente indicada pela gestão documental.
                  </span>
                </p>
              ) : (
                <SeloVigencia orgao={sub?.funcao.orgao} />
              )}
              <AcoesPagina
                rotulo="Perguntar sobre esta série"
                pergunta={`Por quanto tempo guardar "${c.descritor}" (${c.codigo}) e qual a destinação?`}
              />
            </header>

            <dl className="grid gap-3 sm:grid-cols-3">
              <Prazo titulo="Fase corrente" icone={<Clock size={14} aria-hidden="true" />}>
                {fase(c.fase_corrente_anos, c.fase_corrente_condicao, true)}
              </Prazo>
              <Prazo
                titulo="Fase intermediária"
                icone={<FolderClock size={14} aria-hidden="true" />}
              >
                {fase(c.fase_interm_anos, c.fase_interm_condicao, false)}
              </Prazo>
              <Prazo titulo="Destinação final" icone={<Archive size={14} aria-hidden="true" />}>
                <Badge tone={dest.tone}>{dest.label}</Badge>
              </Prazo>
            </dl>
            {total !== null && (
              <p className="text-sm text-muted">
                Guarda total antes da destinação:{" "}
                <strong className="text-foreground">
                  {total === 1 ? "1 ano" : `${total} anos`}
                </strong>
                .
              </p>
            )}

            {(c.observacoes || sub?.recomendacao) && (
              <div className="flex flex-col gap-2">
                {c.observacoes && (
                  <p className="text-sm text-foreground">
                    <strong>Observações:</strong> {c.observacoes}
                  </p>
                )}
                {sub?.recomendacao && (
                  <p className="rounded-md bg-warning/10 px-3 py-2 text-sm text-foreground">
                    <strong>Recomendação da subfunção:</strong> {sub.recomendacao}
                  </p>
                )}
              </div>
            )}

            <div className="max-w-xl">
              <CalculadoraTemporalidade serie={c} />
            </div>

            <ModelosDaSerie codigo={c.codigo} descritor={c.descritor} vigente={!c.revogada_em} />

            <Procedimentos codigo={c.codigo} />

            <Historico codigo={c.codigo} />

            <p className="text-xs text-muted print:hidden">
              <Link
                href={sub ? `/atlas/ttdd?codigo=${encodeURIComponent(sub.codigo)}` : "/atlas/ttdd"}
                className="text-primary hover:underline"
              >
                Ver as demais séries {sub ? `de ${sub.nome}` : "da tabela"}
              </Link>
            </p>
          </>
        )}
      </DataState>
    </div>
  );
}
