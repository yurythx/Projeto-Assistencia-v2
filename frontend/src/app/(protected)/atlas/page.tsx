"use client";

import {
  ArrowRight,
  Building2,
  ChartPie,
  FileClock,
  FileText,
  FileUp,
  Network,
  Plus,
  Workflow as IconeFluxo,
} from "lucide-react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";

import { AtlasBusca } from "@/components/atlas/AtlasBusca";
import { resumoProcedimentos } from "@/components/atlas/secretaria";
import { NovoProcedimentoForm } from "@/components/atlas/NovoProcedimentoForm";
import { WorkflowList } from "@/components/atlas/WorkflowList";
import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { Pagination } from "@/components/nexus/Pagination";
import { Button, buttonClass } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useApiPage, useApiQuery, withQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { EstruturaTTDD, ProcedimentosOrgao, Workflow } from "@/lib/nexus/types";

/** Links antigos (?procedimento=, ?ttdd=) da Busca Global e do assistente
 * continuam valendo: redirecionam para as páginas próprias. */
function useRedirecionaLegado() {
  const params = useSearchParams();
  const router = useRouter();
  const procedimento = params.get("procedimento");
  const ttdd = params.get("ttdd");
  useEffect(() => {
    if (procedimento) router.replace(`/atlas/procedimentos/${encodeURIComponent(procedimento)}`);
    else if (ttdd) router.replace(`/atlas/ttdd/${encodeURIComponent(ttdd)}`);
  }, [procedimento, ttdd, router]);
}

