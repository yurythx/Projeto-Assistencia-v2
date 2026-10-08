"use client";

import { Download, FileUp, Search } from "lucide-react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Suspense, useState } from "react";

import { ArvoreTTDD, contem } from "@/components/atlas/ArvoreTTDD";
import { SeloVigencia } from "@/components/atlas/SeloVigencia";
import { Trilha } from "@/components/atlas/Trilha";
import { destinacao, fase } from "@/components/atlas/labels";
import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { Pagination } from "@/components/nexus/Pagination";
import { Badge } from "@/components/ui/Badge";
import { Button, buttonClass } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { useApiPage, useApiQuery, withQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { ClassificacaoTTDD, EstruturaTTDD } from "@/lib/nexus/types";

/** Séries consecutivas da mesma subfunção: a recomendação aparece uma vez. */
function agrupar(items: ClassificacaoTTDD[]) {
  const grupos: {
    chave: string;
    sub?: ClassificacaoTTDD["subfuncao"];
    series: ClassificacaoTTDD[];
  }[] = [];
  for (const c of items) {
    const chave = c.subfuncao?.codigo ?? "";
    const ultimo = grupos.at(-1);
    if (ultimo && ultimo.chave === chave) ultimo.series.push(c);
    else grupos.push({ chave, sub: c.subfuncao, series: [c] });
  }
  return grupos;
}

function Serie({ c }: { c: ClassificacaoTTDD }) {
  const dest = destinacao(c.destinacao_final);
  return (
    <li>
      <Link
        href={`/atlas/ttdd/${encodeURIComponent(c.codigo)}`}
        className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 rounded-md px-3 py-2.5 hover:bg-surface-hover"
      >
        <span className="min-w-0 flex-1">
          <span className="font-mono text-xs font-bold text-primary">{c.codigo}</span>{" "}
          <span className="font-medium text-foreground">{c.descritor}</span>
          <span className="block text-xs text-muted">
            Corrente: {fase(c.fase_corrente_anos, c.fase_corrente_condicao, true)} · Intermediária:{" "}
            {fase(c.fase_interm_anos, c.fase_interm_condicao, false)}
          </span>
        </span>
        <Badge tone={dest.tone}>{dest.label}</Badge>
      </Link>
    </li>
  );
}

/** Consulta da TTDD oficial: plano de classificação à esquerda, séries à
 * direita. Filtros na URL (?codigo, ?q, ?page) — o link é compartilhável. */
function Consulta() {
  const { can } = useNexus();
  const params = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  const codigo = params.get("codigo") ?? "";
  const q = params.get("q") ?? "";
  const page = Math.max(1, Number(params.get("page")) || 1);
  const [texto, setTexto] = useState(q);

  const estrutura = useApiQuery<EstruturaTTDD[]>("v1/atlas/ttdd/estrutura");
  const list = useApiPage<ClassificacaoTTDD>(
    withQuery("v1/atlas/ttdd", { q, codigo, page, page_size: 50 }),
  );
  const items = list.data?.items ?? [];

  const navegar = (p: { codigo?: string; q?: string; page?: number }) =>
    router.replace(
      withQuery(pathname, { codigo, q, ...p, page: p.page && p.page > 1 ? p.page : undefined }),
    );

  const orgao = (estrutura.data ?? []).find((o) => contem(o.prefixo, codigo));
  const funcao = orgao?.funcoes.find((f) => contem(f.codigo, codigo));
  const sub = funcao?.subfuncoes.find((s) => contem(s.codigo, codigo));
  const titulo = sub?.nome ?? funcao?.nome ?? orgao?.nome ?? "Todas as séries";

  return (
    <div className="flex flex-col gap-6">
      <Trilha
        itens={[
          { label: "Atlas", href: "/atlas" },
          { label: "Tabela de Temporalidade", href: "/atlas/ttdd" },
          ...(orgao ? [{ label: orgao.nome, href: `/atlas/ttdd?codigo=${orgao.prefixo}` }] : []),
          ...(funcao ? [{ label: funcao.nome, href: `/atlas/ttdd?codigo=${funcao.codigo}` }] : []),
          ...(sub ? [{ label: sub.nome }] : []),
        ]}
      />
      <PageHeader
        eyebrow="Atlas"
        title="Tabela de Temporalidade e Destinação de Documentos"
        description="Por quanto tempo guardar cada série documental e o que fazer depois — conforme a TTDD publicada no Diário Oficial."
        actions={
          <div className="flex flex-wrap gap-2">
            <a
              href={`/api/backend/${withQuery("v1/atlas/ttdd/exportar", { codigo, q })}`}
              download="ttdd.csv"
              className={buttonClass("secondary")}
            >
              <Download size={16} aria-hidden="true" /> Exportar CSV
            </a>
            {can("atlas:manage") && (
              <Link href="/atlas/ttdd/atualizar" className={buttonClass("secondary")}>
                <FileUp size={16} aria-hidden="true" /> Atualizar TTDD
              </Link>
            )}
          </div>
        }
      />

      <div className="grid gap-6 lg:grid-cols-12">
        <aside className="lg:col-span-4 xl:col-span-3">
          <div className="rounded-lg border border-surface-border bg-surface p-3 lg:sticky lg:top-[calc(var(--topbar-h)+1rem)] lg:max-h-[calc(100dvh-var(--topbar-h)-2rem)] lg:overflow-y-auto">
            <ArvoreTTDD estrutura={estrutura.data ?? []} codigo={codigo} />
          </div>
        </aside>

        <div className="flex flex-col gap-4 lg:col-span-8 xl:col-span-9">
          <form
            role="search"
            className="flex items-end gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              navegar({ q: texto.trim(), page: 1 });
            }}
          >
            <div className="flex-1">
              <Input
                id="atlas-ttdd-q"
                label="Código ou descritor"
                placeholder="Ex.: 2.0.02 ou pregão"
                value={texto}
                onChange={(e) => setTexto(e.target.value)}
              />
            </div>
            <Button type="submit" variant="secondary">
              <Search size={14} aria-hidden="true" /> Buscar
            </Button>
          </form>

          <div className="flex flex-wrap items-center justify-between gap-2">
            <h2 className="text-lg font-bold text-foreground">
              {titulo}
              {list.data?.meta && (
                <span className="ml-2 text-sm font-normal text-muted">
                  {list.data.meta.total_items}{" "}
                  {list.data.meta.total_items === 1 ? "série" : "séries"}
                </span>
              )}
            </h2>
            {orgao && <SeloVigencia orgao={orgao} />}
          </div>

          <DataState
            loading={list.isLoading}
            error={list.error}
            onRetry={() => void list.mutate()}
            empty={items.length === 0}
            emptyTitle="Nenhuma série encontrada"
            emptyDescription="Revise os termos da busca ou escolha outro ramo da tabela."
          >
            <div className="flex flex-col gap-4">
              {agrupar(items).map((g, i) => (
                <section
                  key={`${g.chave}-${i}`}
                  className="rounded-lg border border-surface-border bg-surface"
                >
                  {g.sub && (
                    <header className="border-b border-surface-border px-3 py-2">
                      <h3 className="text-sm font-semibold text-foreground">
                        <span className="font-mono text-xs text-muted">{g.sub.codigo}</span>{" "}
                        {g.sub.funcao.nome} › {g.sub.nome}
                      </h3>
                      {g.sub.recomendacao && (
                        <p className="mt-1 text-xs text-warning">
                          Recomendação: {g.sub.recomendacao}
                        </p>
                      )}
                    </header>
                  )}
                  <ul className="divide-y divide-surface-border">
                    {g.series.map((c) => (
                      <Serie key={c.codigo} c={c} />
                    ))}
                  </ul>
                </section>
              ))}
            </div>
            <Pagination meta={list.data?.meta} onPage={(p) => navegar({ page: p })} />
          </DataState>
        </div>
      </div>
    </div>
  );
}

export default function TTDDPage() {
  return (
    <Suspense>
      <Consulta />
    </Suspense>
  );
}
