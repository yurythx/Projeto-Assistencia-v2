"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { ChevronRight } from "lucide-react";

import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { Badge } from "@/components/ui/Badge";
import { useApiQuery } from "@/lib/api/swr";
import { TIPOS_UNIDADE, oferta, type TipoUnidade, type Unidade } from "@/lib/sempras/types";

const ORDEM_TIPOS = Object.keys(TIPOS_UNIDADE) as TipoUnidade[];

/**
 * SEMPRAS — escolha da unidade (ADR 031 §4). Cada unidade tem o seu próprio
 * atendimento em /sempras/u/<unidade>; quem é lotado numa unidade só entra
 * direto nela. O Superintendente vê todas, agrupadas por tipo.
 */
export default function SemprasPage() {
  const router = useRouter();
  const unidades = useApiQuery<Unidade[]>("v1/sempras/unidades");
  const lista = unidades.data ?? [];
  const unica = lista.length === 1 ? lista[0] : undefined;

  useEffect(() => {
    if (unica) router.replace(`/sempras/u/${unica.id}`);
  }, [unica, router]);

  const grupos = ORDEM_TIPOS.map((tipo) => ({ tipo, unidades: lista.filter((u) => u.tipo === tipo) })).filter(
    (g) => g.unidades.length > 0,
  );

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Assistência Social"
        title="SEMPRAS — Atendimento"
        description="Escolha a unidade: cada uma tem a sua recepção, fila e serviços."
      />
      <DataState
        loading={unidades.isLoading || Boolean(unica)}
        error={unidades.error}
        onRetry={() => void unidades.mutate()}
        empty={lista.length === 0}
        emptyTitle="Você não está lotado em nenhuma unidade da SEMPRAS"
        emptyDescription="Peça à TI a inclusão no grupo do AD da sua unidade (ex.: SEMPRAS_CRAS_CENTRAL_TECNICO)."
      >
        <div className="flex flex-col gap-6">
          {grupos.map(({ tipo, unidades: us }) => (
            <section key={tipo} aria-labelledby={`grupo-${tipo}`} className="flex flex-col gap-3">
              <h2 id={`grupo-${tipo}`} className="text-sm font-semibold uppercase tracking-wider text-muted">
                {TIPOS_UNIDADE[tipo].label} <span className="font-normal normal-case">· {TIPOS_UNIDADE[tipo].protecao}</span>
              </h2>
              <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                {us.map((u) => (
                  <li key={u.id}>
                    <Link
                      href={`/sempras/u/${u.id}`}
                      className="flex items-center justify-between gap-3 rounded-xl border border-surface-border bg-surface p-4 transition-colors hover:border-primary/50 hover:bg-primary/5"
                    >
                      <span className="flex flex-col gap-1">
                        <span className="font-semibold text-foreground">{u.nome}</span>
                        <span className="flex flex-wrap gap-1">
                          {!oferta(u, "fila") && <Badge tone="neutral">Sem fila</Badge>}
                          {!u.painel_exibe_nome && oferta(u, "fila") && <Badge tone="neutral">Painel só com senha</Badge>}
                          {!u.ativa && <Badge tone="warning">Desativada</Badge>}
                        </span>
                      </span>
                      <ChevronRight size={18} className="shrink-0 text-muted" aria-hidden="true" />
                    </Link>
                  </li>
                ))}
              </ul>
            </section>
          ))}
        </div>
      </DataState>
    </div>
  );
}
