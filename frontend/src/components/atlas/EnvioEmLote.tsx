"use client";

import { CheckCircle2, Upload, XCircle } from "lucide-react";
import { useId, useState } from "react";

import { Button } from "@/components/ui/Button";
import { ApiError, apiClient } from "@/lib/api/client";
import { FORMATOS_MODELO, MAX_MODELO_BYTES, nomeDoArquivo } from "@/lib/atlas/modelos";
import type { ModeloDocumento } from "@/lib/nexus/types";

interface Resultado {
  arquivo: string;
  nome: string;
  serie?: string;
  erro?: string;
}

const motivo = (e: unknown) => (e instanceof ApiError ? e.message : "falha ao enviar");

/** Envio de vários modelos de uma vez (ADR 025): o nome vem do arquivo; um
 * código de série no início liga o modelo à série. Cada arquivo é tratado
 * sozinho — um erro não impede os demais. */
export function EnvioEmLote({ onDone }: { onDone: () => void }) {
  const id = useId();
  const [arquivos, setArquivos] = useState<File[]>([]);
  const [resultados, setResultados] = useState<Resultado[]>([]);
  const [enviando, setEnviando] = useState(false);

  async function enviar() {
    setEnviando(true);
    const out: Resultado[] = [];
    for (const f of arquivos) {
      const { nome, serie } = nomeDoArquivo(f.name);
      const r: Resultado = { arquivo: f.name, nome, serie };
      if (f.size > MAX_MODELO_BYTES) {
        r.erro = "o arquivo passa de 10 MB";
      } else {
        try {
          const form = new FormData();
          form.set("nome", nome);
          form.set("arquivo", f);
          const novo = await apiClient.postForm<ModeloDocumento>("v1/atlas/admin/modelos", form);
          if (serie) {
            try {
              await apiClient.post(`v1/atlas/admin/ttdd/${encodeURIComponent(serie)}/modelos`, {
                modelo_id: novo.data.id,
              });
            } catch (e) {
              r.erro = `cadastrado, mas não ligado à série ${serie}: ${motivo(e)}`;
            }
          }
        } catch (e) {
          r.erro = motivo(e);
        }
      }
      out.push(r);
      setResultados([...out]);
    }
    setEnviando(false);
    setArquivos([]);
    onDone();
  }

  return (
    <div className="flex flex-col gap-4 text-sm">
      <p className="text-muted">
        O nome de cada modelo vem do nome do arquivo. Comece o nome pelo código da série para já
        ligar o modelo a ela — por exemplo, <code>2.0.02.00.07 - Edital de pregão.docx</code>.
      </p>
      <div className="flex flex-col gap-1">
        <label htmlFor={`${id}-arquivos`} className="font-medium text-foreground">
          Arquivos dos modelos
        </label>
        <input
          id={`${id}-arquivos`}
          type="file"
          multiple
          accept={FORMATOS_MODELO}
          onChange={(e) => {
            setResultados([]);
            setArquivos(Array.from(e.target.files ?? []));
          }}
          className="text-sm"
        />
      </div>
      {arquivos.length > 0 && (
        <ul className="flex flex-col gap-0.5 text-xs text-muted" aria-label="Arquivos escolhidos">
          {arquivos.map((f) => {
            const { nome, serie } = nomeDoArquivo(f.name);
            return (
              <li key={f.name}>
                {nome}
                {serie && ` → série ${serie}`}
              </li>
            );
          })}
        </ul>
      )}
      <div>
        <Button onClick={() => void enviar()} disabled={arquivos.length === 0} loading={enviando}>
          <Upload size={14} aria-hidden="true" /> Enviar {arquivos.length || ""}{" "}
          {arquivos.length === 1 ? "arquivo" : "arquivos"}
        </Button>
      </div>
      {resultados.length > 0 && (
        <ul className="flex flex-col gap-1" aria-label="Resultado do envio">
          {resultados.map((r) => (
            <li key={r.arquivo} className="flex items-start gap-2">
              {r.erro ? (
                <XCircle size={14} aria-hidden="true" className="mt-0.5 shrink-0 text-danger" />
              ) : (
                <CheckCircle2
                  size={14}
                  aria-hidden="true"
                  className="mt-0.5 shrink-0 text-success"
                />
              )}
              <span>
                <span className="font-medium text-foreground">{r.nome}</span>
                {r.serie && !r.erro && (
                  <span className="text-muted"> — ligado à série {r.serie}</span>
                )}
                {r.erro && <span className="block text-xs text-danger">{r.erro}</span>}
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
