"use client";

import { BadgeCheck, ClipboardCheck, Printer } from "lucide-react";
import Link from "next/link";
import { useId, useState, type FormEvent } from "react";

import { fmtDate, useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import type { SituacaoValidacao, ValidacaoAtlas, Workflow } from "@/lib/nexus/types";

export const SITUACAO: Record<
  SituacaoValidacao,
  { label: string; tone: "neutral" | "warning" | "success" }
> = {
  RASCUNHO: { label: "Rascunho", tone: "neutral" },
  EM_VALIDACAO: { label: "Em validação", tone: "warning" },
  HOMOLOGADO: { label: "Homologado", tone: "success" },
};

/** Registrar uma entrevista de validação no departamento. */
function NovaEntrevista({ wf, onDone }: { wf: Workflow; onDone: () => void }) {
  const id = useId();
  const { run, pending } = useAction();
  const hoje = new Date().toISOString().slice(0, 10);

  async function salvar(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    const form = e.currentTarget;
    const ok = await run(
      () =>
        apiClient.post(`v1/atlas/admin/workflows/${wf.id}/validacoes`, {
          realizada_em: String(fd.get("realizada_em") ?? ""),
          unidade: String(fd.get("unidade") ?? "").trim(),
          participantes: String(fd.get("participantes") ?? "").trim(),
          registro: String(fd.get("registro") ?? "").trim(),
          pendencias: String(fd.get("pendencias") ?? "").trim(),
        }),
      "Entrevista registrada",
    );
    if (ok) {
      form.reset();
      onDone();
    }
  }

  return (
    <form
      onSubmit={(e) => void salvar(e)}
      className="flex flex-col gap-3 border-t border-surface-border pt-3"
    >
      <h3 className="text-sm font-semibold text-foreground">Registrar entrevista</h3>
      <div className="grid gap-3 sm:grid-cols-[10rem_1fr]">
        <Input
          id={`${id}-data`}
          name="realizada_em"
          type="date"
          label="Data *"
          required
          max={hoje}
          defaultValue={hoje}
        />
        <Input
          id={`${id}-unidade`}
          name="unidade"
          label="Departamento *"
          required
          maxLength={200}
          placeholder="Ex.: Licitações (SEMAD)"
        />
      </div>
      <Input
        id={`${id}-part`}
        name="participantes"
        label="Participantes (função ou setor)"
        maxLength={1000}
        placeholder="Ex.: chefe do setor, pregoeiro"
      />
      <Textarea
        id={`${id}-reg`}
        name="registro"
        label="O que foi validado ou corrigido *"
        rows={3}
        required
        maxLength={10000}
      />
      <Textarea id={`${id}-pend`} name="pendencias" label="Pendências" rows={2} maxLength={5000} />
      <div>
        <Button type="submit" size="sm" loading={pending}>
          Registrar
        </Button>
      </div>
    </form>
  );
}

/** Validação do procedimento (atlas:manage — ADR 028): situação, registro
 * das entrevistas nos departamentos e a homologação, que publica o fluxo. */
export function ValidacaoProcedimento({
  wf,
  onAtualizado,
}: {
  wf: Workflow;
  onAtualizado: () => void;
}) {
  const lista = useApiQuery<ValidacaoAtlas[]>(`v1/atlas/admin/workflows/${wf.id}/validacoes`);
  const { run, pending } = useAction();
  const [confirmar, setConfirmar] = useState(false);
  // Sem situação (resposta anterior à ADR 028): homologado.
  const atual = wf.situacao ?? "HOMOLOGADO";
  const situacao = SITUACAO[atual];
  const homologado = atual === "HOMOLOGADO";
  const entrevistas = lista.data ?? [];

  async function mudar(nova: SituacaoValidacao) {
    if (
      await run(
        () => apiClient.post(`v1/atlas/admin/workflows/${wf.id}/situacao`, { situacao: nova }),
        "Situação atualizada",
      )
    )
      onAtualizado();
  }

  async function homologar() {
    if (
      await run(
        () => apiClient.post(`v1/atlas/admin/workflows/${wf.id}/homologar`),
        "Procedimento homologado e publicado",
      )
    ) {
      setConfirmar(false);
      onAtualizado();
    }
  }

  return (
    <div className="flex flex-col gap-3 text-sm">
      <div className="flex flex-wrap items-center gap-2">
        <Badge tone={situacao.tone}>{situacao.label}</Badge>
        {!homologado && (
          <span className="text-xs text-muted">
            Não publicado: fora da consulta, da busca e do assistente.
          </span>
        )}
      </div>
      {!homologado && (
        <div className="flex flex-wrap gap-2">
          {atual === "RASCUNHO" ? (
            <Button
              size="sm"
              variant="secondary"
              loading={pending}
              onClick={() => void mudar("EM_VALIDACAO")}
            >
              <ClipboardCheck size={14} aria-hidden="true" className="mr-1" /> Iniciar validação
            </Button>
          ) : (
            <Button
              size="sm"
              variant="ghost"
              loading={pending}
              onClick={() => void mudar("RASCUNHO")}
            >
              Voltar a rascunho
            </Button>
          )}
          {confirmar ? (
            <span className="flex flex-wrap items-center gap-2 rounded-md bg-warning/10 px-2 py-1">
              <span className="text-xs">
                Publica esta versão e substitui a anterior. Confirmar?
              </span>
              <Button size="sm" loading={pending} onClick={() => void homologar()}>
                Homologar e publicar
              </Button>
              <Button size="sm" variant="ghost" onClick={() => setConfirmar(false)}>
                Cancelar
              </Button>
            </span>
          ) : (
            <Button size="sm" onClick={() => setConfirmar(true)}>
              <BadgeCheck size={14} aria-hidden="true" className="mr-1" /> Homologar
            </Button>
          )}
        </div>
      )}
      <Link
        href={`/atlas/procedimentos/${wf.id}/ficha`}
        className="inline-flex items-center gap-1 text-sm text-primary hover:underline"
      >
        <Printer size={14} aria-hidden="true" /> Ficha de validação para imprimir
      </Link>
      <h3 className="mt-1 text-sm font-semibold text-foreground">
        Entrevistas ({entrevistas.length})
      </h3>
      {entrevistas.length === 0 ? (
        <p className="text-xs text-muted">Nenhuma entrevista registrada para este procedimento.</p>
      ) : (
        <ol className="flex flex-col gap-2" aria-label="Entrevistas de validação">
          {entrevistas.map((v) => (
            <li key={v.id} className="rounded-md border border-surface-border p-2">
              <span className="block text-xs text-muted">
                {fmtDate(v.realizada_em)} · {v.unidade}
                {v.participantes && ` · ${v.participantes}`}
              </span>
              <span className="block whitespace-pre-line">{v.registro}</span>
              {v.pendencias && (
                <span className="mt-1 block whitespace-pre-line text-xs text-warning">
                  Pendências: {v.pendencias}
                </span>
              )}
            </li>
          ))}
        </ol>
      )}
      <NovaEntrevista
        wf={wf}
        onDone={() => {
          void lista.mutate();
          onAtualizado();
        }}
      />
    </div>
  );
}
