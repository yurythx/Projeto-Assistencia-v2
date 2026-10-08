"use client";

import Link from "next/link";

import { Trilha } from "@/components/atlas/Trilha";
import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { CoberturaAtlas } from "@/lib/nexus/types";

const pct = (parte: number, todo: number) => (todo ? Math.round((parte / todo) * 100) : 0);

/** Barra de progresso acessível (valor e texto). */
function Barra({ parte, todo, rotulo }: { parte: number; todo: number; rotulo: string }) {
  const p = pct(parte, todo);
  return (
    <div className="flex items-center gap-2">
      <div
        role="progressbar"
        aria-label={rotulo}
        aria-valuenow={p}
        aria-valuemin={0}
        aria-valuemax={100}
        className="h-2 w-24 overflow-hidden rounded-full bg-surface-hover"
      >
        <div className="h-full bg-primary" style={{ width: `${p}%` }} />
      </div>
      <span className="text-xs text-muted">
        {parte}/{todo} ({p}%)
      </span>
    </div>
  );
}

function Numero({
  valor,
  rotulo,
  detalhe,
}: {
  valor: string | number;
  rotulo: string;
  detalhe: string;
}) {
  return (
    <li className="rounded-lg border border-surface-border bg-surface p-4">
      <span className="block text-2xl font-bold text-foreground">{valor}</span>
      <span className="block text-sm font-medium text-foreground">{rotulo}</span>
      <span className="block text-xs text-muted">{detalhe}</span>
    </li>
  );
}

/** Painel de cobertura do Atlas (atlas:manage — ADR 025): quanto da TTDD já
 * tem fluxo e modelo, e a lista de trabalho de quem alimenta o Atlas. */
