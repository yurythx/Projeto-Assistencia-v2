"use client";

import { Calculator } from "lucide-react";
import { useId, useState } from "react";

import { calcularDestinacao, dataBR, rotuloReferencia } from "@/lib/atlas/temporalidade";
import type { ClassificacaoTTDD } from "@/lib/nexus/types";

import { destinacao } from "./labels";

/** Calculadora de temporalidade: a partir de uma data, quando termina cada
 * fase e quando cabe a destinação final. Estimativa — não substitui a
 * análise da unidade de gestão documental. */
export function CalculadoraTemporalidade({ serie }: { serie: ClassificacaoTTDD }) {
  const id = useId();
  const [data, setData] = useState("");
  const ref = data ? new Date(`${data}T00:00:00Z`) : null;
  const calc = ref && !Number.isNaN(ref.getTime()) ? calcularDestinacao(serie, ref) : null;
  const dest = destinacao(serie.destinacao_final).label.toLowerCase();

  return (
    <section
      aria-labelledby={`${id}-t`}
      className="rounded-lg border border-surface-border bg-surface p-4 print:hidden"
    >
      <h3 id={`${id}-t`} className="flex items-center gap-2 text-sm font-bold text-foreground">
        <Calculator size={16} aria-hidden="true" className="text-primary" /> Calculadora de
        temporalidade
      </h3>
      <label htmlFor={`${id}-d`} className="mt-3 block text-sm font-medium text-foreground">
        {rotuloReferencia(serie)}
      </label>
      <input
        id={`${id}-d`}
        type="date"
        value={data}
        onChange={(e) => setData(e.target.value)}
        className="mt-1 rounded-md border border-surface-border bg-surface px-3 py-2 text-sm text-foreground focus-visible:outline-2 focus-visible:outline-primary"
      />
      <div aria-live="polite" className="mt-3 text-sm">
        {calc && !calc.possivel && <p className="text-warning">{calc.motivo}</p>}
        {calc?.possivel && (
          <ul className="flex flex-col gap-1 text-foreground">
            <li>
              Fase corrente até <strong>{dataBR(calc.fimCorrente)}</strong>
            </li>
            <li>
              {calc.semIntermediaria ? (
                "Sem fase intermediária"
              ) : (
                <>
                  Fase intermediária até <strong>{dataBR(calc.fimIntermediaria)}</strong>
                </>
              )}
            </li>
            <li>
              A partir de <strong>{dataBR(calc.fimIntermediaria)}</strong>: {dest}
            </li>
          </ul>
        )}
      </div>
      <p className="mt-3 text-xs text-muted">
        Estimativa a partir da data informada. A eliminação depende de aprovação da CCPAD; confirme
        com a unidade de gestão documental.
      </p>
    </section>
  );
}
