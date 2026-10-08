"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { ArrowLeft, Lock, NotebookPen } from "lucide-react";
import { useState, type FormEvent } from "react";

import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { P, STATUS, TIPOS, rotuloPrioridade, type Prontuario, type Unidade } from "@/lib/sempras/types";

/**
 * Ficha do munícipe (prontuário): cadastro, atendimentos e evoluções da
 * área de quem consulta. Toda abertura é registrada na trilha de
 * auditoria; registros psicológicos aparecem sem o texto para quem não
 * tem o sigilo da psicologia (ADR 030 §5–6).
 */
export default function FichaPage() {
  const { id } = useParams<{ id: string }>();
  const { can } = useNexus();
  const { run, pending } = useAction();
  const ficha = useApiQuery<Prontuario>(`v1/sempras/cidadaos/${id}/prontuario`, { revalidateOnFocus: false });
  const unidades = useApiQuery<Unidade[]>("v1/sempras/unidades");
  const [unidade, setUnidade] = useState("");
  const [tipo, setTipo] = useState("social");
  const [texto, setTexto] = useState("");
  const nomes = new Map((unidades.data ?? []).map((u) => [u.id, u.nome]));
  const unidadeId = unidade || unidades.data?.[0]?.id || "";
  const tipos = can(P.psicologia) ? TIPOS : TIPOS.filter((t) => t.value === "social");

  async function registrar(e: FormEvent) {
    e.preventDefault();
    const ok = await run(
      () => apiClient.post(`v1/sempras/cidadaos/${id}/registros`, { unidade_id: unidadeId, tipo, texto: texto.trim() }),
      "Evolução registrada",
    );
    if (ok) {
      setTexto("");
      void ficha.mutate();
    }
  }

  const c = ficha.data?.cidadao;
  return (
    <div className="flex flex-col gap-6">
      <Link href="/sempras" className="inline-flex items-center gap-1 text-sm text-primary underline">
        <ArrowLeft size={14} aria-hidden="true" /> Voltar ao atendimento
      </Link>
      <DataState loading={ficha.isLoading} error={ficha.error} onRetry={() => void ficha.mutate()} empty={!c}>
        {c && (
          <>
            <PageHeader eyebrow="Ficha do munícipe" title={c.nome_social || c.nome}
              description="Acesso registrado na trilha de auditoria (LGPD). Mostra só o histórico das unidades onde você atua." />
            <Card>
              <CardHeader>
                <CardTitle as="h2">Cadastro</CardTitle>
              </CardHeader>
              <CardContent className="pb-5">
                <dl className="mt-3 grid gap-x-6 gap-y-2 text-sm sm:grid-cols-2">
                  {[
                    ["Nome civil", c.nome], ["Nome social", c.nome_social], ["CPF", c.cpf], ["NIS", c.nis],
                    ["Nascimento", c.data_nascimento], ["Mãe", c.nome_mae], ["Telefone", c.telefone], ["Endereço", c.endereco],
                  ].map(([rotulo, valor]) => (
                    <div key={rotulo}>
                      <dt className="text-muted">{rotulo}</dt>
                      <dd>{valor || "—"}</dd>
                    </div>
                  ))}
                </dl>
              </CardContent>
            </Card>

            <div className="grid gap-6 xl:grid-cols-2">
              <Card>
                <CardHeader>
                  <CardTitle as="h2">Atendimentos</CardTitle>
                </CardHeader>
                <CardContent className="pb-5">
                  {ficha.data!.atendimentos.length === 0 ? (
                    <p className="mt-3 text-sm text-muted">Sem atendimentos nas suas unidades.</p>
                  ) : (
                    <ul aria-label="Atendimentos" className="mt-3 flex flex-col gap-2 text-sm">
                      {ficha.data!.atendimentos.map((a) => (
                        <li key={a.id} className="flex flex-wrap items-center gap-2">
                          <Badge tone={STATUS[a.status].tone}>{STATUS[a.status].label}</Badge>
                          <span>{a.tipo === "psicologico" ? "Psicológico" : "Social"}</span>
                          {a.prioridade && <span className="text-muted">{rotuloPrioridade(a.prioridade)}</span>}
                          <span className="text-muted">{nomes.get(a.unidade_id) ?? ""} · {fmtDateTime(a.created_at)}</span>
                        </li>
                      ))}
                    </ul>
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle as="h2">Evoluções</CardTitle>
                </CardHeader>
                <CardContent className="flex flex-col gap-4 pb-5">
                  <form onSubmit={registrar} className="mt-3 flex flex-col gap-3">
                    <div className="grid gap-3 sm:grid-cols-2">
                      <Select id="evo-unidade" label="Unidade" value={unidadeId} onChange={(e) => setUnidade(e.target.value)}
                        options={(unidades.data ?? []).map((u) => ({ value: u.id, label: u.nome }))} />
                      <Select id="evo-tipo" label="Tipo" value={tipo} onChange={(e) => setTipo(e.target.value)} options={tipos} />
                    </div>
                    <Textarea id="evo-texto" label="Nova evolução *" value={texto} onChange={(e) => setTexto(e.target.value)}
                      maxLength={20000} rows={4} required />
                    <div>
                      <Button type="submit" loading={pending} disabled={!unidadeId}>
                        <NotebookPen size={16} aria-hidden="true" className="mr-1" /> Registrar
                      </Button>
                    </div>
                  </form>
                  <ol aria-label="Evoluções" className="flex flex-col gap-3">
                    {ficha.data!.registros.map((r) => (
                      <li key={r.id} className="rounded-lg border border-surface-border p-3 text-sm">
                        <p className="text-xs text-muted">
                          {fmtDateTime(r.created_at)} · {r.autor_nome || "—"} · {nomes.get(r.unidade_id) ?? ""} ·{" "}
                          {r.tipo === "psicologico" ? "Psicológico" : "Social"}
                        </p>
                        {r.sigiloso ? (
                          <p className="mt-1 inline-flex items-center gap-1 text-muted">
                            <Lock size={14} aria-hidden="true" /> Registro sigiloso da psicologia
                          </p>
                        ) : (
                          <p className="mt-1 whitespace-pre-wrap">{r.texto}</p>
                        )}
                      </li>
                    ))}
                  </ol>
                </CardContent>
              </Card>
            </div>
          </>
        )}
      </DataState>
    </div>
  );
}
