import { TriangleAlert } from "lucide-react";
import Link from "next/link";

import type { OrgaoOrganograma } from "@/lib/nexus/types";

import {
  detalheProcedimentos,
  linkFuncao,
  linkSubfuncao,
  procedimentos,
  rotulo,
  subfuncoesDe,
} from "./organograma";

/** Número em destaque do quadro-resumo. */
function Numero({ valor, rotulo: r }: { valor: number; rotulo: string }) {
  return (
    <div className="flex flex-col items-center rounded-md border border-surface-border px-3 py-1.5">
      <span className="text-xl font-bold text-foreground">{valor}</span>
      <span className="text-[11px] text-muted">{r}</span>
    </div>
  );
}

/** Fichas por secretaria: uma página por secretaria, em formato de cartaz,
 * com as funções em colunas, as subfunções listadas e o quadro-resumo. Para
 * imprimir e levar às entrevistas, junto com o caderno. */
export function OrganogramaFichas({
  orgaos,
  gestao,
}: {
  orgaos: OrgaoOrganograma[];
  gestao: boolean;
}) {
  return (
    <div className="flex flex-col gap-10">
      {orgaos.map((o, i) => {
        const lacunas = subfuncoesDe(o).filter((s) => s.lacunas > 0);
        return (
          <article
            key={o.prefixo}
            aria-labelledby={`ficha-org-${o.prefixo}`}
            className={`rounded-lg border-2 border-foreground/70 p-4 ${i > 0 ? "print:break-before-page" : ""}`}
          >
            <header className="flex flex-wrap items-start justify-between gap-3 border-b-2 border-foreground/70 pb-3">
              <div className="min-w-0">
                <p className="font-mono text-xs text-muted">TTDD {o.prefixo}</p>
                <h2
                  id={`ficha-org-${o.prefixo}`}
                  className="text-xl font-bold leading-tight text-foreground"
                >
                  {o.nome}
                </h2>
                <p className="mt-1 text-xs text-muted">
                  Procedimentos: {detalheProcedimentos(o, gestao)}
                </p>
              </div>
              <div className="flex gap-2">
                <Numero valor={o.funcoes.length} rotulo="funções" />
                <Numero valor={subfuncoesDe(o).length} rotulo="subfunções" />
                <Numero valor={o.series} rotulo="séries" />
                <Numero valor={procedimentos(o, gestao)} rotulo="procedimentos" />
              </div>
            </header>
            <ul
              className="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-3 print:grid-cols-3"
              aria-label={`Funções de ${o.nome}`}
            >
              {o.funcoes.map((f) => (
                <li
                  key={f.codigo}
                  className="break-inside-avoid rounded-md border border-surface-border p-2"
                >
                  <Link href={linkFuncao(o.prefixo, f.codigo)} className="block hover:underline">
                    <span className="font-mono text-[11px] text-muted">{f.codigo}</span>
                    <span className="block text-sm font-bold uppercase leading-tight text-foreground">
                      {f.nome}
                    </span>
                  </Link>
                  <p className="text-[11px] text-muted">{rotulo(f, gestao)}</p>
                  <ul className="mt-1 flex flex-col gap-0.5 text-xs">
                    {f.subfuncoes.map((s) => (
                      <li key={s.codigo} className="flex items-start gap-1">
                        {s.lacunas > 0 ? (
                          <TriangleAlert
                            size={12}
                            className="mt-0.5 shrink-0 text-warning"
                            aria-label="processo sem procedimento"
                          />
                        ) : (
                          <span aria-hidden="true" className="w-3 shrink-0 text-center">
                            •
                          </span>
                        )}
                        <Link href={linkSubfuncao(s.codigo)} className="min-w-0 hover:underline">
                          {s.nome}{" "}
                          <span className="text-muted">
                            ({s.series} {s.series === 1 ? "série" : "séries"}
                            {procedimentos(s, gestao) > 0 && `, ${procedimentos(s, gestao)} proc.`})
                          </span>
                        </Link>
                      </li>
                    ))}
                  </ul>
                </li>
              ))}
            </ul>
            {lacunas.length > 0 && (
              <footer className="mt-3 border-t border-foreground/30 pt-2 text-xs">
                <p className="font-semibold text-foreground">
                  <TriangleAlert
                    size={12}
                    className="mr-1 inline text-warning"
                    aria-hidden="true"
                  />
                  Processos da TTDD sem procedimento{gestao ? "" : " publicado"} ({lacunas.length}):
                </p>
                <p className="text-muted">
                  {lacunas.map((s) => `${s.codigo} ${s.nome}`).join(" · ")}
                </p>
              </footer>
            )}
          </article>
        );
      })}
    </div>
  );
}
