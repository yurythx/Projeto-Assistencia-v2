import Link from "next/link";

import type { EstruturaTTDD } from "@/lib/nexus/types";

/** O nó `prefixo` contém o código selecionado? ("2.0" contém "2.0.02.00"). */
export const contem = (prefixo: string, codigo: string) =>
  codigo === prefixo || codigo.startsWith(`${prefixo}.`);

const href = (codigo: string) => `/atlas/ttdd?codigo=${encodeURIComponent(codigo)}`;

function Item({
  codigo,
  label,
  total,
  atual,
}: {
  codigo: string;
  label: string;
  total: number;
  atual: string;
}) {
  const selecionado = codigo === atual;
  return (
    <Link
      href={href(codigo)}
      aria-current={selecionado ? "page" : undefined}
      className={`flex items-baseline justify-between gap-2 rounded-md px-2 py-1 text-sm ${
        selecionado
          ? "bg-primary/10 font-semibold text-primary"
          : "text-foreground hover:bg-surface-hover"
      }`}
    >
      <span>
        <span className="font-mono text-xs text-muted">{codigo}</span> {label}
      </span>
      <span className="shrink-0 text-xs text-muted">{total}</span>
    </Link>
  );
}

/** Plano de classificação da TTDD (órgão › função › subfunção) como
 * navegação: abre só o ramo do código selecionado. */
export function ArvoreTTDD({ estrutura, codigo }: { estrutura: EstruturaTTDD[]; codigo: string }) {
  return (
    <nav aria-label="Plano de classificação da TTDD" className="text-sm">
      <Link
        href="/atlas/ttdd"
        aria-current={codigo === "" ? "page" : undefined}
        className={`block rounded-md px-2 py-1 ${codigo === "" ? "bg-primary/10 font-semibold text-primary" : "hover:bg-surface-hover"}`}
      >
        Todos os órgãos
      </Link>
      <ul className="mt-1 flex flex-col gap-0.5">
        {estrutura.map((o) => (
          <li key={o.prefixo}>
            <Item codigo={o.prefixo} label={o.nome} total={o.total} atual={codigo} />
            {contem(o.prefixo, codigo) && (
              <ul className="ml-3 flex flex-col gap-0.5 border-l border-surface-border pl-2">
                {o.funcoes.map((f) => (
                  <li key={f.codigo}>
                    <Item codigo={f.codigo} label={f.nome} total={f.total} atual={codigo} />
                    {contem(f.codigo, codigo) && f.subfuncoes.length > 0 && (
                      <ul className="ml-3 flex flex-col gap-0.5 border-l border-surface-border pl-2">
                        {f.subfuncoes.map((s) => (
                          <li key={s.codigo}>
                            <Item codigo={s.codigo} label={s.nome} total={s.total} atual={codigo} />
                          </li>
                        ))}
                      </ul>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </li>
        ))}
      </ul>
    </nav>
  );
}