export default function CoberturaPage() {
  const { can } = useNexus();
  const gestao = can("atlas:manage");
  const painel = useApiQuery<CoberturaAtlas>(gestao ? "v1/atlas/admin/cobertura" : null);
  if (!gestao) {
    return (
      <p className="text-sm text-muted">
        O painel de cobertura é da gestão do Atlas (permissão atlas:manage).
      </p>
    );
  }
  const c = painel.data;
  return (
    <div className="flex flex-col gap-6">
      <Trilha itens={[{ label: "Atlas", href: "/atlas" }, { label: "Cobertura" }]} />
      <PageHeader
        eyebrow="Atlas"
        title="Cobertura do Atlas"
        description="Quanto da Tabela de Temporalidade já tem fluxo homologado e modelo de documento, e o que falta cadastrar."
      />
      <DataState
        loading={painel.isLoading}
        error={painel.error}
        onRetry={() => void painel.mutate()}
        empty={!c}
      >
        {c && (
          <>
            <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4" aria-label="Totais">
              <Numero
                valor={`${pct(c.totais.series_com_procedimento, c.totais.series)}%`}
                rotulo="Séries com fluxo"
                detalhe={`${c.totais.series_com_procedimento} de ${c.totais.series} séries vigentes`}
              />
              <Numero
                valor={c.totais.procedimentos}
                rotulo="Procedimentos em vigor"
                detalhe={`${c.totais.pecas} peças exigidas`}
              />
              <Numero
                valor={`${pct(c.totais.pecas_com_modelo, c.totais.pecas)}%`}
                rotulo="Peças com modelo"
                detalhe={`${c.totais.pecas_com_modelo} de ${c.totais.pecas} (próprio ou da série)`}
              />
              <Numero
                valor={c.totais.modelos}
                rotulo="Modelos ativos"
                detalhe={`${c.totais.modelos_sem_uso} sem uso`}
              />
              <Numero
                valor={c.totais.rascunhos + c.totais.em_validacao}
                rotulo="Em validação"
                detalhe={`${c.totais.rascunhos} rascunhos · ${c.totais.em_validacao} em entrevistas`}
              />
            </ul>

            <Card>
              <CardHeader>
                <CardTitle as="h2">Por secretaria</CardTitle>
              </CardHeader>
              <CardContent className="overflow-x-auto pb-5 pt-3">
                <table className="w-full text-left text-sm">
                  <thead>
                    <tr className="text-xs text-muted">
                      <th scope="col" className="py-2 pr-3">
                        Secretaria
                      </th>
                      <th scope="col" className="py-2 pr-3">
                        Séries
                      </th>
                      <th scope="col" className="py-2 pr-3">
                        Com fluxo
                      </th>
                      <th scope="col" className="py-2">
                        Com modelo
                      </th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-surface-border">
                    {c.orgaos.map((o) => (
                      <tr key={o.prefixo}>
                        <th scope="row" className="py-2 pr-3 font-medium text-foreground">
                          <Link
                            href={`/atlas/ttdd?codigo=${encodeURIComponent(o.prefixo)}`}
                            className="hover:text-primary hover:underline"
                          >
                            {o.nome}
                          </Link>
                        </th>
                        <td className="py-2 pr-3">{o.series}</td>
                        <td className="py-2 pr-3">
                          <Barra
                            parte={o.series_com_procedimento}
                            todo={o.series}
                            rotulo={`${o.nome}: séries com fluxo`}
                          />
                        </td>
                        <td className="py-2">
                          <Barra
                            parte={o.series_com_modelo}
                            todo={o.series}
                            rotulo={`${o.nome}: séries com modelo`}
                          />
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle as="h2">
                  Processos da TTDD sem procedimento ({c.lacunas.length} subfunções)
                </CardTitle>
              </CardHeader>
              <CardContent className="pb-5 pt-3 text-sm">
                <p className="mb-3 text-xs text-muted">
                  Subfunções com séries que descrevem um processo ou pedido (requerimento, licença,
                  certidão…) e nenhum procedimento, nem rascunho. Uma subfunção costuma virar um ou
                  poucos fluxos — não um por série.
                </p>
                {c.lacunas.length === 0 ? (
                  <p className="text-muted">Toda subfunção com processo já tem procedimento.</p>
                ) : (
                  <ul className="flex flex-col gap-2" aria-label="Subfunções sem procedimento">
                    {c.lacunas.map((l) => (
                      <li key={l.codigo}>
                        <Link
                          href={`/atlas/ttdd?codigo=${encodeURIComponent(l.codigo)}`}
                          className="font-medium text-primary hover:underline"
                        >
                          <span className="font-mono">{l.codigo}</span> {l.nome}
                        </Link>{" "}
                        <span className="text-xs text-muted">
                          — {l.orgao} · {l.series} {l.series === 1 ? "série" : "séries"}:{" "}
                          {l.exemplos.join("; ")}
                          {l.series > l.exemplos.length && "…"}
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>

            <div className="grid gap-6 lg:grid-cols-2">
              <Card>
                <CardHeader>
                  <CardTitle as="h2">Peças sem modelo ({c.pecas_sem_modelo.length})</CardTitle>
                </CardHeader>
                <CardContent className="pb-5 pt-3 text-sm">
                  {c.pecas_sem_modelo.length === 0 ? (
                    <p className="text-muted">Todas as peças têm modelo para baixar.</p>
                  ) : (
                    <ul className="flex flex-col gap-1" aria-label="Peças sem modelo">
                      {c.pecas_sem_modelo.slice(0, 200).map((p) => (
                        <li key={`${p.workflow_id}-${p.etapa}-${p.peca}`}>
                          <Link
                            href={`/atlas/procedimentos/${p.workflow_id}`}
                            className="text-primary hover:underline"
                          >
                            {p.peca}
                          </Link>{" "}
                          <span className="text-xs text-muted">
                            — {p.codigo_processual}, etapa {p.etapa}
                          </span>
                        </li>
                      ))}
                    </ul>
                  )}
                </CardContent>
              </Card>
              <Card>
                <CardHeader>
                  <CardTitle as="h2">Modelos sem uso ({c.modelos_sem_uso.length})</CardTitle>
                </CardHeader>
                <CardContent className="pb-5 pt-3 text-sm">
                  {c.modelos_sem_uso.length === 0 ? (
                    <p className="text-muted">
                      Todos os modelos ativos estão ligados a uma peça ou série.
                    </p>
                  ) : (
                    <ul className="flex flex-col gap-1" aria-label="Modelos sem uso">
                      {c.modelos_sem_uso.map((m) => (
                        <li key={m.id}>
                          <Link
                            href={`/atlas/modelos?q=${encodeURIComponent(m.nome)}`}
                            className="text-primary hover:underline"
                          >
                            {m.nome}
                          </Link>
                        </li>
                      ))}
                    </ul>
                  )}
                </CardContent>
              </Card>
            </div>
          </>
        )}
      </DataState>
    </div>
  );
}
