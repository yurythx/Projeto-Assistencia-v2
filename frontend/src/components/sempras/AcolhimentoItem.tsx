"use client";

import Link from "next/link";
import { useRef, useState, type FormEvent } from "react";

import { useAction } from "@/components/nexus/useAction";
import { abrirDocumento, enviarDocumento } from "@/components/sempras/casa-abrigo/documentos";
import { Badge } from "@/components/ui/Badge";
import { Button, buttonClass } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import {
  CATEGORIAS_DIAGNOSTICO,
  MOTIVOS_ACOLHIMENTO,
  SITUACOES_JURIDICAS,
  STATUS_ACOLHIMENTO,
  TIPOS_DESLIGAMENTO,
  TIPOS_DOCUMENTO_CASA_ABRIGO,
  dataISO,
  diaDe,
  fmtDia,
  hojeISO,
  type Acolhimento,
  type TipoDesligamento,
  type TipoDocumentoCasaAbrigo,
} from "@/lib/sempras/types";

/** A próxima reavaliação do PIA (R7): a última gravação, ou a entrada, + 90 dias. */
export function proximaReavaliacao(a: Pick<Acolhimento, "data_entrada" | "pia">): string {
  const base = new Date(`${diaDe(a.pia?.atualizado_em ?? a.data_entrada)}T00:00:00`);
  base.setDate(base.getDate() + 90);
  return hojeISO(base);
}

/**
 * Um acolhimento da Casa Abrigo na ficha do munícipe: o resumo do cadastro,
 * o prazo do PIA, os documentos e o desligamento (ação própria da equipe
 * técnica). O cadastro completo, o PIA, os termos e as visitas ficam no
 * espaço da Casa Abrigo (ADR 032 §3) — os atalhos levam direto a eles.
 */
