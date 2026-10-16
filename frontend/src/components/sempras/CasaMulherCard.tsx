"use client";

import { ShieldAlert } from "lucide-react";
import { useState, type FormEvent } from "react";

import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import {
  MOTIVOS_ENCERRAMENTO_CASA_MULHER,
  NIVEIS_RISCO,
  RELACOES_AGRESSOR,
  STATUS_MEDIDA_PROTETIVA,
  VIOLENCIAS_CASA_MULHER,
  type CasoCasaMulher,
  type MotivoEncerramentoCasaMulher,
  type NivelRisco,
  type RelacaoAgressor,
  type StatusMedidaProtetiva,
  type ViolenciaCasaMulher,
} from "@/lib/sempras/types";

interface Avaliacao {
  violencias: ViolenciaCasaMulher[];
  relacao_agressor: RelacaoAgressor;
  nivel_risco: NivelRisco;
  medida_protetiva_status: StatusMedidaProtetiva;
  medida_protetiva_processo: string;
  patrulha_maria_da_penha: boolean;
  filhos: number;
  abrigada: boolean;
  observacoes: string;
}

const VAZIA: Avaliacao = {
  violencias: [],
  relacao_agressor: "conjuge_companheiro",
  nivel_risco: "medio",
  medida_protetiva_status: "nao_solicitada",
  medida_protetiva_processo: "",
  patrulha_maria_da_penha: false,
  filhos: 0,
  abrigada: false,
  observacoes: "",
};

const opcoes = <K extends string>(m: Record<K, string>) => Object.entries(m).map(([value, label]) => ({ value, label: label as string }));

/**
 * Casa da Mulher (Lei 11.340/2006 — Maria da Penha, ADR 031): o caso de
 * acompanhamento da mulher em situação de violência — formas de violência,
 * vínculo com o agressor, avaliação de risco, medida protetiva de urgência,
 * Patrulha Maria da Penha e abrigamento sigiloso. O relato fica nas
 * evoluções do prontuário (com o sigilo da psicologia).
 */