/** Atalhos por secretaria: cada órgão abre a TTDD filtrada. */
function Secretarias() {
  const estrutura = useApiQuery<EstruturaTTDD[]>("v1/atlas/ttdd/estrutura");
  const orgaos = estrutura.data ?? [];
  if (orgaos.length === 0) return null;
  return (
    <section aria-labelledby="atlas-secretarias">
      <div className="flex items-baseline justify-between gap-2">
        <h2 id="atlas-secretarias" className="text-lg font-bold text-foreground">
          Temporalidade por secretaria
        </h2>
        <Link
          href="/atlas/ttdd"
          className="inline-flex items-center gap-1 text-sm text-primary hover:underline"
        >
          Tabela completa <ArrowRight size={14} aria-hidden="true" />
        </Link>
      </div>
      <ul className="mt-3 grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
        {orgaos.map((o) => (
          <li key={o.prefixo}>
            <Link
              href={`/atlas/ttdd?codigo=${encodeURIComponent(o.prefixo)}`}
              className="flex h-full items-start gap-3 rounded-lg border border-surface-border bg-surface p-3 transition-colors hover:border-primary/50 hover:bg-surface-hover"
            >
              <Building2 size={18} aria-hidden="true" className="mt-0.5 shrink-0 text-primary" />
              <span className="min-w-0">
                <span className="block text-sm font-semibold text-foreground">{o.nome}</span>
                <span className="block text-xs text-muted">
                  <span className="font-mono">{o.prefixo}</span> · {o.total}{" "}
                  {o.total === 1 ? "série" : "séries"}
                </span>
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}

/** Atalhos por secretaria para os procedimentos — a página de cada uma é a
 * base das entrevistas de validação. O público só vê as que têm procedimento
 * publicado. */
function ProcedimentosPorSecretaria({ gestao }: { gestao: boolean }) {
  const resumo = useApiQuery<ProcedimentosOrgao[]>(
    `v1/atlas/${gestao ? "admin/" : ""}workflows/secretarias`,
  );
  const orgaos = (resumo.data ?? []).filter(
    (o) => o.publicados + (gestao ? o.em_validacao + o.rascunhos : 0) > 0,
  );
  if (orgaos.length === 0) return null;
  return (
    <section aria-labelledby="atlas-proc-secretarias">
      <h2 id="atlas-proc-secretarias" className="text-lg font-bold text-foreground">
        Procedimentos por secretaria
      </h2>
      <ul className="mt-3 grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
        {orgaos.map((o) => (
          <li key={o.prefixo}>
            <Link
              href={`/atlas/secretarias/${encodeURIComponent(o.prefixo)}`}
              className="flex h-full items-start gap-3 rounded-lg border border-surface-border bg-surface p-3 transition-colors hover:border-primary/50 hover:bg-surface-hover"
            >
              <IconeFluxo size={18} aria-hidden="true" className="mt-0.5 shrink-0 text-primary" />
              <span className="min-w-0">
                <span className="block text-sm font-semibold text-foreground">{o.nome}</span>
                <span className="block text-xs text-muted">
                  <span className="font-mono">{o.prefixo}</span> · {resumoProcedimentos(o, gestao)}
                </span>
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}

/** Procedimentos em cartões. A gestão lista pela rota administrativa
 * (inclui os desativados). */
function Procedimentos({ canManage }: { canManage: boolean }) {
  const [page, setPage] = useState(1);
  const base = canManage ? "v1/atlas/admin/workflows" : "v1/atlas/workflows";
  const list = useApiPage<Workflow>(withQuery(base, { page, page_size: 12 }));
  const items = list.data?.items ?? [];
  return (
    <section aria-labelledby="atlas-procedimentos">
      <h2 id="atlas-procedimentos" className="text-lg font-bold text-foreground">
        Procedimentos
      </h2>
      <div className="mt-3">
        <DataState
          loading={list.isLoading}
          error={list.error}
          onRetry={() => void list.mutate()}
          empty={items.length === 0}
          emptyTitle="Nenhum procedimento cadastrado"
        >
          <WorkflowList items={items} />
          <Pagination meta={list.data?.meta} onPage={setPage} />
        </DataState>
      </div>
    </section>
  );
}

function Inicio() {
  useRedirecionaLegado();
  const { can } = useNexus();
  const router = useRouter();
  const canManage = can("atlas:manage");
  const [creating, setCreating] = useState(false);

  return (
    <div className="flex flex-col gap-8">
      <PageHeader
        eyebrow="Atlas"
        title="Procedimentos e temporalidade"
        description="Como tramitar cada processo (padrão SEI) e por quanto tempo guardar cada documento, conforme a Tabela de Temporalidade e Destinação de Documentos oficial."
        actions={
          <div className="flex flex-wrap gap-2">
            <Link href="/atlas/ttdd" className={buttonClass("secondary")}>
              <FileClock size={16} aria-hidden="true" /> Tabela de Temporalidade
            </Link>
            <Link href="/atlas/organograma" className={buttonClass("secondary")}>
              <Network size={16} aria-hidden="true" /> Organograma
            </Link>
            <Link href="/atlas/modelos" className={buttonClass("secondary")}>
              <FileText size={16} aria-hidden="true" /> Modelos de documento
            </Link>
            {canManage && (
              <>
                <Link href="/atlas/cobertura" className={buttonClass("secondary")}>
                  <ChartPie size={16} aria-hidden="true" /> Cobertura
                </Link>
                <Link href="/atlas/importar" className={buttonClass("secondary")}>
                  <FileUp size={16} aria-hidden="true" /> Importar
                </Link>
              </>
            )}
            {canManage && (
              <Button onClick={() => setCreating(true)}>
                <Plus size={16} aria-hidden="true" className="mr-1" /> Novo procedimento
              </Button>
            )}
          </div>
        }
      />

      <AtlasBusca />
      <ProcedimentosPorSecretaria gestao={canManage} />
      <Procedimentos canManage={canManage} />
      <Secretarias />

      <Dialog
        open={creating}
        onClose={() => setCreating(false)}
        title="Novo procedimento"
        description="Cabeçalho, enquadramento na TTDD e etapas."
        size="lg"
      >
        {creating && (
          <NovoProcedimentoForm
            onDone={(wf) => {
              setCreating(false);
              router.push(`/atlas/procedimentos/${wf.id}`);
            }}
          />
        )}
      </Dialog>
    </div>
  );
}

export default function AtlasPage() {
  return (
    <Suspense>
      <Inicio />
    </Suspense>
  );
}
