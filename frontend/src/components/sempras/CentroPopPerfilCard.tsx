"use client";

import { useState, type FormEvent } from "react";

import { useAction } from "@/components/nexus/useAction";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import {
  MOTIVOS_VULNERABILIDADE,
  SITUACOES_RUA,
  type CentroPopPerfil,
  type MotivoVulnerabilidade,
  type SituacaoRua,
} from "@/lib/sempras/types";

/** Perfil de rua do Centro POP — extensão 1:1 do cadastro (ADR 030,
 * Fase 9). Componente próprio pra inicializar o formulário com os
 * valores já salvos sem depender de re-render da página inteira (mesmo
 * padrão do PIA da Casa Abrigo em AcolhimentoItem). */
export function CentroPopPerfilCard({ cidadaoId, unidadeId, perfil, onChanged }: {
  cidadaoId: string; unidadeId: string; perfil: CentroPopPerfil | null; onChanged: () => void;
}) {
  const [situacao, setSituacao] = useState<SituacaoRua>(perfil?.situacao ?? "situacao_rua");
  const [tempoRuaAnos, setTempoRuaAnos] = useState(perfil?.tempo_rua_anos != null ? String(perfil.tempo_rua_anos) : "");
  const [motivo, setMotivo] = useState<MotivoVulnerabilidade>(perfil?.motivo_vulnerabilidade ?? "outro");
  const [possuiDoenca, setPossuiDoenca] = useState(perfil?.possui_doenca ?? false);
  const [doencaDescricao, setDoencaDescricao] = useState(perfil?.doenca_descricao ?? "");
  const [usaMedicacao, setUsaMedicacao] = useState(perfil?.usa_medicacao ?? false);
  const [medicacaoDescricao, setMedicacaoDescricao] = useState(perfil?.medicacao_descricao ?? "");
  const [usoDrogas, setUsoDrogas] = useState(perfil?.uso_drogas ?? false);
  const [drogasDescricao, setDrogasDescricao] = useState(perfil?.drogas_descricao ?? "");
  const { run, pending } = useAction();

  async function salvar(e: FormEvent) {
    e.preventDefault();
    const ok = await run(
      () => apiClient.put(`v1/sempras/cidadaos/${cidadaoId}/centro-pop-perfil`, {
        unidade_id: unidadeId, situacao, tempo_rua_anos: tempoRuaAnos ? Number(tempoRuaAnos) : null,
        motivo_vulnerabilidade: motivo, possui_doenca: possuiDoenca, doenca_descricao: doencaDescricao.trim(),
        usa_medicacao: usaMedicacao, medicacao_descricao: medicacaoDescricao.trim(),
        uso_drogas: usoDrogas, drogas_descricao: drogasDescricao.trim(),
      }),
      "Perfil de rua atualizado",
    );
    if (ok) onChanged();
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle as="h2">Perfil de rua (Centro POP)</CardTitle>
      </CardHeader>
      <CardContent className="pb-5">
        <form onSubmit={salvar} className="mt-3 flex flex-col gap-3">
          <div className="grid gap-3 sm:grid-cols-2">
            <Select id="perfil-rua-situacao" label="Situação" value={situacao} onChange={(e) => setSituacao(e.target.value as SituacaoRua)}
              options={Object.entries(SITUACOES_RUA).map(([value, label]) => ({ value, label }))} />
            <Input id="perfil-rua-tempo" label="Tempo de rua (anos)" type="number" min={0} step="0.5" value={tempoRuaAnos}
              onChange={(e) => setTempoRuaAnos(e.target.value)} />
          </div>
          <Select id="perfil-rua-motivo" label="Motivo da vulnerabilidade" value={motivo} onChange={(e) => setMotivo(e.target.value as MotivoVulnerabilidade)}
            options={Object.entries(MOTIVOS_VULNERABILIDADE).map(([value, label]) => ({ value, label }))} />

          <fieldset className="rounded-lg border border-surface-border p-3">
            <legend className="px-1 text-xs font-medium text-muted">Saúde e uso de substâncias</legend>
            <div className="flex flex-col gap-3">
              <label className="flex items-center gap-2 text-sm">
                <input type="checkbox" checked={possuiDoenca} onChange={(e) => setPossuiDoenca(e.target.checked)} /> Possui doença
              </label>
              {possuiDoenca && (
                <Textarea id="perfil-rua-doenca" label="Descrição da doença" rows={2} maxLength={1000}
                  value={doencaDescricao} onChange={(e) => setDoencaDescricao(e.target.value)} />
              )}
              <label className="flex items-center gap-2 text-sm">
                <input type="checkbox" checked={usaMedicacao} onChange={(e) => setUsaMedicacao(e.target.checked)} /> Usa medicação
              </label>
              {usaMedicacao && (
                <Textarea id="perfil-rua-medicacao" label="Descrição da medicação" rows={2} maxLength={1000}
                  value={medicacaoDescricao} onChange={(e) => setMedicacaoDescricao(e.target.value)} />
              )}
              <label className="flex items-center gap-2 text-sm">
                <input type="checkbox" checked={usoDrogas} onChange={(e) => setUsoDrogas(e.target.checked)} /> Uso de drogas
              </label>
              {usoDrogas && (
                <Textarea id="perfil-rua-drogas" label="Descrição do uso de drogas" rows={2} maxLength={1000}
                  value={drogasDescricao} onChange={(e) => setDrogasDescricao(e.target.value)} />
              )}
            </div>
          </fieldset>

          <div>
            <Button type="submit" loading={pending} disabled={!unidadeId}>
              Salvar perfil de rua
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
