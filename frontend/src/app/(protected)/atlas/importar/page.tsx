"use client";

import { CheckCircle2, Download, FileUp, ListChecks } from "lucide-react";
import Link from "next/link";
import { useId, useState } from "react";

import { Trilha } from "@/components/atlas/Trilha";
import { PageHeader } from "@/components/nexus/PageHeader";
import { useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button, buttonClass } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { apiClient } from "@/lib/api/client";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { ImportacaoProcedimentos, SituacaoImportacao } from "@/lib/nexus/types";

const SITUACOES: {
  valor: SituacaoImportacao;
  titulo: string;
  tom: "info" | "warning" | "danger" | "neutral";
}[] = [
  { valor: "NOVO", titulo: "Novos", tom: "info" },
  { valor: "NOVA_VERSAO", titulo: "Novas versões", tom: "warning" },
  { valor: "INALTERADO", titulo: "Sem mudança", tom: "neutral" },
  { valor: "ERRO", titulo: "Com erro", tom: "danger" },
];

/** Importar procedimentos em lote (atlas:manage — ADR 025): baixar os
 * procedimentos em vigor, revisar ou acrescentar no arquivo, simular e
 * aplicar só o arquivo conferido. */
export default function ImportarProcedimentosPage() {
  const { can } = useNexus();
  const id = useId();
  const { run, pending } = useAction();
  const [arquivo, setArquivo] = useState<{ nome: string; conteudo: string } | null>(null);
  const [resultado, setResultado] = useState<ImportacaoProcedimentos | null>(null);
  // Rascunho (padrão): os procedimentos entram para validação, sem publicar.
  const [rascunho, setRascunho] = useState(true);

  if (!can("atlas:manage")) {
    return (
      <p className="text-sm text-muted">
        Importar procedimentos é restrito à gestão do Atlas (permissão atlas:manage).
      </p>
    );
  }

  async function escolher(file: File | undefined) {
    setResultado(null);
    setArquivo(file ? { nome: file.name, conteudo: await file.text() } : null);
  }

  async function enviar(aplicar: boolean) {
    if (!arquivo) return;
    const res = await run(
      () =>
        apiClient.post<ImportacaoProcedimentos>(
          `v1/atlas/admin/workflows/importacao/${aplicar ? "aplicar" : "simular"}`,
          aplicar
            ? { conteudo: arquivo.conteudo, hash: resultado?.hash, rascunho }
            : { conteudo: arquivo.conteudo, rascunho },
        ),
      aplicar ? "Procedimentos importados" : undefined,
    );
    if (res) setResultado(res.data);
  }

  const erros = resultado?.totais.ERRO ?? 0;
  const mudancas = (resultado?.totais.NOVO ?? 0) + (resultado?.totais.NOVA_VERSAO ?? 0);

  return (
    <div className="flex flex-col gap-6">
      <Trilha itens={[{ label: "Atlas", href: "/atlas" }, { label: "Importar procedimentos" }]} />
      <PageHeader
        eyebrow="Atlas"
        title="Importar procedimentos"
        description="Cadastre ou atualize vários fluxos de uma vez: baixe o arquivo dos procedimentos em vigor, edite ou acrescente procedimentos, simule e aplique. Cada procedimento novo entra na versão 1; o que mudou ganha uma nova versão; o que está igual fica como está."
      />

      <Card>
        <CardHeader>
          <CardTitle as="h2" className="flex items-center gap-2">
            <FileUp size={18} aria-hidden="true" className="text-primary" /> 1. Enviar o arquivo
          </CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4 pb-5 pt-3 text-sm">
          <ol className="list-decimal space-y-1 pl-5 text-muted">
            <li>
              Baixe <code>procedimentos.json</code>: ele já vem no formato da importação e serve de
              modelo para os novos.
            </li>
            <li>
              Cada peça pode apontar para um modelo da biblioteca pelo nome (campo{" "}
              <code>&quot;modelo&quot;</code>).
            </li>
          </ol>
          <div className="flex flex-wrap items-end gap-3">
            <a
              href="/api/backend/v1/atlas/admin/workflows/exportar"
              download="procedimentos.json"
              className={buttonClass("secondary")}
            >
              <Download size={16} aria-hidden="true" /> Baixar os procedimentos em vigor
            </a>
            <div className="flex flex-col gap-1">
              <label htmlFor={`${id}-arquivo`} className="text-sm font-medium text-foreground">
                Arquivo de procedimentos (.json)
              </label>
              <input
                id={`${id}-arquivo`}
                type="file"
                accept=".json,application/json"
                onChange={(e) => void escolher(e.target.files?.[0])}
                className="text-sm"
              />
            </div>
            <label className="flex items-center gap-2 pb-2 text-sm">
              <input
                type="checkbox"
                checked={rascunho}
                onChange={(e) => {
                  setRascunho(e.target.checked);
                  setResultado(null);
                }}
                className="h-4 w-4 accent-primary"
              />
              Importar como rascunho, para validação (não publica)
            </label>
            <Button
              onClick={() => void enviar(false)}
              disabled={!arquivo}
              loading={pending && !resultado}
            >
              <ListChecks size={16} aria-hidden="true" /> Simular
            </Button>
          </div>
        </CardContent>
      </Card>

      {resultado && (
        <Card>
          <CardHeader>
            <CardTitle as="h2">
              2. Resultado {resultado.aplicada ? "(aplicado)" : "(simulação — nada foi gravado)"}
            </CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-4 pb-5 pt-3 text-sm">
            <ul className="grid gap-2 sm:grid-cols-4" aria-label="Totais por situação">
              {SITUACOES.map((s) => (
                <li
                  key={s.valor}
                  className="rounded-lg border border-surface-border p-3 text-center"
                >
                  <span className="block text-2xl font-bold text-foreground">
                    {resultado.totais[s.valor] ?? 0}
                  </span>
                  <Badge tone={s.tom}>{s.titulo}</Badge>
                </li>
              ))}
            </ul>
            <ul
              className="divide-y divide-surface-border rounded-md border border-surface-border"
              aria-label="Procedimentos do arquivo"
            >
              {resultado.itens.map((it) => {
                const s = SITUACOES.find((x) => x.valor === it.situacao)!;
                return (
                  <li
                    key={it.linha}
                    className="flex flex-wrap items-start justify-between gap-2 px-3 py-2"
                  >
                    <span>
                      <span className="font-mono text-xs font-bold text-primary">
                        {it.codigo_processual || `item ${it.linha}`}
                      </span>{" "}
                      <span className="text-foreground">{it.titulo}</span>
                      {it.erro && (
                        <span className="block text-xs text-danger">
                          Item {it.linha}: {it.erro}
                        </span>
                      )}
                    </span>
                    <span className="flex items-center gap-2">
                      {it.versao > 0 && (
                        <span className="text-xs text-muted">versão {it.versao}</span>
                      )}
                      <Badge tone={s.tom}>{s.titulo}</Badge>
                    </span>
                  </li>
                );
              })}
            </ul>
            {resultado.aplicada ? (
              <p className="flex items-center gap-2 font-medium text-success">
                <CheckCircle2 size={16} aria-hidden="true" /> Importação aplicada.{" "}
                <Link href="/atlas" className="text-primary hover:underline">
                  Ver os procedimentos
                </Link>
              </p>
            ) : erros > 0 ? (
              <p className="text-danger">
                Corrija os itens com erro no arquivo e simule de novo: a importação só é aplicada
                sem erros.
              </p>
            ) : mudancas === 0 ? (
              <p className="text-muted">
                Nada muda: os procedimentos do arquivo já estão em vigor.
              </p>
            ) : (
              <div>
                <Button onClick={() => void enviar(true)} loading={pending}>
                  Aplicar a importação
                </Button>
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
