import type { Metadata } from "next";
import Link from "next/link";
import { Download, Search } from "lucide-react";

import { destinacao, fase } from "@/components/atlas/labels";
import { PublicShell } from "@/components/layout/PublicShell";
import { publicGet, publicGetOr } from "@/lib/api/publicServer";
import { APP_URL } from "@/lib/env";
import type { ClassificacaoTTDD, EstruturaTTDD } from "@/lib/nexus/types";

const description =
  "Tabela de Temporalidade e Destinação de Documentos (TTDD) oficial: por quanto tempo cada documento é guardado e o que acontece depois.";

export const metadata: Metadata = {
  title: "Tabela de Temporalidade",
  description,
  alternates: { canonical: `${APP_URL}/temporalidade` },
};

type Props = { searchParams: Promise<{ q?: string; codigo?: string; page?: string }> };

const POR_PAGINA = 50;

/** TTDD pública, sem login (transparência ativa — ADR 026): busca, filtro
 * por secretaria, paginação e exportação em planilha. */
export default async function TemporalidadePublicaPage({ searchParams }: Props) {
  const { q = "", codigo = "", page = "1" } = await searchParams;
  const pagina = Math.max(1, Number.parseInt(page, 10) || 1);
  const filtro = new URLSearchParams({ page: String(pagina), page_size: String(POR_PAGINA) });
  if (q) filtro.set("q", q);
  if (codigo) filtro.set("codigo", codigo);
  const [estrutura, resultado] = await Promise.all([
    publicGetOr<EstruturaTTDD[]>("atlas/ttdd/estrutura", [], 3600),
    publicGet<ClassificacaoTTDD[]>(`atlas/ttdd?${filtro}`, 300).catch(() => ({
      data: [],
      meta: undefined,
    })),
  ]);
  const series = resultado.data;
  const total =
    (resultado.meta as { total_items?: number } | undefined)?.total_items ?? series.length;
  const paginas = Math.max(1, Math.ceil(total / POR_PAGINA));
  const link = (extra: Record<string, string>) => {
    const p = new URLSearchParams({ ...(q ? { q } : {}), ...(codigo ? { codigo } : {}), ...extra });
    return `/temporalidade?${p}`;
  };
  const exportar = new URLSearchParams({ ...(q ? { q } : {}), ...(codigo ? { codigo } : {}) });

  return (
    <PublicShell>
      <div className="mx-auto flex w-full max-w-5xl flex-col gap-6 px-6 py-12">
        <div>
          <p className="dateline">Atlas · Gestão documental</p>
          <h1 className="mt-2 text-3xl font-bold">Tabela de Temporalidade</h1>
          <p className="mt-1 text-muted">{description}</p>
        </div>

        <form
          action="/temporalidade"
          method="get"
          role="search"
          className="flex flex-wrap items-end gap-2"
        >
          {codigo && <input type="hidden" name="codigo" value={codigo} />}
          <div className="flex flex-col gap-1">
            <label htmlFor="ttdd-q" className="text-sm font-medium">
              Procurar documento
            </label>
            <input
              id="ttdd-q"
              name="q"
              defaultValue={q}
              placeholder="Ex.: pregão, folha de pagamento"
              className="w-72 rounded-md border border-surface-border bg-surface px-3 py-2 text-sm"
            />
          </div>
          <button
            type="submit"
            className="inline-flex items-center gap-1 rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground"
          >
            <Search size={14} aria-hidden="true" /> Buscar
          </button>
          <a
            href={`/api/public/v1/atlas/ttdd/exportar${exportar.size ? `?${exportar}` : ""}`}
            className="inline-flex items-center gap-1 rounded-md border border-surface-border px-3 py-2 text-sm hover:bg-surface-hover"
          >
            <Download size={14} aria-hidden="true" /> Baixar planilha (CSV)
          </a>
        </form>

        {estrutura.length > 0 && (
          <nav aria-label="Secretarias" className="flex flex-wrap gap-2 text-sm">
            <Link
              href={q ? `/temporalidade?q=${encodeURIComponent(q)}` : "/temporalidade"}
              aria-current={!codigo ? "page" : undefined}
              className={`rounded-full px-3 py-1 ${!codigo ? "bg-primary text-primary-foreground" : "bg-surface-hover"}`}
            >
              Todas
            </Link>
            {estrutura.map((o) => (
              <Link
                key={o.prefixo}
                href={`/temporalidade?${new URLSearchParams({ ...(q ? { q } : {}), codigo: o.prefixo })}`}
                aria-current={codigo === o.prefixo ? "page" : undefined}
                className={`rounded-full px-3 py-1 ${codigo === o.prefixo ? "bg-primary text-primary-foreground" : "bg-surface-hover"}`}
              >
                {o.nome}
              </Link>
            ))}
          </nav>
        )}

        <p className="text-sm text-muted" aria-live="polite">
          {total} {total === 1 ? "série" : "séries"}
        </p>
        {series.length === 0 ? (
          <p className="rounded-xl border border-dashed border-surface-border p-10 text-center text-muted">
            Nenhuma série encontrada.
          </p>
        ) : (
          <ul
            className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border bg-surface"
            aria-label="Séries"
          >
            {series.map((c) => (
              <li key={c.codigo} className="flex flex-col gap-1 p-4">
                <Link
                  href={`/temporalidade/${encodeURIComponent(c.codigo)}`}
                  className="hover:underline"
                >
                  <span className="font-mono text-xs font-bold text-primary">{c.codigo}</span>{" "}
                  <span className="font-medium">{c.descritor}</span>
                </Link>
                <span className="text-xs text-muted">
                  Corrente: {fase(c.fase_corrente_anos, c.fase_corrente_condicao, true)} ·
                  Intermediária: {fase(c.fase_interm_anos, c.fase_interm_condicao, false)} ·{" "}
                  {destinacao(c.destinacao_final).label}
                </span>
              </li>
            ))}
          </ul>
        )}

        {paginas > 1 && (
          <nav aria-label="Paginação" className="flex items-center justify-between text-sm">
            {pagina > 1 ? (
              <Link
                href={link({ page: String(pagina - 1) })}
                className="text-primary hover:underline"
              >
                ← Anterior
              </Link>
            ) : (
              <span />
            )}
            <span className="text-muted">
              Página {pagina} de {paginas}
            </span>
            {pagina < paginas ? (
              <Link
                href={link({ page: String(pagina + 1) })}
                className="text-primary hover:underline"
              >
                Próxima →
              </Link>
            ) : (
              <span />
            )}
          </nav>
        )}
      </div>
    </PublicShell>
  );
}
