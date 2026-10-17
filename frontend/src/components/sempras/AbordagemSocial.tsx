"use client";

import { useState, type FormEvent } from "react";

import { DataState } from "@/components/nexus/DataState";
import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import { RESULTADOS_ABORDAGEM, type Abordagem, type ResultadoAbordagem } from "@/lib/sempras/types";

function competenciaAtual(): string {
  const agora = new Date();
  return `${agora.getFullYear()}-${String(agora.getMonth() + 1).padStart(2, "0")}`;
}

/** Agora no formato de <input type="datetime-local"> (hora local). */
function agoraLocal(): string {
  const d = new Date();
  d.setMinutes(d.getMinutes() - d.getTimezoneOffset());
  return d.toISOString().slice(0, 16);
}

/**
 * Abordagem social do Centro POP (busca ativa da equipe na rua): onde,
 * quando, a quem — o munícipe cadastrado (`cidadaoId`, quando aberta pela
 * ficha) ou só uma descrição, quando a pessoa não se identifica — e o
 * resultado. Alimenta o RMA do Centro POP.
 */
export function AbordagemSocial({ unidadeId, cidadaoId }: { unidadeId: string; cidadaoId?: string }) {
  const [mes, setMes] = useState(competenciaAtual);
  const lista = useApiQuery<Abordagem[]>(cidadaoId ? null : `v1/sempras/unidades/${unidadeId}/abordagens?mes=${mes}`);
  const [descricao, setDescricao] = useState("");
  const [local, setLocal] = useState("");
  const [quando, setQuando] = useState(agoraLocal);
  const [resultado, setResultado] = useState<ResultadoAbordagem>("orientado");
  const [drogas, setDrogas] = useState(false);
  const [saude, setSaude] = useState(false);
  const [obs, setObs] = useState("");
  const { run, pending } = useAction();

  async function registrar(e: FormEvent) {
    e.preventDefault();
    const ok = await run(
      () =>
        apiClient.post(`v1/sempras/unidades/${unidadeId}/abordagens`, {
          ...(cidadaoId ? { cidadao_id: cidadaoId } : {}),
          descricao_pessoa: descricao.trim(),
          local: local.trim(),
          data_abordagem: new Date(quando).toISOString(),
          resultado,
          uso_drogas_aparente: drogas,
          demanda_saude: saude,
          observacoes: obs.trim(),
        }),
      "Abordagem registrada",
    );
    if (ok) {
      setDescricao("");
      setLocal("");
      setObs("");
      setDrogas(false);
      setSaude(false);
      if (!cidadaoId) void lista.mutate();
    }
  }

  const itens = lista.data ?? [];
  const podeRegistrar = local.trim().length >= 3 && (Boolean(cidadaoId) || descricao.trim() !== "");
  return (
    <Card>
      <CardHeader>
        <CardTitle as="h2">Abordagem social</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-4 pb-5">
        <form onSubmit={registrar} aria-label="Registrar abordagem" className="grid gap-3 sm:grid-cols-2">
          {!cidadaoId && (
            <Input id="abordagem-descricao" label="Pessoa abordada (descrição) *" maxLength={300} value={descricao}
              onChange={(e) => setDescricao(e.target.value)} placeholder="Ex.: homem, uns 40 anos, sob a marquise"
              autoComplete="off" />
          )}
          <Input id="abordagem-local" label="Local *" maxLength={300} value={local} onChange={(e) => setLocal(e.target.value)}
            placeholder="Praça, terminal, logradouro" autoComplete="off" />
          <Input id="abordagem-quando" label="Data e hora *" type="datetime-local" value={quando}
            onChange={(e) => setQuando(e.target.value)} />
          <Select id="abordagem-resultado" label="Resultado *" value={resultado}
            onChange={(e) => setResultado(e.target.value as ResultadoAbordagem)}
            options={Object.entries(RESULTADOS_ABORDAGEM).map(([value, r]) => ({ value, label: r.label }))} />
          <div className="flex flex-wrap gap-4 text-sm sm:col-span-2">
            <label className="flex items-center gap-2">
              <input type="checkbox" checked={drogas} onChange={(e) => setDrogas(e.target.checked)} /> Uso aparente de drogas
            </label>
            <label className="flex items-center gap-2">
              <input type="checkbox" checked={saude} onChange={(e) => setSaude(e.target.checked)} /> Demanda de saúde
            </label>
          </div>
          <Textarea id="abordagem-obs" label="Observações" rows={2} maxLength={2000} value={obs}
            onChange={(e) => setObs(e.target.value)} className="sm:col-span-2" />
          <div>
            <Button type="submit" loading={pending} disabled={!podeRegistrar}>
              Registrar abordagem
            </Button>
          </div>
        </form>

        {!cidadaoId && (
          <>
            <Input id="abordagem-mes" label="Mês" type="month" value={mes} onChange={(e) => setMes(e.target.value)} className="max-w-[200px]" />
            <DataState loading={lista.isLoading} error={lista.error} onRetry={() => void lista.mutate()}
              empty={itens.length === 0} emptyTitle="Nenhuma abordagem neste mês">
              <ul aria-label="Abordagens do mês" className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
                {itens.map((a) => (
                  <li key={a.id} className="flex flex-wrap items-center justify-between gap-3 p-3 text-sm">
                    <div>
                      <p className="font-medium">{a.descricao_pessoa || "Munícipe cadastrado"} · {a.local}</p>
                      <p className="text-xs text-muted">
                        {fmtDateTime(a.data_abordagem)}
                        {a.uso_drogas_aparente && " · uso aparente de drogas"}
                        {a.demanda_saude && " · demanda de saúde"}
                        {a.observacoes && ` · ${a.observacoes}`}
                      </p>
                    </div>
                    <Badge tone={RESULTADOS_ABORDAGEM[a.resultado].tone}>{RESULTADOS_ABORDAGEM[a.resultado].label}</Badge>
                  </li>
                ))}
              </ul>
            </DataState>
          </>
        )}
      </CardContent>
    </Card>
  );
}
