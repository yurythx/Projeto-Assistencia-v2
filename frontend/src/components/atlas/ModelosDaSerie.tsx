"use client";

import { Download, FileText, Link2, Upload, X } from "lucide-react";
import Link from "next/link";
import { useId, useState } from "react";

import { DataState } from "@/components/nexus/DataState";
import { fmtBytes, useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button, buttonClass } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import { FORMATOS_MODELO, MAX_MODELO_BYTES, extensao, urlArquivoModelo } from "@/lib/atlas/modelos";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { ModeloDocumento } from "@/lib/nexus/types";

/** Gestão (atlas:manage): ligar um modelo da biblioteca à série ou enviar
 * um arquivo, que vira um modelo novo já ligado. */
function GestaoModelosSerie({
  codigo,
  descritor,
  ligados,
  onAtualizado,
}: {
  codigo: string;
  descritor: string;
  ligados: ModeloDocumento[];
  onAtualizado: (lista: ModeloDocumento[]) => void;
}) {
  const id = useId();
  const { run, pending } = useAction();
  const biblioteca = useApiQuery<ModeloDocumento[]>("v1/atlas/admin/modelos");
  const livres = (biblioteca.data ?? []).filter(
    (m) => m.ativo && !ligados.some((l) => l.id === m.id),
  );
  const [escolha, setEscolha] = useState("");
  const [nome, setNome] = useState(descritor.slice(0, 150));
  const [arquivo, setArquivo] = useState<File | null>(null);
  const [erro, setErro] = useState("");
  const rota = `v1/atlas/admin/ttdd/${encodeURIComponent(codigo)}/modelos`;

  async function ligar(modeloId: string, sucesso: string) {
    const res = await run(
      () => apiClient.post<ModeloDocumento[]>(rota, { modelo_id: modeloId }),
      sucesso,
    );
    if (res) onAtualizado(res.data);
    return !!res;
  }

  async function enviar() {
    if (!arquivo) return;
    const form = new FormData();
    form.set("nome", nome);
    form.set("arquivo", arquivo);
    const novo = await run(() =>
      apiClient.postForm<ModeloDocumento>("v1/atlas/admin/modelos", form),
    );
    if (novo && (await ligar(novo.data.id, "Modelo cadastrado e ligado à série"))) {
      setArquivo(null);
      void biblioteca.mutate();
    }
  }

  return (
    <div className="mt-4 grid gap-4 border-t border-surface-border pt-4 md:grid-cols-2">
      <div className="flex flex-col gap-2">
        <Select
          id={`${id}-modelo`}
          label="Ligar um modelo da biblioteca"
          value={escolha}
          onChange={(e) => setEscolha(e.target.value)}
          options={[
            { value: "", label: livres.length ? "Escolha o modelo" : "Nenhum modelo disponível" },
            ...livres.map((m) => ({ value: m.id, label: m.nome })),
          ]}
        />
        <div>
          <Button
            variant="secondary"
            size="sm"
            loading={pending}
            disabled={!escolha}
            onClick={() =>
              void ligar(escolha, "Modelo ligado à série").then((ok) => ok && setEscolha(""))
            }
          >
            <Link2 size={14} aria-hidden="true" /> Ligar
          </Button>
        </div>
      </div>
      <div className="flex flex-col gap-2">
        <Input
          id={`${id}-nome`}
          label="Ou enviar um modelo novo — nome"
          value={nome}
          maxLength={150}
          onChange={(e) => setNome(e.target.value)}
        />
        <label htmlFor={`${id}-arquivo`} className="text-sm font-medium text-foreground">
          Arquivo do modelo
        </label>
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
        <p id={`${id}-ajuda`} className="text-xs text-muted">
          Entra na biblioteca de modelos e fica ligado a esta série. Word, LibreOffice, PDF, RTF ou
          planilha — até 10 MB.
        </p>
        {erro && (
          <p role="alert" className="text-xs text-danger">
            {erro}
          </p>
        )}
        <div>
          <Button
            size="sm"
            loading={pending}
            disabled={!arquivo || !nome.trim()}
            onClick={() => void enviar()}
          >
            <Upload size={14} aria-hidden="true" /> Enviar
          </Button>
        </div>
      </div>
    </div>
  );
}

