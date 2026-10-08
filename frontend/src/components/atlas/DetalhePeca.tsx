"use client";

import { Download, ExternalLink, Upload } from "lucide-react";
import Link from "next/link";
import { useId, useState } from "react";

import { useAction } from "@/components/nexus/useAction";
import { Button, buttonClass } from "@/components/ui/Button";
import { Select } from "@/components/ui/Select";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import { FORMATOS_MODELO, MAX_MODELO_BYTES, extensao, urlArquivoModelo } from "@/lib/atlas/modelos";
import type { EtapaDocumento, ModeloDocumento, UUID, Workflow } from "@/lib/nexus/types";

/** Modelos ativos da série da TTDD do procedimento: valem para a peça
 * que não tem modelo próprio. */
export interface ModelosSerie {
  codigo: string;
  modelos: ModeloDocumento[];
}

/** Gestão do modelo da peça (atlas:manage): o procedimento em vigor e o que
 * fazer depois de alterar. */
export interface GestaoPecas {
  workflowId: UUID;
  onAtualizado: (wf: Workflow) => void;
}

/** Ligar a peça a outro modelo da biblioteca, ou enviar um arquivo: nova
 * versão do modelo ligado, ou um modelo novo com o nome da peça (ADR 024). */
function GestaoModelo({ peca, gestao }: { peca: EtapaDocumento; gestao: GestaoPecas }) {
  const id = useId();
  const { run, pending } = useAction();
  const lista = useApiQuery<ModeloDocumento[]>("v1/atlas/admin/modelos");
  const modelos = (lista.data ?? []).filter((m) => m.ativo || m.id === peca.modelo_id);
  const [escolha, setEscolha] = useState(peca.modelo_id ?? "");
  const [arquivo, setArquivo] = useState<File | null>(null);
  const [erro, setErro] = useState("");
  const rota = `v1/atlas/admin/workflows/${gestao.workflowId}/pecas/${peca.id}/modelo`;

  async function ligar(modeloId: string | null, sucesso: string) {
    const res = await run(() => apiClient.put<Workflow>(rota, { modelo_id: modeloId }), sucesso);
    if (res) gestao.onAtualizado(res.data);
  }

  async function enviar() {
    if (!arquivo) return;
    const form = new FormData();
    form.set("arquivo", arquivo);
    if (peca.modelo) {
      form.set("nota", `Enviado pela peça "${peca.nome_documento}"`);
      const ok = await run(
        () => apiClient.postForm(`v1/atlas/admin/modelos/${peca.modelo!.id}/versoes`, form),
        "Nova versão do modelo publicada",
      );
      if (!ok) return;
      // Recarrega o procedimento para mostrar a versão nova.
      await ligar(peca.modelo.id, "");
    } else {
      form.set("nome", peca.nome_documento);
      const novo = await run(() =>
        apiClient.postForm<ModeloDocumento>("v1/atlas/admin/modelos", form),
      );
      if (!novo) return;
      await ligar(novo.data.id, "Modelo cadastrado e ligado à peça");
    }
    setArquivo(null);
  }

  return (
    <div className="flex flex-col gap-4 border-t border-surface-border pt-3">
      <div className="flex flex-wrap items-end gap-2">
        <div className="min-w-56 flex-1">
          <Select
            id={`${id}-modelo`}
            label="Modelo da biblioteca"
            value={escolha}
            onChange={(e) => setEscolha(e.target.value)}
            options={[
              { value: "", label: "Sem modelo" },
              ...modelos.map((m) => ({
                value: m.id,
                label: m.ativo ? m.nome : `${m.nome} (desativado)`,
              })),
            ]}
          />
        </div>
        <Button
          variant="secondary"
          size="sm"
          loading={pending}
          disabled={escolha === (peca.modelo_id ?? "")}
          onClick={() =>
            void ligar(
              escolha || null,
              escolha ? "Modelo ligado à peça" : "Modelo retirado da peça",
            )
          }
        >
          Salvar
        </Button>
      </div>

      <div className="flex flex-col gap-1">
        <label htmlFor={`${id}-arquivo`} className="text-sm font-medium text-foreground">
          {peca.modelo ? "Enviar nova versão do modelo" : "Enviar arquivo de modelo para esta peça"}
        </label>
        <div className="flex flex-wrap items-center gap-2">
          <input
            id={`${id}-arquivo`}
            type="file"
            accept={FORMATOS_MODELO}
            aria-describedby={`${id}-ajuda`}
            onChange={(e) => {
              const f = e.target.files?.[0] ?? null;
              const grande = !!f && f.size > MAX_MODELO_BYTES;
              setErro(grande ? "O arquivo passa de 10 MB." : "");
              setArquivo(grande ? null : f);
            }}
            className="text-sm"
          />
          <Button size="sm" loading={pending} disabled={!arquivo} onClick={() => void enviar()}>
            <Upload size={14} aria-hidden="true" /> Enviar
          </Button>
        </div>
        <p id={`${id}-ajuda`} className="text-xs text-muted">
          {peca.modelo
            ? `A nova versão de "${peca.modelo.nome}" vale para todos os procedimentos que usam este modelo.`
            : `Cria o modelo "${peca.nome_documento}" na biblioteca e liga a esta peça.`}{" "}
          Word, LibreOffice, PDF, RTF ou planilha — até 10 MB.
        </p>
        {erro && (
          <p role="alert" className="text-xs text-danger">
            {erro}
          </p>
        )}
      </div>
    </div>
  );
}

