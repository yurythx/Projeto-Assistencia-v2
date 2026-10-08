import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ArrowLeft, Download } from "lucide-react";

import { destinacao, fase, fonteTTDD, revogacao } from "@/components/atlas/labels";
import { PublicShell } from "@/components/layout/PublicShell";
import { PublicApiError, publicGet, publicGetOr } from "@/lib/api/publicServer";
import { extensao, urlArquivoModelo } from "@/lib/atlas/modelos";
import { APP_URL } from "@/lib/env";
import type { ClassificacaoTTDD, ModeloDocumento, Workflow } from "@/lib/nexus/types";

type Props = { params: Promise<{ codigo: string }> };

async function carregar(codigo: string): Promise<ClassificacaoTTDD | null> {
  try {
    return (await publicGet<ClassificacaoTTDD>(`atlas/ttdd/${encodeURIComponent(codigo)}`, 300))
      .data;
  } catch (e) {
    if (e instanceof PublicApiError && (e.status === 404 || e.status === 400)) return null;
    throw e;
  }
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const c = await carregar((await params).codigo);
  if (!c) return { title: "Série não encontrada" };
  return {
    title: `${c.codigo} — ${c.descritor}`,
    description: `Temporalidade da série ${c.codigo} da TTDD.`,
    alternates: { canonical: `${APP_URL}/temporalidade/${c.codigo}` },
  };
}

/** Série da TTDD, pública: prazos, destinação, fonte oficial, modelos para
 * baixar e os procedimentos que produzem o documento (ADR 026). */
export default async function SeriePublicaPage({ params }: Props) {
  const c = await carregar((await params).codigo);
  if (!c) notFound();
  const [modelos, procedimentos] = await Promise.all([
    publicGetOr<ModeloDocumento[]>(`atlas/ttdd/${encodeURIComponent(c.codigo)}/modelos`, [], 300),
    publicGetOr<Workflow[]>(
      `atlas/workflows?codigo_ttdd=${encodeURIComponent(c.codigo)}&page_size=50`,
      [],
      300,
    ),
  ]);
  const ativos = modelos.filter((m) => m.ativo);
  const sub = c.subfuncao;
  const aviso = revogacao(c);

  return (
    <PublicShell>
      <article className="mx-auto flex w-full max-w-4xl flex-col gap-6 px-6 py-12">
        <Link
          href="/temporalidade"
          className="inline-flex items-center gap-1 text-sm text-muted hover:text-foreground"
        >
          <ArrowLeft size={14} aria-hidden="true" /> Tabela de Temporalidade
        </Link>
        <header>
          <p className="font-mono text-sm font-bold text-primary">{c.codigo}</p>
          <h1 className="mt-1 text-3xl font-bold">{c.descritor}</h1>
          {sub && (
            <p className="mt-1 text-sm text-muted">
              {sub.funcao.orgao.nome} › {sub.funcao.nome} › {sub.nome}
            </p>
          )}
          {aviso && (
            <p className="mt-2 rounded-md bg-warning/10 px-3 py-2 text-sm">Série {aviso}.</p>
          )}
        </header>

        <dl className="grid gap-3 sm:grid-cols-3">
          <div className="rounded-xl border border-surface-border bg-surface p-4">
            <dt className="text-xs text-muted">Fase corrente</dt>
            <dd className="text-lg font-semibold">
              {fase(c.fase_corrente_anos, c.fase_corrente_condicao, true)}
            </dd>
          </div>
          <div className="rounded-xl border border-surface-border bg-surface p-4">
            <dt className="text-xs text-muted">Fase intermediária</dt>
            <dd className="text-lg font-semibold">
              {fase(c.fase_interm_anos, c.fase_interm_condicao, false)}
            </dd>
          </div>
          <div className="rounded-xl border border-surface-border bg-surface p-4">
            <dt className="text-xs text-muted">Destinação final</dt>
            <dd className="text-lg font-semibold">{destinacao(c.destinacao_final).label}</dd>
          </div>
        </dl>
        {c.observacoes && <p className="text-sm">Observações: {c.observacoes}</p>}
        {sub?.recomendacao && (
          <p className="text-sm text-muted">Recomendação: {sub.recomendacao}</p>
        )}
        {sub && <p className="text-xs text-muted">Fonte: {fonteTTDD(sub.funcao.orgao)}</p>}

        {ativos.length > 0 && (
          <section aria-labelledby="pub-modelos">
            <h2 id="pub-modelos" className="text-xl font-semibold">
              Modelos de documento
            </h2>
            <ul className="mt-3 grid gap-2 sm:grid-cols-2">
              {ativos.map((m) => (
                <li
                  key={m.id}
                  className="flex items-center justify-between gap-2 rounded-xl border border-surface-border bg-surface p-3 text-sm"
                >
                  <span>
                    <span className="block font-medium">{m.nome}</span>
                    <span className="text-xs text-muted">
                      {extensao(m.atual.arquivo_nome)} · versão {m.atual.versao}
                    </span>
                  </span>
                  <a
                    href={urlArquivoModelo(m.id, undefined, true)}
                    download={m.atual.arquivo_nome}
                    className="inline-flex items-center gap-1 font-medium text-primary hover:underline"
                  >
                    <Download size={14} aria-hidden="true" /> Baixar
                    <span className="sr-only">: {m.nome}</span>
                  </a>
                </li>
              ))}
            </ul>
          </section>
        )}

        {procedimentos.length > 0 && (
          <section aria-labelledby="pub-procs">
            <h2 id="pub-procs" className="text-xl font-semibold">
              Procedimentos que produzem este documento
            </h2>
            <ul className="mt-3 flex flex-col gap-1 text-sm">
              {procedimentos.map((w) => (
                <li key={w.id}>
                  <Link href={`/procedimentos/${w.id}`} className="text-primary hover:underline">
                    {w.codigo_processual} — {w.titulo}
                  </Link>
                </li>
              ))}
            </ul>
          </section>
        )}
      </article>
    </PublicShell>
  );
}
