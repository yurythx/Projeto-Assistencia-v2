"use client";

import {
  AlertTriangle,
  Clock,
  FileClock,
  Files,
  History,
  Workflow as WorkflowIcon,
  ClipboardCheck,
} from "lucide-react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useState } from "react";

import { AcoesPagina } from "@/components/atlas/AcoesPagina";
import { SeguirProcedimento } from "@/components/atlas/SeguirProcedimento";
import { ValidacaoProcedimento } from "@/components/atlas/ValidacaoProcedimento";
import { CalculadoraTemporalidade } from "@/components/atlas/CalculadoraTemporalidade";
import { ChecklistDocumentos, LinhaDoTempo, prazoTotalDias } from "@/components/atlas/LinhaDoTempo";
import { NovoProcedimentoForm } from "@/components/atlas/NovoProcedimentoForm";
import { SeloVigencia } from "@/components/atlas/SeloVigencia";
import { Trilha } from "@/components/atlas/Trilha";
import { destinacao, fase, NIVEL_ACESSO, revogacao } from "@/components/atlas/labels";
import { DataState } from "@/components/nexus/DataState";
import { useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { apiClient } from "@/lib/api/client";
import { useApiPage, useApiQuery, withQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { ClassificacaoTTDD, Workflow } from "@/lib/nexus/types";

const cartao = "rounded-lg border border-surface-border bg-surface p-5";

function Secao({
  id,
  titulo,
  icone,
  children,
}: {
  id: string;
  titulo: string;
  icone: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <section aria-labelledby={id} className={cartao}>
      <h2 id={id} className="mb-4 flex items-center gap-2 font-bold text-foreground">
        <span className="text-primary">{icone}</span> {titulo}
      </h2>
      {children}
    </section>
  );
}

/** Resumo da temporalidade da série em que o procedimento se enquadra. */
function Temporalidade({ codigo, c }: { codigo: string; c?: ClassificacaoTTDD }) {
  if (!c) return <p className="text-sm text-muted">Série {codigo} não encontrada na TTDD.</p>;
  const dest = destinacao(c.destinacao_final);
  const revogada = revogacao(c);
  return (
    <div className="flex flex-col gap-3 text-sm">
      {revogada && <Badge tone="danger">Série {revogada}</Badge>}
      <Link
        href={`/atlas/ttdd/${encodeURIComponent(c.codigo)}`}
        className="font-medium text-primary hover:underline"
      >
        <span className="font-mono">{c.codigo}</span> — {c.descritor}
      </Link>
      <dl className="grid grid-cols-2 gap-2">
        <div>
          <dt className="text-xs text-muted">Fase corrente</dt>
          <dd className="text-foreground">
            {fase(c.fase_corrente_anos, c.fase_corrente_condicao, true)}
          </dd>
        </div>
        <div>
          <dt className="text-xs text-muted">Fase intermediária</dt>
          <dd className="text-foreground">
            {fase(c.fase_interm_anos, c.fase_interm_condicao, false)}
          </dd>
        </div>
        <div className="col-span-2">
          <dt className="text-xs text-muted">Destinação final</dt>
          <dd>
            <Badge tone={dest.tone}>{dest.label}</Badge>
          </dd>
        </div>
      </dl>
      {c.subfuncao?.recomendacao && (
        <p className="rounded-md bg-warning/10 px-3 py-2 text-xs text-foreground">
          <strong>Recomendação:</strong> {c.subfuncao.recomendacao}
        </p>
      )}
      {!revogada && <SeloVigencia orgao={c.subfuncao?.funcao.orgao} />}
      <CalculadoraTemporalidade serie={c} />
    </div>
  );
}

/** Versões do procedimento (gestão): a vigente e as anteriores, desativadas. */
function Versoes({ codigo, atual }: { codigo: string; atual: string }) {
  const list = useApiPage<Workflow>(
    withQuery("v1/atlas/admin/workflows", { codigo_processual: codigo, page_size: 50 }),
  );
  const versoes = list.data?.items ?? [];
  if (versoes.length < 2) return null;
  return (
    <Secao id="atlas-versoes" titulo="Versões" icone={<History size={16} aria-hidden="true" />}>
      <ul className="flex flex-col gap-1 text-sm">
        {versoes.map((v) => (
          <li key={v.id} className="flex items-center justify-between gap-2">
            {v.id === atual ? (
              <span aria-current="page" className="font-semibold text-foreground">
                Versão {v.versao}
              </span>
            ) : (
              <Link href={`/atlas/procedimentos/${v.id}`} className="text-primary hover:underline">
                Versão {v.versao}
              </Link>
            )}
            <span className="flex items-center gap-2 text-xs text-muted">
              {new Date(v.created_at).toLocaleDateString("pt-BR")}
              <Badge tone={v.ativo ? "success" : "neutral"}>
                {v.ativo ? "Vigente" : "Inativa"}
              </Badge>
            </span>
          </li>
        ))}
      </ul>
    </Secao>
  );
}

/** Página do procedimento: percurso em linha do tempo, peças exigidas e
 * temporalidade. A gestão (atlas:manage) usa a rota administrativa, que
 * também mostra os desativados, e pode ativar/desativar. */
export default function ProcedimentoPage() {
  const { id } = useParams<{ id: string }>();
  const { can } = useNexus();
  const canManage = can("atlas:manage");
  const path = `v1/atlas/${canManage ? "admin/" : ""}workflows/${encodeURIComponent(id)}`;
  const detail = useApiQuery<Workflow>(path);
  const { run, pending } = useAction();
  const router = useRouter();
  const [versionando, setVersionando] = useState(false);
  const wf = detail.data;
  const serieRevogada = wf?.classificacao && revogacao(wf.classificacao);

  async function alternar(ativo: boolean) {
    const res = await run(
      () =>
        apiClient.post<Workflow>(
          `v1/atlas/admin/workflows/${encodeURIComponent(id)}/${ativo ? "ativar" : "desativar"}`,
        ),
      ativo ? "Procedimento ativado" : "Procedimento desativado",
    );
    if (res) await detail.mutate(res.data, { revalidate: false });
  }

  return (
    <div className="flex flex-col gap-6">
      <Trilha
        itens={[
          { label: "Atlas", href: "/atlas" },
          { label: "Procedimentos", href: "/atlas" },
          { label: wf?.codigo_processual ?? "Procedimento" },
        ]}
      />
      <DataState
        loading={detail.isLoading}
        error={detail.error}
        empty={!wf}
        emptyTitle="Procedimento não encontrado"
        onRetry={() => void detail.mutate()}
      >
        {wf && (
          <>
            <header className="flex flex-col gap-3">
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-mono text-sm font-bold text-primary">
                  {wf.codigo_processual} · versão {wf.versao}
                </span>
                <Badge tone={NIVEL_ACESSO[wf.nivel_acesso].tone}>
                  {NIVEL_ACESSO[wf.nivel_acesso].label}
                </Badge>
                {!wf.ativo && <Badge tone="danger">Inativo</Badge>}
              </div>
              <h1 className="text-2xl font-bold text-foreground">{wf.titulo}</h1>
              <p className="max-w-3xl text-muted">{wf.objetivo}</p>
              <dl className="flex flex-wrap gap-x-8 gap-y-2 text-sm">
                <div>
                  <dt className="text-xs text-muted">Público-alvo</dt>
                  <dd className="text-foreground">{wf.publico_alvo}</dd>
                </div>
                <div>
                  <dt className="text-xs text-muted">Etapas</dt>
                  <dd className="text-foreground">{wf.etapas.length}</dd>
                </div>
                <div>
                  <dt className="text-xs text-muted">Prazo previsto</dt>
                  <dd className="flex items-center gap-1 text-foreground">
                    <Clock size={14} aria-hidden="true" /> {prazoTotalDias(wf.etapas)} dias
                  </dd>
                </div>
                {wf.nivel_acesso !== "PUBLICO" && (
                  <div>
                    <dt className="text-xs text-muted">Hipótese legal da restrição</dt>
                    <dd className="text-foreground">{wf.hipotese_legal_restricao}</dd>
                  </div>
                )}
              </dl>
              {serieRevogada && (
                <p
                  role="status"
                  className="flex max-w-3xl items-start gap-2 rounded-md border border-danger/30 bg-danger/10 px-3 py-2 text-sm text-foreground"
                >
                  <AlertTriangle
                    size={16}
                    aria-hidden="true"
                    className="mt-0.5 shrink-0 text-danger"
                  />
                  <span>
                    A série {wf.codigo_ttdd} deste procedimento foi {serieRevogada} e não está na
                    TTDD em vigor.
                    {canManage
                      ? " Revise o enquadramento e publique uma nova versão."
                      : " Confirme a classificação com a gestão documental."}
                  </span>
                </p>
              )}
              <AcoesPagina
                rotulo="Perguntar sobre este fluxo"
                pergunta={`Como funciona o fluxo de "${wf.titulo}" (${wf.codigo_processual}), do início ao fim?`}
              >
                <SeguirProcedimento id={wf.id} />
                {canManage && (
                  <Button size="sm" onClick={() => setVersionando(true)}>
                    Nova versão
                  </Button>
                )}
                {canManage &&
                  wf.situacao !== "RASCUNHO" &&
                  wf.situacao !== "EM_VALIDACAO" &&
                  (wf.ativo ? (
                    <Button
                      variant="secondary"
                      size="sm"
                      loading={pending}
                      onClick={() => void alternar(false)}
                    >
                      Desativar procedimento
                    </Button>
                  ) : (
                    <Button size="sm" loading={pending} onClick={() => void alternar(true)}>
                      Ativar procedimento
                    </Button>
                  ))}
              </AcoesPagina>
            </header>

            <div className="grid gap-6 lg:grid-cols-3">
              <div className="lg:col-span-2">
                <Secao
                  id="atlas-percurso"
                  titulo="Percurso"
                  icone={<WorkflowIcon size={16} aria-hidden="true" />}
                >
                  <div className="pl-3">
                    <LinhaDoTempo etapas={wf.etapas} />
                  </div>
                </Secao>
              </div>
              <div className="flex flex-col gap-6">
                <Secao
                  id="atlas-pecas"
                  titulo="Peças exigidas"
                  icone={<Files size={16} aria-hidden="true" />}
                >
                  <ChecklistDocumentos
                    etapas={wf.etapas}
                    codigoTTDD={wf.codigo_ttdd}
                    gestao={
                      canManage
                        ? {
                            workflowId: wf.id,
                            onAtualizado: (w) => void detail.mutate(w, { revalidate: false }),
                          }
                        : undefined
                    }
                  />
                </Secao>
                <Secao
                  id="atlas-temporalidade"
                  titulo="Temporalidade"
                  icone={<FileClock size={16} aria-hidden="true" />}
                >
                  <Temporalidade codigo={wf.codigo_ttdd} c={wf.classificacao} />
                </Secao>
                {canManage && (
                  <Secao
                    id="atlas-validacao"
                    titulo="Validação"
                    icone={<ClipboardCheck size={16} aria-hidden="true" />}
                  >
                    <ValidacaoProcedimento wf={wf} onAtualizado={() => void detail.mutate()} />
                  </Secao>
                )}
                {canManage && <Versoes codigo={wf.codigo_processual} atual={wf.id} />}
              </div>
            </div>
            <Dialog
              open={versionando}
              onClose={() => setVersionando(false)}
              title={`Nova versão de ${wf.codigo_processual}`}
              description="Preenchido com a versão atual. Ao publicar, as demais versões são desativadas."
              size="lg"
            >
              {versionando && (
                <NovoProcedimentoForm
                  base={wf}
                  onDone={(nova) => {
                    setVersionando(false);
                    router.push(`/atlas/procedimentos/${nova.id}`);
                  }}
                />
              )}
            </Dialog>
          </>
        )}
      </DataState>
    </div>
  );
}