/** Detalhe da peça: o modelo para baixar (todos) e, para a gestão, ligar,
 * trocar ou enviar o modelo — sem publicar nova versão do procedimento. */
export function DetalhePeca({
  peca,
  gestao,
  serie,
}: {
  peca: EtapaDocumento;
  gestao?: GestaoPecas;
  serie?: ModelosSerie;
}) {
  return (
    <div className="flex flex-col gap-3 text-sm">
      {peca.modelo ? (
        <div className="flex flex-wrap items-center justify-between gap-2 rounded-md bg-primary/5 p-3">
          <span>
            <span className="block font-medium text-foreground">Modelo: {peca.modelo.nome}</span>
            <span className="block text-xs text-muted">
              Versão {peca.modelo.versao} · {extensao(peca.modelo.arquivo_nome)} ·{" "}
              {peca.modelo.arquivo_nome} ·{" "}
              <Link href="/atlas/modelos" className="text-primary hover:underline">
                versões anteriores na biblioteca
              </Link>
            </span>
          </span>
          <a
            href={urlArquivoModelo(peca.modelo.id)}
            download={peca.modelo.arquivo_nome}
            aria-label={`Baixar o modelo ${peca.modelo.nome}`}
            className={buttonClass("primary", "sm")}
          >
            <Download size={14} aria-hidden="true" /> Baixar o modelo
          </a>
        </div>
      ) : peca.modelo_minuta_padrao_url ? (
        <a
          href={peca.modelo_minuta_padrao_url}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1 text-primary hover:underline"
        >
          Modelo externo <ExternalLink size={12} aria-hidden="true" />
          <span className="sr-only">(abre em nova janela)</span>
        </a>
      ) : serie && serie.modelos.length > 0 ? (
        <div className="flex flex-col gap-2 rounded-md bg-primary/5 p-3">
          <span className="text-xs text-muted">
            Modelos da série{" "}
            <Link
              href={`/atlas/ttdd/${encodeURIComponent(serie.codigo)}`}
              className="font-mono text-primary hover:underline"
            >
              {serie.codigo}
            </Link>{" "}
            do procedimento:
          </span>
          <ul className="flex flex-col gap-2">
            {serie.modelos.map((m) => (
              <li key={m.id} className="flex flex-wrap items-center justify-between gap-2">
                <span>
                  <span className="block font-medium text-foreground">Modelo: {m.nome}</span>
                  <span className="block text-xs text-muted">
                    Versão {m.atual.versao} · {extensao(m.atual.arquivo_nome)} ·{" "}
                    {m.atual.arquivo_nome}
                  </span>
                </span>
                <a
                  href={urlArquivoModelo(m.id)}
                  download={m.atual.arquivo_nome}
                  aria-label={`Baixar o modelo ${m.nome}`}
                  className={buttonClass("primary", "sm")}
                >
                  <Download size={14} aria-hidden="true" /> Baixar o modelo
                </a>
              </li>
            ))}
          </ul>
        </div>
      ) : (
        <p className="text-muted">Nenhum modelo cadastrado para esta peça.</p>
      )}
      {gestao && <GestaoModelo key={peca.modelo_id ?? "nenhum"} peca={peca} gestao={gestao} />}
    </div>
  );
}
