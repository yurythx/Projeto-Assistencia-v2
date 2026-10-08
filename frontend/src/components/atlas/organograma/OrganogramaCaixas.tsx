import { TriangleAlert } from "lucide-react";
import Link from "next/link";

import type { FuncaoOrganograma, OrgaoOrganograma } from "@/lib/nexus/types";

import { linkFuncao, linkSecretaria, linkSubfuncao, procedimentos, rotulo } from "./organograma";

/** Até 6 funções por linha; a Saúde (12) fica em duas. */
const POR_LINHA = 6;

function linhas<T>(itens: T[]): T[][] {
  const out: T[][] = [];
  for (let i = 0; i < itens.length; i += POR_LINHA) out.push(itens.slice(i, i + POR_LINHA));
  return out;
}

/** Linha vertical entre as caixas. */
const Haste = ({ className = "" }: { className?: string }) => (
  <span aria-hidden="true" className={`mx-auto block h-4 w-px bg-foreground/40 ${className}`} />
);

function CaixaFuncao({
  prefixo,
  f,
  gestao,
}: {
  prefixo: string;
  f: FuncaoOrganograma;
  gestao: boolean;
}) {
  return (
    <li className="flex min-w-0 flex-col">
      <Haste />
      <Link
        href={linkFuncao(prefixo, f.codigo)}
        className="block rounded-md border-2 border-primary/60 bg-surface p-2 text-center hover:bg-surface-hover"
      >
        <span className="block font-mono text-[11px] text-muted">{f.codigo}</span>
        <span className="block text-sm font-semibold leading-tight text-foreground">{f.nome}</span>
        <span className="mt-1 block text-[11px] text-muted">{rotulo(f, gestao)}</span>
      </Link>
      <ul
        className="mt-1 flex flex-col gap-0.5 border-l border-foreground/30 pl-2 text-[11px] leading-snug"
        aria-label={`Subfunções de ${f.nome}`}
      >
        {f.subfuncoes.map((s) => (
          <li key={s.codigo} className="flex items-start gap-1">
            <Link href={linkSubfuncao(s.codigo)} className="min-w-0 hover:underline">
              <span className="font-mono text-muted">{s.codigo.split(".").at(-1)}</span> {s.nome}
              {procedimentos(s, gestao) > 0 && (
                <>
                  {" "}
                  <span className="text-primary">({procedimentos(s, gestao)})</span>
                </>
              )}
            </Link>
            {s.lacunas > 0 && (
              <TriangleAlert
                size={11}
                className="mt-0.5 shrink-0 text-warning"
                aria-label="processo sem procedimento"
              />
            )}
          </li>
        ))}
      </ul>
    </li>
  );
}

/** Organograma clássico: a secretaria no topo e as funções em caixas ligadas
 * por linhas, com as subfunções listadas sob cada uma. Na impressão, uma
 * secretaria por página (paisagem). */
export function OrganogramaCaixas({
  orgaos,
  gestao,
}: {
  orgaos: OrgaoOrganograma[];
  gestao: boolean;
}) {
  return (
    <div className="flex flex-col gap-10">
      {orgaos.map((o, i) => (
        <section
          key={o.prefixo}
          aria-labelledby={`caixas-${o.prefixo}`}
          className={`overflow-x-auto pb-2 ${i > 0 ? "print:break-before-page" : ""}`}
        >
          <div className="mx-auto min-w-[40rem]">
            <Link
              href={linkSecretaria(o.prefixo)}
              className="mx-auto block max-w-md rounded-lg border-2 border-primary bg-primary/10 p-3 text-center hover:bg-primary/15"
            >
              <span className="block font-mono text-xs text-muted">TTDD {o.prefixo}</span>
              <h2 id={`caixas-${o.prefixo}`} className="font-bold leading-tight text-foreground">
                {o.nome}
              </h2>
              <span className="mt-1 block text-xs text-muted">
                {rotulo(o, gestao)}
                {o.lacunas > 0 && ` · ${o.lacunas} sem fluxo`}
              </span>
            </Link>
            {linhas(o.funcoes).map((linha, n) => (
              <div key={n}>
                <Haste />
                <ul
                  className="relative grid gap-x-3"
                  style={{ gridTemplateColumns: `repeat(${linha.length}, minmax(0, 1fr))` }}
                  aria-label={`Funções de ${o.nome}${n > 0 ? ` (continuação)` : ""}`}
                >
                  {/* Barramento entre o centro da primeira e o da última caixa. */}
                  {linha.length > 1 && (
                    <span
                      aria-hidden="true"
                      className="absolute top-0 h-px bg-foreground/40"
                      style={{ left: `${50 / linha.length}%`, right: `${50 / linha.length}%` }}
                    />
                  )}
                  {linha.map((f) => (
                    <CaixaFuncao key={f.codigo} prefixo={o.prefixo} f={f} gestao={gestao} />
                  ))}
                </ul>
              </div>
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}
