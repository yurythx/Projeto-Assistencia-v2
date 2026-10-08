"use client";

import { Download, FileText, History, Pencil, Plus, Upload, Eye } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { Suspense, useId, useMemo, useState } from "react";

import { EnvioEmLote } from "@/components/atlas/EnvioEmLote";
import { Trilha } from "@/components/atlas/Trilha";
import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { fmtBytes, fmtDate, useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button, buttonClass } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Textarea } from "@/components/ui/Textarea";
import { Toggle } from "@/components/ui/Toggle";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import { FORMATOS_MODELO, MAX_MODELO_BYTES, extensao, urlArquivoModelo } from "@/lib/atlas/modelos";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { ModeloDocumento } from "@/lib/nexus/types";

type Edicao =
  | { tipo: "novo" }
  | { tipo: "lote" }
  | { tipo: "versao"; modelo: ModeloDocumento }
  | { tipo: "editar"; modelo: ModeloDocumento };

/** Campo de arquivo com o limite e os formatos da biblioteca. */
function CampoArquivo({ onChange }: { onChange: (f: File | null) => void }) {
  const id = useId();
  const [erro, setErro] = useState("");
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={id} className="text-sm font-medium text-foreground">
        Arquivo do modelo
      </label>
      <input
        id={id}
        type="file"
        accept={FORMATOS_MODELO}
        aria-describedby={`${id}-ajuda`}
        onChange={(e) => {
          const f = e.target.files?.[0] ?? null;
          const grande = !!f && f.size > MAX_MODELO_BYTES;
          setErro(grande ? "O arquivo passa de 10 MB." : "");
          onChange(grande ? null : f);
        }}
        className="text-sm"
      />
      <p id={`${id}-ajuda`} className="text-xs text-muted">
        Word, LibreOffice, PDF, RTF ou planilha — até 10 MB.
      </p>
      {erro && (
        <p role="alert" className="text-xs text-danger">
          {erro}
        </p>
      )}
    </div>
  );
}

/** Cadastro, nova versão ou edição de um modelo (atlas:manage). */
function FormModelo({
  edicao,
  onDone,
}: {
  edicao: Exclude<Edicao, { tipo: "lote" }>;
  onDone: () => void;
}) {
  const { run, pending } = useAction();
  const atual = edicao.tipo === "novo" ? undefined : edicao.modelo;
  const [nome, setNome] = useState(atual?.nome ?? "");
  const [descricao, setDescricao] = useState(atual?.descricao ?? "");
  const [ativo, setAtivo] = useState(atual?.ativo ?? true);
  const [arquivo, setArquivo] = useState<File | null>(null);
  const [nota, setNota] = useState("");

  async function salvar(e: React.FormEvent) {
    e.preventDefault();
    if (edicao.tipo === "editar") {
      const ok = await run(
        () =>
          apiClient.put(`v1/atlas/admin/modelos/${edicao.modelo.id}`, { nome, descricao, ativo }),
        "Modelo atualizado",
      );
      if (ok) onDone();
      return;
    }
    if (!arquivo) return;
    const form = new FormData();
    form.set("arquivo", arquivo);
    form.set("nota", nota);
    if (edicao.tipo === "novo") {
      form.set("nome", nome);
      form.set("descricao", descricao);
    }
    const ok = await run(
      () =>
        apiClient.postForm(
          edicao.tipo === "novo"
            ? "v1/atlas/admin/modelos"
            : `v1/atlas/admin/modelos/${edicao.modelo.id}/versoes`,
          form,
        ),
      edicao.tipo === "novo" ? "Modelo cadastrado" : "Nova versão publicada",
    );
    if (ok) onDone();
  }

  return (
    <form onSubmit={(e) => void salvar(e)} className="flex flex-col gap-4">
      {edicao.tipo !== "versao" && (
        <>
          <Input
            label="Nome do modelo"
            name="nome"
            value={nome}
            maxLength={150}
            required
            onChange={(e) => setNome(e.target.value)}
          />
          <Textarea
            label="Descrição (quando usar, o que preencher)"
            name="descricao"
            value={descricao}
            maxLength={2000}
            rows={3}
            onChange={(e) => setDescricao(e.target.value)}
          />
        </>
      )}
      {edicao.tipo === "editar" ? (
        <div className="flex items-center gap-3">
          <Toggle checked={ativo} onChange={setAtivo} label="Modelo ativo" />
          <span className="text-sm text-foreground">
            {ativo
              ? "Ativo — disponível para novas peças"
              : "Desativado — sai da escolha de novas peças, mas segue baixável onde já está ligado"}
          </span>
        </div>
      ) : (
        <>
          <CampoArquivo onChange={setArquivo} />
          <Input
            label={edicao.tipo === "novo" ? "Nota (opcional)" : "O que mudou nesta versão"}
            name="nota"
            value={nota}
            maxLength={500}
            onChange={(e) => setNota(e.target.value)}
          />
        </>
      )}
      {edicao.tipo === "versao" && (
        <p className="text-xs text-muted">
          Todas as peças ligadas a este modelo passam a oferecer a nova versão; as anteriores ficam
          no histórico.
        </p>
      )}
      <div>
        <Button type="submit" loading={pending} disabled={edicao.tipo !== "editar" && !arquivo}>
          {edicao.tipo === "novo"
            ? "Cadastrar modelo"
            : edicao.tipo === "versao"
              ? "Publicar versão"
              : "Salvar"}
        </Button>
      </div>
    </form>
  );
}

