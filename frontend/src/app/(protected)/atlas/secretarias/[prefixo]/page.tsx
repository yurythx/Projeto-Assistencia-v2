"use client";

import { ArrowLeft, FileClock, Printer } from "lucide-react";
import Link from "next/link";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { Suspense } from "react";

import {
  funcaoDaSecretaria,
  porFuncao,
  resumoProcedimentos,
  SITUACOES,
  useFuncoes,
  useProcedimentosDaSecretaria,
} from "@/components/atlas/secretaria";
import { WorkflowList } from "@/components/atlas/WorkflowList";
import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { buttonClass } from "@/components/ui/Button";
import { Select } from "@/components/ui/Select";
import { useApiQuery, withQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { CoberturaAtlas, ProcedimentosOrgao } from "@/lib/nexus/types";

/** Subfunções da secretaria com processo na TTDD e nenhum procedimento. */
function Lacunas({ prefixo }: { prefixo: string }) {
  // prefixo: a secretaria ou, com o filtro, a função.
  const cobertura = useApiQuery<CoberturaAtlas>("v1/atlas/admin/cobertura");
  const lacunas = (cobertura.data?.lacunas ?? []).filter((l) => l.codigo.startsWith(`${prefixo}.`));
  if (lacunas.length === 0) return null;
  return (
    <section
      aria-labelledby="secretaria-lacunas"
      className="rounded-lg border border-warning/40 p-4"
    >
      <h2 id="secretaria-lacunas" className="font-semibold text-foreground">
        Processos da TTDD sem procedimento ({lacunas.length})
      </h2>
      <p className="mt-1 text-xs text-muted">
        Pergunte na entrevista se estes processos tramitam e se precisam de fluxo.
      </p>
      <ul className="mt-2 flex flex-col gap-1 text-sm" aria-label="Subfunções sem procedimento">
        {lacunas.map((l) => (
          <li key={l.codigo}>
            <Link
              href={`/atlas/ttdd?codigo=${encodeURIComponent(l.codigo)}`}
              className="text-primary hover:underline"
            >
              <span className="font-mono">{l.codigo}</span> {l.nome}
            </Link>{" "}
            <span className="text-xs text-muted">— {l.exemplos.join("; ")}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}

function Secretaria() {
  const { prefixo } = useParams<{ prefixo: string }>();
  const params = useSearchParams();
  const router = useRouter();
  const { can } = useNexus();
  const gestao = can("atlas:manage");
  const situacao = gestao ? (params.get("situacao") ?? "") : "";
  const funcao = funcaoDaSecretaria(prefixo, params.get("funcao"));
  const funcoes = useFuncoes(prefixo);
  const nomeFuncao = funcoes.find((f) => f.codigo === funcao)?.nome;
  const filtrar = (mudanca: { situacao?: string; funcao?: string }) =>
    router.replace(
      withQuery(`/atlas/secretarias/${encodeURIComponent(prefixo)}`, {
        situacao,
        funcao,
        ...mudanca,
      }),
    );

  const resumo = useApiQuery<ProcedimentosOrgao[]>(
    `v1/atlas/${gestao ? "admin/" : ""}workflows/secretarias`,
  );
  const orgao = resumo.data?.find((o) => o.prefixo === prefixo);
  const lista = useProcedimentosDaSecretaria(prefixo, gestao, situacao, funcao);
  const items = lista.data?.items ?? [];
  const total = lista.data?.meta?.total_items ?? 0;
  const caderno = withQuery(`/atlas/secretarias/${encodeURIComponent(prefixo)}/caderno`, {
    situacao,
    funcao,
  });

  return (
    <div className="flex flex-col gap-6">
      <Link
        href="/atlas"
        className="inline-flex items-center gap-1 text-sm text-muted hover:text-foreground"
      >
        <ArrowLeft size={14} aria-hidden="true" /> Atlas
      </Link>
      <PageHeader
        eyebrow={`Secretaria · TTDD ${prefixo}${nomeFuncao ? ` · ${nomeFuncao}` : ""}`}
        title={orgao?.nome ?? `Órgão ${prefixo}`}
        description={
          orgao
            ? `Procedimentos: ${resumoProcedimentos(orgao, gestao)}.`
            : "Procedimentos enquadrados nesta secretaria da Tabela de Temporalidade."
        }
        actions={
          <div className="flex flex-wrap gap-2">
            <Link
              href={`/atlas/ttdd?codigo=${encodeURIComponent(prefixo)}`}
              className={buttonClass("secondary")}
            >
              <FileClock size={16} aria-hidden="true" /> Temporalidade da secretaria
            </Link>
            {gestao && items.length > 0 && (
              <Link href={caderno} className={buttonClass("primary")}>
                <Printer size={16} aria-hidden="true" /> Caderno da entrevista
              </Link>
            )}
          </div>
        }
      />

      <div className="flex flex-wrap gap-4">
        {funcoes.length > 0 && (
          <div className="w-full max-w-md">
            <Select
              label="Função (departamento)"
              placeholder="Todas as funções"
              options={funcoes.map((f) => ({ value: f.codigo, label: `${f.codigo} · ${f.nome}` }))}
              value={funcao}
              onChange={(e) => filtrar({ funcao: e.target.value })}
            />
          </div>
        )}
        {gestao && (
          <div className="w-full max-w-xs">
            <Select
              label="Situação"
              options={SITUACOES}
              value={situacao}
              onChange={(e) => filtrar({ situacao: e.target.value })}
            />
          </div>
        )}
      </div>

      <DataState
        loading={lista.isLoading}
        error={lista.error}
        onRetry={() => void lista.mutate()}
        empty={items.length === 0}
        emptyTitle={
          situacao || funcao
            ? "Nenhum procedimento com estes filtros"
            : "Nenhum procedimento nesta secretaria"
        }
      >
        <div className="flex flex-col gap-6">
          {porFuncao(items).map((g) => (
            <section key={g.codigo || "outros"} aria-labelledby={`funcao-${g.codigo || "outros"}`}>
              <h2
                id={`funcao-${g.codigo || "outros"}`}
                className="mb-2 text-base font-bold text-foreground"
              >
                {g.codigo && <span className="mr-2 font-mono text-sm text-muted">{g.codigo}</span>}
                {g.nome}
              </h2>
              <WorkflowList items={g.itens} />
            </section>
          ))}
          {total > items.length && (
            <p className="text-sm text-muted">
              Mostrando {items.length} de {total}. Filtre pela função ou pela situação para ver os
              demais.
            </p>
          )}
        </div>
      </DataState>

      {gestao && <Lacunas prefixo={funcao || prefixo} />}
    </div>
  );
}

export default function SecretariaPage() {
  return (
    <Suspense>
      <Secretaria />
    </Suspense>
  );
}