export function CasaMulherCard({
  cidadaoId,
  unidadeId,
  casos,
  nomeUnidade,
  onChanged,
}: {
  cidadaoId: string;
  unidadeId: string;
  casos: CasoCasaMulher[];
  nomeUnidade: (id: string) => string;
  onChanged: () => void;
}) {
  const [form, setForm] = useState<Avaliacao>(VAZIA);
  const [editando, setEditando] = useState<string | null>(null);
  const [encerrando, setEncerrando] = useState<string | null>(null);
  const [motivo, setMotivo] = useState<MotivoEncerramentoCasaMulher>("superacao_violencia");
  const { run, pending } = useAction();
  const { run: runEncerrar, pending: pendingEncerrar } = useAction();
  const ativo = casos.find((c) => c.status === "ativo" && c.unidade_id === unidadeId);
  const set = <K extends keyof Avaliacao>(k: K, v: Avaliacao[K]) => setForm((f) => ({ ...f, [k]: v }));

  function reavaliar(c: CasoCasaMulher) {
    setEditando(c.id);
    setForm({
      violencias: c.violencias, relacao_agressor: c.relacao_agressor, nivel_risco: c.nivel_risco,
      medida_protetiva_status: c.medida_protetiva_status, medida_protetiva_processo: c.medida_protetiva_processo,
      patrulha_maria_da_penha: c.patrulha_maria_da_penha, filhos: c.filhos, abrigada: c.abrigada, observacoes: c.observacoes,
    });
  }

  async function salvar(e: FormEvent) {
    e.preventDefault();
    const corpo = { ...form, medida_protetiva_processo: form.medida_protetiva_processo.trim(), observacoes: form.observacoes.trim() };
    const ok = await run(
      () =>
        editando
          ? apiClient.put(`v1/sempras/casa-mulher/${editando}`, corpo)
          : apiClient.post(`v1/sempras/cidadaos/${cidadaoId}/casa-mulher`, { ...corpo, unidade_id: unidadeId }),
      editando ? "Caso reavaliado" : "Caso da Casa da Mulher aberto",
    );
    if (ok) {
      setForm(VAZIA);
      setEditando(null);
      onChanged();
    }
  }

  async function encerrar(id: string) {
    const ok = await runEncerrar(() => apiClient.post(`v1/sempras/casa-mulher/${id}/encerrar`, { motivo }), "Caso encerrado");
    if (ok) {
      setEncerrando(null);
      onChanged();
    }
  }

  const mostrarForm = !ativo || editando !== null;

  return (
    <Card>
      <CardHeader>
        <CardTitle as="h2">Casa da Mulher</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-4 pb-5">
        <p className="flex items-start gap-2 rounded-lg border border-warning/30 bg-warning/5 p-2.5 text-xs text-muted">
          <ShieldAlert size={14} className="mt-0.5 shrink-0 text-warning" aria-hidden="true" />
          Caso sigiloso (Lei 11.340/2006): não aparece no resumo das outras unidades e o painel de chamada mostra só a senha.
        </p>

        {mostrarForm && (
          <form onSubmit={salvar} aria-label={editando ? "Reavaliar o caso" : "Abrir caso"} className="flex flex-col gap-3">
            <fieldset className="flex flex-wrap gap-3 text-sm">
              <legend className="mb-1 text-muted">Formas de violência (art. 7º)</legend>
              {(Object.keys(VIOLENCIAS_CASA_MULHER) as ViolenciaCasaMulher[]).map((v) => (
                <label key={v} className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    checked={form.violencias.includes(v)}
                    onChange={() =>
                      set("violencias", form.violencias.includes(v) ? form.violencias.filter((x) => x !== v) : [...form.violencias, v])
                    }
                  />{" "}
                  {VIOLENCIAS_CASA_MULHER[v]}
                </label>
              ))}
            </fieldset>
            <div className="grid gap-3 sm:grid-cols-3">
              <Select id="cm-relacao" label="Relação com o agressor" value={form.relacao_agressor}
                onChange={(e) => set("relacao_agressor", e.target.value as RelacaoAgressor)} options={opcoes(RELACOES_AGRESSOR)} />
              <Select id="cm-risco" label="Avaliação de risco" value={form.nivel_risco}
                onChange={(e) => set("nivel_risco", e.target.value as NivelRisco)}
                options={Object.entries(NIVEIS_RISCO).map(([value, r]) => ({ value, label: r.label }))} />
              <Select id="cm-medida" label="Medida protetiva" value={form.medida_protetiva_status}
                onChange={(e) => set("medida_protetiva_status", e.target.value as StatusMedidaProtetiva)}
                options={opcoes(STATUS_MEDIDA_PROTETIVA)} />
              <Input id="cm-processo" label="Nº do processo da medida" maxLength={60} value={form.medida_protetiva_processo}
                onChange={(e) => set("medida_protetiva_processo", e.target.value)} autoComplete="off" />
              <Input id="cm-filhos" label="Filhos" type="number" min={0} max={30} value={form.filhos}
                onChange={(e) => set("filhos", Number(e.target.value))} />
            </div>
            <div className="flex flex-wrap gap-4 text-sm">
              <label className="flex items-center gap-2">
                <input type="checkbox" checked={form.patrulha_maria_da_penha}
                  onChange={(e) => set("patrulha_maria_da_penha", e.target.checked)} />
                Acompanhada pela Patrulha Maria da Penha
              </label>
              <label className="flex items-center gap-2">
                <input type="checkbox" checked={form.abrigada} onChange={(e) => set("abrigada", e.target.checked)} />
                Abrigada (endereço sigiloso)
              </label>
            </div>
            <Textarea id="cm-observacoes" label="Observações do caso" rows={3} maxLength={4000} value={form.observacoes}
              onChange={(e) => set("observacoes", e.target.value)} />
            <div className="flex gap-2">
              <Button type="submit" loading={pending} disabled={!unidadeId || form.violencias.length === 0}>
                {editando ? "Salvar reavaliação" : "Abrir caso"}
              </Button>
              {editando && (
                <Button type="button" variant="secondary" onClick={() => { setEditando(null); setForm(VAZIA); }}>
                  Cancelar
                </Button>
              )}
            </div>
          </form>
        )}

        {casos.length === 0 ? (
          <p className="text-sm text-muted">Nenhum caso da Casa da Mulher nas suas unidades.</p>
        ) : (
          <ul aria-label="Casos da Casa da Mulher" className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
            {casos.map((c) => (
              <li key={c.id} className="flex flex-col gap-2 p-3 text-sm">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <p className="font-medium">
                      {c.caso_codigo} · {c.violencias.map((v) => VIOLENCIAS_CASA_MULHER[v]).join(", ")}
                    </p>
                    <p className="text-xs text-muted">
                      {nomeUnidade(c.unidade_id)} · {fmtDateTime(c.data_abertura)} · {RELACOES_AGRESSOR[c.relacao_agressor]} ·
                      medida {STATUS_MEDIDA_PROTETIVA[c.medida_protetiva_status].toLowerCase()}
                      {c.medida_protetiva_processo && ` (${c.medida_protetiva_processo})`}
                      {c.patrulha_maria_da_penha && " · Patrulha Maria da Penha"}
                      {c.abrigada && c.data_abrigamento && ` · abrigada desde ${fmtDateTime(c.data_abrigamento)}`}
                      {c.motivo_encerramento && ` · ${MOTIVOS_ENCERRAMENTO_CASA_MULHER[c.motivo_encerramento]}`}
                    </p>
                  </div>
                  <span className="flex flex-wrap items-center gap-2">
                    <Badge tone={NIVEIS_RISCO[c.nivel_risco].tone}>{NIVEIS_RISCO[c.nivel_risco].label}</Badge>
                    <Badge tone={c.status === "ativo" ? "warning" : "neutral"}>{c.status === "ativo" ? "Ativo" : "Encerrado"}</Badge>
                    {c.status === "ativo" && (
                      <>
                        <Button size="sm" variant="secondary" onClick={() => reavaliar(c)}>Reavaliar</Button>
                        <Button size="sm" variant="danger" onClick={() => setEncerrando(c.id)}>Encerrar</Button>
                      </>
                    )}
                  </span>
                </div>
                {encerrando === c.id && (
                  <div className="flex flex-wrap items-end gap-2">
                    <Select id={`cm-motivo-${c.id}`} label="Motivo do encerramento" value={motivo}
                      onChange={(e) => setMotivo(e.target.value as MotivoEncerramentoCasaMulher)}
                      options={opcoes(MOTIVOS_ENCERRAMENTO_CASA_MULHER)} />
                    <Button size="sm" variant="danger" loading={pendingEncerrar} onClick={() => void encerrar(c.id)}>
                      Confirmar encerramento
                    </Button>
                    <Button size="sm" variant="secondary" onClick={() => setEncerrando(null)}>Voltar</Button>
                  </div>
                )}
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