export function AcolhimentoItem({ acolhimento, nomeUnidade, onChanged }: { acolhimento: Acolhimento; nomeUnidade: string; onChanged: () => void }) {
  const a = acolhimento;
  const espaco = `/sempras/u/${a.unidade_id}`;
  const [desligando, setDesligando] = useState(false);
  const [tipo, setTipo] = useState<TipoDesligamento>("reintegracao_familia_natural");
  const [motivo, setMotivo] = useState("");
  const [dataSaida, setDataSaida] = useState(hojeISO());
  const [guia, setGuia] = useState("");
  const { run: runDesligar, pending: pendingDesligar } = useAction();

  const [tipoDocumento, setTipoDocumento] = useState<TipoDocumentoCasaAbrigo>("acolhido");
  const [enviando, setEnviando] = useState(false);
  const { run: runDocumento } = useAction();
  const fileInput = useRef<HTMLInputElement>(null);

  async function confirmarDesligamento(e: FormEvent) {
    e.preventDefault();
    const ok = await runDesligar(
      () => apiClient.post(`v1/sempras/casa-abrigo/${a.id}/desligar`, { tipo, motivo: motivo.trim(), data_saida: dataISO(dataSaida), guia: guia.trim() }),
      "Acolhimento desligado",
    );
    if (ok) {
      setDesligando(false);
      setMotivo("");
      onChanged();
    }
  }

  async function enviar(file: File) {
    setEnviando(true);
    const ok = await runDocumento(async () => {
      await enviarDocumento(a.id, tipoDocumento, file);
      return true;
    }, "Documento enviado");
    setEnviando(false);
    if (ok) onChanged();
  }

  async function remover(id: string) {
    const ok = await runDocumento(() => apiClient.delete(`v1/sempras/casa-abrigo-documentos/${id}`), "Documento removido");
    if (ok) onChanged();
  }

  const reavaliacao = proximaReavaliacao(a);
  const vencido = a.status === "acolhido" && reavaliacao < hojeISO();

  return (
    <li className="flex flex-col gap-3 p-3 text-sm">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex flex-col gap-1">
          <p className="font-medium">{a.motivos.map((m) => (m === "outro" ? a.outro_motivo || "Outro" : MOTIVOS_ACOLHIMENTO[m])).join(", ")}</p>
          <p className="text-xs text-muted">
            {nomeUnidade} · na unidade desde {fmtDia(a.data_entrada)}
            {a.situacao_juridica && ` · ${SITUACOES_JURIDICAS[a.situacao_juridica]}`}
          </p>
          <p className="text-xs text-muted">
            Processo: {a.processo_judicial || "N/I"} · Guia: {a.guia_acolhimento || "N/I"}
            {a.conselheiro_nome && ` · Conselheiro(a): ${a.conselheiro_nome}`}
          </p>
          {a.status === "desligado" ? (
            <p className="text-xs text-muted">
              Desligado em {fmtDia(a.data_desligamento)}
              {a.desligamento_tipo && ` · ${TIPOS_DESLIGAMENTO[a.desligamento_tipo]}`}
              {a.motivo_desligamento && ` · ${a.motivo_desligamento}`}
            </p>
          ) : (
            <p className={`text-xs ${vencido ? "font-semibold text-danger" : "text-muted"}`}>
              {a.pia ? "Próxima reavaliação do P.I.A." : "P.I.A. a elaborar até"} {fmtDia(reavaliacao)}
              {vencido && " (atrasada)"}
            </p>
          )}
        </div>
        <span className="flex flex-wrap items-center gap-2">
          <Badge tone={STATUS_ACOLHIMENTO[a.status].tone}>{STATUS_ACOLHIMENTO[a.status].label}</Badge>
          {a.status === "acolhido" && (
            <Button size="sm" variant="danger" onClick={() => setDesligando(true)}>
              Desligar
            </Button>
          )}
        </span>
      </div>

      {a.diagnosticos.length > 0 && (
        <p className="text-xs">
          <span className="text-muted">Saúde: </span>
          {a.diagnosticos.map((d) => `${d.diagnostico} (${CATEGORIAS_DIAGNOSTICO[d.categoria].toLowerCase()})`).join(", ")}
        </p>
      )}
      {a.irmaos.length > 0 && (
        <p className="text-xs">
          <span className="text-muted">Irmãos: </span>
          {a.irmaos.map((i) => `${i.nome}${i.esta_acolhido ? " (acolhido)" : ""}`).join(", ")}
        </p>
      )}
      <p className="text-xs text-muted">
        {a.visitantes.length} visita(s) · {a.termos.length} termo(s) · {a.pia_metas.length} ação(ões) no P.I.A.
      </p>

      <nav aria-label="Atalhos do acolhimento" className="flex flex-wrap gap-2">
        <Link href={`${espaco}?aba=acolhidos`} className={buttonClass("secondary", "sm")}>
          Cadastro completo
        </Link>
        <Link href={`${espaco}?aba=pia&acolhido=${a.id}`} className={buttonClass("secondary", "sm")}>
          P.I.A.
        </Link>
        <Link href={`${espaco}?aba=termos`} className={buttonClass("secondary", "sm")}>
          Termos
        </Link>
        <Link href={`${espaco}?aba=visitantes`} className={buttonClass("secondary", "sm")}>
          Visitantes
        </Link>
      </nav>

      <div className="rounded-lg border border-surface-border p-3">
        <h3 className="text-xs font-medium text-muted">Documentos (PDF)</h3>
        {a.documentos.length > 0 && (
          <ul aria-label={`Documentos do acolhimento de ${fmtDia(a.data_entrada)}`} className="mt-2 flex flex-col gap-1">
            {a.documentos.map((doc) => (
              <li key={doc.id} className="flex items-center justify-between gap-2">
                <span>
                  {doc.nome_original} · {TIPOS_DOCUMENTO_CASA_ABRIGO[doc.tipo]}
                  {doc.status === "pending" && <Badge tone="warning">Enviando</Badge>}
                </span>
                <span className="flex gap-1">
                  {doc.status === "ready" && (
                    <Button size="sm" variant="ghost" aria-label={`Baixar documento ${doc.nome_original}`}
                      onClick={() => void runDocumento(() => abrirDocumento(doc.id))}>
                      Baixar
                    </Button>
                  )}
                  <Button size="sm" variant="ghost" aria-label={`Remover documento ${doc.nome_original}`} onClick={() => void remover(doc.id)}>
                    Remover
                  </Button>
                </span>
              </li>
            ))}
          </ul>
        )}
        <div className="mt-2 grid gap-2 sm:grid-cols-[1fr_auto] sm:items-end">
          <Select id={`documento-tipo-${a.id}`} label="Tipo de documento" value={tipoDocumento}
            onChange={(e) => setTipoDocumento(e.target.value as TipoDocumentoCasaAbrigo)}
            options={Object.entries(TIPOS_DOCUMENTO_CASA_ABRIGO).map(([value, label]) => ({ value, label }))} />
          <input ref={fileInput} type="file" accept="application/pdf" className="sr-only" id={`documento-arquivo-${a.id}`}
            onChange={(e) => { const f = e.target.files?.[0]; if (f) void enviar(f); e.target.value = ""; }} />
          <Button size="sm" loading={enviando} onClick={() => fileInput.current?.click()}>
            {enviando ? "Enviando…" : "Enviar PDF"}
          </Button>
        </div>
      </div>

      <Dialog open={desligando} onClose={() => setDesligando(false)} title="Desligar acolhimento" description="Restrito à equipe técnica.">
        <form onSubmit={confirmarDesligamento} className="flex flex-col gap-3">
          <Select id={`desligamento-tipo-${a.id}`} label="Tipo de desligamento" value={tipo} onChange={(e) => setTipo(e.target.value as TipoDesligamento)}
            options={Object.entries(TIPOS_DESLIGAMENTO).map(([value, label]) => ({ value, label }))} />
          <Textarea id={`desligamento-motivo-${a.id}`} label="Motivo do desligamento *" rows={3} maxLength={2000}
            value={motivo} onChange={(e) => setMotivo(e.target.value)} required />
          <div className="grid gap-3 sm:grid-cols-2">
            <Input id={`desligamento-data-${a.id}`} label="Data da saída *" type="date" max={hojeISO()} value={dataSaida}
              onChange={(e) => setDataSaida(e.target.value)} required />
            <Input id={`desligamento-guia-${a.id}`} label="Nº da guia de desligamento" value={guia} onChange={(e) => setGuia(e.target.value)} />
          </div>
          <div>
            <Button type="submit" variant="danger" loading={pendingDesligar} disabled={motivo.trim().length < 3 || !dataSaida}>
              Confirmar
            </Button>
          </div>
        </form>
      </Dialog>
    </li>
  );
}