/** Histórico de versões (carregado ao abrir). */
function Historico({ id }: { id: string }) {
  const det = useApiQuery<ModeloDocumento>(`v1/atlas/modelos/${id}`);
  const versoes = det.data?.versoes ?? [];
  if (det.isLoading) return <p className="px-3 py-2 text-xs text-muted">Carregando…</p>;
  return (
    <ol className="divide-y divide-surface-border">
      {versoes.map((v) => (
        <li key={v.versao} className="flex flex-wrap items-center justify-between gap-2 px-3 py-2">
          <span>
            <span className="font-medium text-foreground">Versão {v.versao}</span>{" "}
            <span className="text-xs text-muted">
              · {fmtDate(v.created_at)} · {v.arquivo_nome} · {fmtBytes(v.tamanho)}
            </span>
            {v.nota && <span className="block text-xs text-muted">{v.nota}</span>}
          </span>
          <a
            href={urlArquivoModelo(id, v.versao)}
            download={v.arquivo_nome}
            aria-label={`Baixar a versão ${v.versao}`}
            className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
          >
            <Download size={12} aria-hidden="true" /> Baixar
          </a>
        </li>
      ))}
    </ol>
  );
}

function CartaoModelo({
  m,
  canManage,
  onEditar,
}: {
  m: ModeloDocumento;
  canManage: boolean;
  onEditar: (e: Edicao) => void;
}) {
  const [historico, setHistorico] = useState(false);
  return (
    <li className="flex flex-col rounded-lg border border-surface-border bg-surface">
      <div className="flex flex-1 flex-col gap-2 p-4">
        <div className="flex items-start gap-3">
          <FileText size={20} aria-hidden="true" className="mt-0.5 shrink-0 text-primary" />
          <div className="min-w-0">
            <h3 className="font-semibold text-foreground">{m.nome}</h3>
            <p className="text-xs text-muted">
              {extensao(m.atual.arquivo_nome)} · versão {m.atual.versao} ·{" "}
              {fmtBytes(m.atual.tamanho)} · atualizado em {fmtDate(m.updated_at)}
              {canManage && m.updated_by && ` por ${m.updated_by}`}
            </p>
          </div>
        </div>
        {m.descricao && <p className="text-sm text-foreground">{m.descricao}</p>}
        <div className="flex flex-wrap gap-1">
          {!m.ativo && <Badge tone="warning">Desativado</Badge>}
          <Badge tone="neutral">
            {m.pecas_ligadas === 0
              ? "Nenhuma peça ligada"
              : `${m.pecas_ligadas} ${m.pecas_ligadas === 1 ? "peça ligada" : "peças ligadas"}`}
          </Badge>
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-2 border-t border-surface-border px-4 py-2.5">
        <a
          href={urlArquivoModelo(m.id)}
          download={m.atual.arquivo_nome}
          aria-label={`Baixar ${m.nome}`}
          className={buttonClass("secondary", "sm")}
        >
          <Download size={14} aria-hidden="true" /> Baixar
        </a>
        {extensao(m.atual.arquivo_nome) === "PDF" && (
          <a
            href={`${urlArquivoModelo(m.id)}?inline=1`}
            target="_blank"
            rel="noopener noreferrer"
            aria-label={`Visualizar ${m.nome} (abre em nova janela)`}
            className={buttonClass("ghost", "sm")}
          >
            <Eye size={14} aria-hidden="true" /> Visualizar
          </a>
        )}
        <Button
          variant="ghost"
          size="sm"
          aria-expanded={historico}
          onClick={() => setHistorico((h) => !h)}
        >
          <History size={14} aria-hidden="true" /> Versões
        </Button>
        {canManage && (
          <>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => onEditar({ tipo: "versao", modelo: m })}
            >
              <Upload size={14} aria-hidden="true" /> Nova versão
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => onEditar({ tipo: "editar", modelo: m })}
            >
              <Pencil size={14} aria-hidden="true" /> Editar
            </Button>
          </>
        )}
      </div>
      {historico && (
        <div className="border-t border-surface-border text-sm">
          <Historico id={m.id} />
        </div>
      )}
    </li>
  );
}

