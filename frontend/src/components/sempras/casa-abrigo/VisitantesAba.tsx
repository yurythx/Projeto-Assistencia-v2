"use client";

import { useState, type FormEvent } from "react";
import { LogOut, Trash2 } from "lucide-react";

import { DataState } from "@/components/nexus/DataState";
import { useAction } from "@/components/nexus/useAction";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Table, TableBody, TableHead, TableRow } from "@/components/ui/Table";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import {
  TIPOS_VISITA,
  dataISO,
  fmtDia,
  hojeISO,
  horaAgora,
  type AcolhidoResumo,
  type TipoVisita,
  type Visitante,
} from "@/lib/sempras/types";

import { SeletorAcolhido } from "./campos";

/**
 * Ficha de visitantes e voluntários: a entrada fica registrada com a data e
 * a hora de agora; a saída, quando a pessoa vai embora. Visita a um
 * acolhido exige o acolhido; instituição, doação e voluntariado, não.
 */
export function VisitantesAba({ unidadeId, acolhidos }: { unidadeId: string; acolhidos: AcolhidoResumo[] }) {
  const lista = useApiQuery<Visitante[]>(`v1/sempras/unidades/${unidadeId}/casa-abrigo/visitantes`);
  const [nome, setNome] = useState("");
  const [documento, setDocumento] = useState("");
  const [vinculo, setVinculo] = useState("");
  const [tipo, setTipo] = useState<TipoVisita>("acolhido");
  const [acolhimentoId, setAcolhimentoId] = useState("");
  const [atividade, setAtividade] = useState("");
  const { run, pending } = useAction();
  const { run: runAcao } = useAction();
  const precisaAcolhido = tipo === "acolhido";

  async function registrar(e: FormEvent) {
    e.preventDefault();
    const agora = new Date();
    const ok = await run(
      () => apiClient.post(`v1/sempras/unidades/${unidadeId}/casa-abrigo/visitantes`, {
        tipo_visita: tipo, acolhimento_id: acolhimentoId || null, nome_visitante: nome.trim(), documento_identidade: documento.trim(),
        parentesco_vinculo: vinculo.trim(), data_visita: dataISO(hojeISO(agora)), hora_entrada: horaAgora(agora), observacoes: atividade.trim(),
      }),
      "Entrada registrada",
    );
    if (ok) {
      setNome("");
      setDocumento("");
      setVinculo("");
      setAtividade("");
      void lista.mutate();
    }
  }

  async function saida(v: Visitante) {
    const ok = await runAcao(() => apiClient.post(`v1/sempras/casa-abrigo-visitantes/${v.id}/saida`, { hora_saida: horaAgora() }), "Saída registrada");
    if (ok) void lista.mutate();
  }

  async function remover(v: Visitante) {
    const ok = await runAcao(() => apiClient.delete(`v1/sempras/casa-abrigo-visitantes/${v.id}`), "Visita removida");
    if (ok) void lista.mutate();
  }

  return (
    <div className="flex flex-col gap-6">
      <form onSubmit={registrar} className="flex flex-col gap-3 rounded-xl border border-surface-border p-4">
        <h2 className="text-base font-semibold">Ficha de visitantes e voluntários</h2>
        <div className="grid gap-3 sm:grid-cols-3">
          <Input id="visita-nome" label="Nome do visitante *" value={nome} onChange={(e) => setNome(e.target.value)} autoComplete="off" />
          <Input id="visita-documento" label="CPF / RG" value={documento} onChange={(e) => setDocumento(e.target.value)} autoComplete="off" />
          <Input id="visita-vinculo" label="Vínculo / instituição" placeholder="Ex.: igreja, faculdade, parente…" value={vinculo}
            onChange={(e) => setVinculo(e.target.value)} />
          <Select id="visita-tipo" label="Tipo de visita" value={tipo} onChange={(e) => setTipo(e.target.value as TipoVisita)}
            options={Object.entries(TIPOS_VISITA).map(([value, label]) => ({ value, label }))} />
          <SeletorAcolhido id="visita-acolhido" acolhidos={acolhidos} valor={acolhimentoId} onChange={setAcolhimentoId}
            label={precisaAcolhido ? "Acolhido a ser visitado *" : "Acolhido (opcional)"} vazio={precisaAcolhido ? "Selecione o acolhido…" : "Nenhum"} />
        </div>
        <Textarea id="visita-atividade" label="Atividade desenvolvida" rows={2} value={atividade} onChange={(e) => setAtividade(e.target.value)} />
        <div>
          <Button type="submit" loading={pending} disabled={nome.trim().length < 2 || (precisaAcolhido && !acolhimentoId)}>
            Registrar entrada
          </Button>
        </div>
      </form>

      <section aria-label="Visitas recentes" className="flex flex-col gap-3">
        <h2 className="text-base font-semibold">Histórico de visitas recentes</h2>
        <DataState loading={lista.isLoading} error={lista.error} onRetry={() => void lista.mutate()} empty={(lista.data ?? []).length === 0}
          emptyTitle="Nenhum visitante registrado">
          <Table caption="Visitas recentes">
            <TableHead>
              <TableRow>
                <th className="px-3 py-2">Data / hora</th>
                <th className="px-3 py-2">Visitante</th>
                <th className="px-3 py-2">Acolhido</th>
                <th className="px-3 py-2">Tipo</th>
                <th className="px-3 py-2">Saída</th>
                <th className="px-3 py-2">Ações</th>
              </TableRow>
            </TableHead>
            <TableBody>
              {(lista.data ?? []).map((v) => (
                <TableRow key={v.id}>
                  <td className="px-3 py-2">{fmtDia(v.data_visita)} {v.hora_entrada}</td>
                  <td className="px-3 py-2">
                    {v.nome_visitante}
                    {v.parentesco_vinculo && <span className="text-muted"> · {v.parentesco_vinculo}</span>}
                  </td>
                  <td className="px-3 py-2">{v.acolhido_nome || "—"}</td>
                  <td className="px-3 py-2">{TIPOS_VISITA[v.tipo_visita]}</td>
                  <td className="px-3 py-2">
                    {v.hora_saida || (
                      <Button type="button" size="sm" variant="secondary" onClick={() => void saida(v)}>
                        <LogOut size={14} className="mr-1" aria-hidden="true" /> Registrar saída
                      </Button>
                    )}
                  </td>
                  <td className="px-3 py-2">
                    <Button type="button" size="sm" variant="ghost" aria-label={`Remover visita de ${v.nome_visitante}`} onClick={() => void remover(v)}>
                      <Trash2 size={14} aria-hidden="true" />
                    </Button>
                  </td>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </DataState>
      </section>
    </div>
  );
}
