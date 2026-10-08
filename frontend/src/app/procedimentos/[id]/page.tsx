import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ArrowLeft, Download } from "lucide-react";

import { ASSINATURA, FORMATO, destinacao, fase } from "@/components/atlas/labels";
import { PublicShell } from "@/components/layout/PublicShell";
import { PublicApiError, publicGet, publicGetOr } from "@/lib/api/publicServer";
import { urlArquivoModelo } from "@/lib/atlas/modelos";
import { APP_URL } from "@/lib/env";
import type { ModeloDocumento, Workflow } from "@/lib/nexus/types";

type Props = { params: Promise<{ id: string }> };

async function carregar(id: string): Promise<Workflow | null> {
  try {
    return (await publicGet<Workflow>(`atlas/workflows/${encodeURIComponent(id)}`, 300)).data;
  } catch (e) {
    if (e instanceof PublicApiError && (e.status === 404 || e.status === 400)) return null;
    throw e;
  }
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const w = await carregar((await params).id);
  if (!w) return { title: "Procedimento não encontrado" };
  return {
    title: w.titulo,
    description: w.objetivo,
    alternates: { canonical: `${APP_URL}/procedimentos/${w.id}` },
  };
}

/** Procedimento público: etapas, documentos exigidos com o modelo para
 * baixar (o da peça ou o da série) e a guarda pela TTDD (ADR 026). */
export default async function ProcedimentoPublicoPage({ params }: Props) {
  const w = await carregar((await params).id);
  if (!w) notFound();
  const daSerie = await publicGetOr<ModeloDocumento[]>(
    `atlas/ttdd/${encodeURIComponent(w.codigo_ttdd)}/modelos`,
    [],
    300,
  );
  const reserva = daSerie.find((m) => m.ativo);
  const c = w.classificacao;
  const prazo = w.etapas.reduce((t, e) => t + e.prazo_sla_em_dias, 0);

  return (
    <PublicShell>
      <article className="mx-auto flex w-full max-w-4xl flex-col gap-8 px-6 py-12">
        <Link
          href="/procedimentos"
          className="inline-flex items-center gap-1 text-sm text-muted hover:text-foreground"
        >
          <ArrowLeft size={14} aria-hidden="true" /> Procedimentos
        </Link>
        <header>
          <p className="font-mono text-sm text-muted">{w.codigo_processual}</p>
          <h1 className="mt-1 text-3xl font-bold">{w.titulo}</h1>
          <p className="mt-2 text-muted">{w.objetivo}</p>
          <p className="mt-2 text-sm">
            Para: {w.publico_alvo} · {w.etapas.length} {w.etapas.length === 1 ? "etapa" : "etapas"}{" "}
            · prazo previsto de {prazo} {prazo === 1 ? "dia" : "dias"}
          </p>
        </header>

        <section aria-labelledby="pub-etapas">
          <h2 id="pub-etapas" className="text-xl font-semibold">
            Etapas
          </h2>
          <ol className="mt-3 flex flex-col gap-4">
            {w.etapas.map((e) => (
              <li key={e.id} className="rounded-xl border border-surface-border bg-surface p-4">
                <p className="font-semibold">
                  {e.ordem}. {e.nome_setor}{" "}
                  <span className="text-sm font-normal text-muted">
                    ({e.unidade_administrativa})
                  </span>
                </p>
                <p className="text-sm text-muted">
                  Prazo: {e.prazo_sla_em_dias} {e.prazo_sla_em_dias === 1 ? "dia" : "dias"}
                </p>
                <p className="mt-1 text-sm">{e.atribuicoes_setor}</p>
                {e.documentos.length > 0 && (
                  <ul
                    className="mt-3 flex flex-col gap-2"
                    aria-label={`Documentos da etapa ${e.ordem}`}
                  >
                    {e.documentos.map((d) => {
                      const modelo = d.modelo
                        ? { id: d.modelo.id, nome: d.modelo.nome, arquivo: d.modelo.arquivo_nome }
                        : reserva
                          ? {
                              id: reserva.id,
                              nome: reserva.nome,
                              arquivo: reserva.atual.arquivo_nome,
                            }
                          : undefined;
                      return (
                        <li
                          key={d.id}
                          className="flex flex-wrap items-center justify-between gap-2 text-sm"
                        >
                          <span>
                            <span className="font-medium">{d.nome_documento}</span>{" "}
                            <span className="text-xs text-muted">
                              {d.obrigatorio ? "obrigatório" : "opcional"} ·{" "}
                              {FORMATO[d.formato].toLowerCase()} · assinatura{" "}
                              {ASSINATURA[d.tipo_assinatura].toLowerCase()}
                            </span>
                          </span>
                          {modelo && (
                            <a
                              href={urlArquivoModelo(modelo.id, undefined, true)}
                              download={modelo.arquivo}
                              className="inline-flex items-center gap-1 font-medium text-primary hover:underline"
                            >
                              <Download size={14} aria-hidden="true" /> Baixar modelo
                              <span className="sr-only">: {modelo.nome}</span>
                            </a>
                          )}
                        </li>
                      );
                    })}
                  </ul>
                )}
              </li>
            ))}
          </ol>
        </section>

        {c && (
          <section
            aria-labelledby="pub-guarda"
            className="rounded-xl border border-surface-border bg-surface p-4"
          >
            <h2 id="pub-guarda" className="text-xl font-semibold">
              Guarda dos documentos
            </h2>
            <p className="mt-2 text-sm">
              Série{" "}
              <Link
                href={`/temporalidade/${encodeURIComponent(c.codigo)}`}
                className="text-primary hover:underline"
              >
                {c.codigo} — {c.descritor}
              </Link>
              : fase corrente {fase(c.fase_corrente_anos, c.fase_corrente_condicao, true)}; fase
              intermediária {fase(c.fase_interm_anos, c.fase_interm_condicao, false)}; destinação
              final: {destinacao(c.destinacao_final).label.toLowerCase()}.
            </p>
          </section>
        )}
      </article>
    </PublicShell>
  );
}