/** Biblioteca de modelos de documento (ADR 024): um catálogo reutilizável;
 * as peças dos procedimentos apontam para um modelo, e publicar uma nova
 * versão aqui vale para todos os fluxos que o usam. */
function Biblioteca() {
  const { can } = useNexus();
  const canManage = can("atlas:manage");
  const id = useId();
  const lista = useApiQuery<ModeloDocumento[]>(
    canManage ? "v1/atlas/admin/modelos" : "v1/atlas/modelos",
  );
  // ?q= vem da Busca Global (resultado do tipo modelo).
  const [busca, setBusca] = useState(useSearchParams().get("q") ?? "");
  const [edicao, setEdicao] = useState<Edicao | null>(null);
  const itens = useMemo(() => {
    const q = busca.trim().toLowerCase();
    const todos = lista.data ?? [];
    return q ? todos.filter((m) => `${m.nome} ${m.descricao}`.toLowerCase().includes(q)) : todos;
  }, [lista.data, busca]);

  return (
    <div className="flex flex-col gap-6">
      <Trilha itens={[{ label: "Atlas", href: "/atlas" }, { label: "Modelos de documento" }]} />
      <PageHeader
        eyebrow="Atlas"
        title="Modelos de documento"
        description="Minutas e formulários padronizados das peças dos procedimentos. Baixe o modelo, preencha e junte ao processo."
        actions={
          canManage && (
            <div className="flex flex-wrap gap-2">
              <Button variant="secondary" onClick={() => setEdicao({ tipo: "lote" })}>
                <Upload size={16} aria-hidden="true" className="mr-1" /> Enviar vários
              </Button>
              <Button onClick={() => setEdicao({ tipo: "novo" })}>
                <Plus size={16} aria-hidden="true" className="mr-1" /> Novo modelo
              </Button>
            </div>
          )
        }
      />
      <div className="max-w-md">
        <Input
          id={`${id}-busca`}
          label="Procurar modelo"
          type="search"
          value={busca}
          onChange={(e) => setBusca(e.target.value)}
          placeholder="Ex.: ofício, DFD, termo de referência"
        />
      </div>
      <DataState
        loading={lista.isLoading}
        error={lista.error}
        onRetry={() => void lista.mutate()}
        empty={itens.length === 0}
        emptyTitle={busca ? "Nenhum modelo encontrado" : "Nenhum modelo cadastrado"}
      >
        <ul className="grid gap-3 md:grid-cols-2" aria-label="Modelos">
          {itens.map((m) => (
            <CartaoModelo key={m.id} m={m} canManage={canManage} onEditar={setEdicao} />
          ))}
        </ul>
      </DataState>

      <Dialog
        open={!!edicao}
        onClose={() => setEdicao(null)}
        title={
          edicao?.tipo === "novo"
            ? "Novo modelo"
            : edicao?.tipo === "lote"
              ? "Enviar vários modelos"
              : edicao?.tipo === "versao"
                ? `Nova versão — ${edicao.modelo.nome}`
                : "Editar modelo"
        }
        size="lg"
      >
        {edicao?.tipo === "lote" && <EnvioEmLote onDone={() => void lista.mutate()} />}
        {edicao && edicao.tipo !== "lote" && (
          <FormModelo
            edicao={edicao}
            onDone={() => {
              setEdicao(null);
              void lista.mutate();
            }}
          />
        )}
      </Dialog>
    </div>
  );
}

export default function ModelosPage() {
  return (
    <Suspense>
      <Biblioteca />
    </Suspense>
  );
}