/** Modelos de documento da série (ADR 024): para baixar e preencher; a
 * gestão liga modelos da biblioteca ou envia novos. */
export function ModelosDaSerie({
  codigo,
  descritor,
  vigente,
}: {
  codigo: string;
  descritor: string;
  vigente: boolean;
}) {
  const { can } = useNexus();
  const { run, pending } = useAction();
  const gestao = can("atlas:manage") && vigente;
  const lista = useApiQuery<ModeloDocumento[]>(
    `v1/atlas/ttdd/${encodeURIComponent(codigo)}/modelos`,
  );
  const itens = lista.data ?? [];
  const atualizar = (novos: ModeloDocumento[]) => void lista.mutate(novos, { revalidate: false });

  async function retirar(m: ModeloDocumento) {
    const res = await run(
      () =>
        apiClient.delete<ModeloDocumento[]>(
          `v1/atlas/admin/ttdd/${encodeURIComponent(codigo)}/modelos/${m.id}`,
        ),
      "Modelo retirado da série",
    );
    if (res) atualizar(res.data);
  }

  return (
    <section aria-labelledby="atlas-serie-modelos">
      <h2 id="atlas-serie-modelos" className="mb-3 text-lg font-bold text-foreground">
        Modelos de documento
      </h2>
      <DataState
        loading={lista.isLoading}
        error={lista.error}
        onRetry={() => void lista.mutate()}
        empty={false}
      >
        {itens.length === 0 ? (
          <p className="text-sm text-muted">
            Nenhum modelo cadastrado para esta série.{" "}
            <Link href="/atlas/modelos" className="text-primary hover:underline">
              Ver a biblioteca de modelos
            </Link>
          </p>
        ) : (
          <ul className="grid gap-2 md:grid-cols-2" aria-label="Modelos da série">
            {itens.map((m) => (
              <li
                key={m.id}
                className="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-surface-border p-3 text-sm"
              >
                <span className="flex min-w-0 items-start gap-2">
                  <FileText size={18} aria-hidden="true" className="mt-0.5 shrink-0 text-primary" />
                  <span className="min-w-0">
                    <span className="block font-medium text-foreground">{m.nome}</span>
                    <span className="block text-xs text-muted">
                      {extensao(m.atual.arquivo_nome)} · versão {m.atual.versao} ·{" "}
                      {fmtBytes(m.atual.tamanho)}
                    </span>
                    {m.descricao && <span className="block text-xs text-muted">{m.descricao}</span>}
                    {!m.ativo && <Badge tone="warning">Desativado</Badge>}
                  </span>
                </span>
                <span className="flex items-center gap-1">
                  <a
                    href={urlArquivoModelo(m.id)}
                    download={m.atual.arquivo_nome}
                    aria-label={`Baixar o modelo ${m.nome}`}
                    className={buttonClass("primary", "sm")}
                  >
                    <Download size={14} aria-hidden="true" /> Baixar
                  </a>
                  {gestao && (
                    <Button
                      variant="ghost"
                      size="sm"
                      loading={pending}
                      aria-label={`Retirar o modelo ${m.nome} da série`}
                      onClick={() => void retirar(m)}
                    >
                      <X size={14} aria-hidden="true" />
                    </Button>
                  )}
                </span>
              </li>
            ))}
          </ul>
        )}
        {gestao && (
          <GestaoModelosSerie
            codigo={codigo}
            descritor={descritor}
            ligados={itens}
            onAtualizado={atualizar}
          />
        )}
      </DataState>
    </section>
  );
}
