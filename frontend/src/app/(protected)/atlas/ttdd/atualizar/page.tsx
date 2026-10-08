"use client";

import { AlertTriangle, CheckCircle2, Download, FileUp, ListChecks } from "lucide-react";
import Link from "next/link";
import { useId, useState } from "react";

import { Trilha } from "@/components/atlas/Trilha";
import { destinacao, fase, fonteTTDD } from "@/components/atlas/labels";
import { PageHeader } from "@/components/nexus/PageHeader";
import { useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button, buttonClass } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { apiClient } from "@/lib/api/client";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { ImpactoCargaTTDD, PrazosTTDD, SituacaoCarga } from "@/lib/nexus/types";

const SITUACOES: {
  valor: SituacaoCarga;
  titulo: string;
  tom: "info" | "warning" | "danger" | "success" | "neutral";
}[] = [
  { valor: "NOVA", titulo: "Novas", tom: "info" },
  { valor: "ALTERADA", titulo: "Alteradas", tom: "warning" },
  { valor: "REVOGADA", titulo: "Revogadas", tom: "danger" },
  { valor: "RESTABELECIDA", titulo: "Restabelecidas", tom: "success" },
  { valor: "INALTERADA", titulo: "Sem mudança", tom: "neutral" },
];

/** "1 ano + 4 anos → guarda permanente". */
function resumo(p: PrazosTTDD | null): string {
  if (!p) return "—";
  return `${fase(p.fase_corrente_anos, p.fase_corrente_condicao, true)} + ${fase(p.fase_interm_anos, p.fase_interm_condicao, false)} → ${destinacao(p.destinacao_final).label.toLowerCase()}`;
}

/** Atualizar a TTDD pela tela (atlas:manage — ADR 022): enviar a nova
 * publicação, conferir o impacto e só então aplicar. */
export default function AtualizarTTDDPage() {
  const { can } = useNexus();
  const id = useId();
  const { run, pending } = useAction();
  const [arquivo, setArquivo] = useState<{
    nome: string;
    formato: "csv" | "json";
    conteudo: string;
  } | null>(null);
  const [impacto, setImpacto] = useState<ImpactoCargaTTDD | null>(null);
  const [conferi, setConferi] = useState(false);

  if (!can("atlas:manage")) {
    return (
      <p className="text-sm text-muted">
        Atualizar a TTDD é restrito à gestão do Atlas (permissão atlas:manage).
      </p>
    );
  }

  async function escolher(file: File | undefined) {
    setImpacto(null);
    setConferi(false);
    if (!file) return setArquivo(null);
    setArquivo({
      nome: file.name,
      formato: file.name.toLowerCase().endsWith(".json") ? "json" : "csv",
      conteudo: await file.text(),
    });
  }

  async function simular() {
    if (!arquivo) return;
    const res = await run(() =>
      apiClient.post<ImpactoCargaTTDD>("v1/atlas/admin/ttdd/carga/simular", {
        formato: arquivo.formato,
        conteudo: arquivo.conteudo,
      }),
    );
    if (res) {
      setImpacto(res.data);
      setConferi(false);
    }
  }

  async function aplicar() {
    if (!arquivo || !impacto) return;
    const res = await run(
      () =>
        apiClient.post<ImpactoCargaTTDD>("v1/atlas/admin/ttdd/carga/aplicar", {
          formato: arquivo.formato,
          conteudo: arquivo.conteudo,
          hash: impacto.hash,
        }),
      "Nova TTDD aplicada",
    );
    if (res) setImpacto(res.data);
  }

  const mudancas = impacto?.series ?? [];
  const grupo = (s: SituacaoCarga) => mudancas.filter((x) => x.situacao === s);

  return (
    <div className="flex flex-col gap-6">
      <Trilha
        itens={[
          { label: "Atlas", href: "/atlas" },
          { label: "Tabela de Temporalidade", href: "/atlas/ttdd" },
          { label: "Atualizar" },
        ]}
      />
      <PageHeader
        eyebrow="Atlas"
        title="Atualizar a TTDD"
        description="Nova publicação da CCPAD no Diário Oficial: envie a tabela revisada, confira o impacto e aplique. Séries retiradas ficam revogadas (nunca apagadas) e os prazos anteriores vão para o histórico."
      />

      <Card>
        <CardHeader>
          <CardTitle as="h2" className="flex items-center gap-2">
            <FileUp size={18} aria-hidden="true" className="text-primary" /> 1. Enviar a nova tabela
          </CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4 pb-5 pt-3 text-sm">
          <ol className="list-decimal space-y-1 pl-5 text-muted">
            <li>
              Baixe a TTDD atual em planilha, revise no Excel (mantenha o cabeçalho e as colunas) e
              salve em CSV; ou use o <code>ttdd.json</code> gerado do PDF oficial (
              <code>scripts/ttdd</code>).
            </li>
            <li>
              A carga vale para os órgãos presentes no arquivo: séries de outros órgãos não mudam.
            </li>
          </ol>
          <div className="flex flex-wrap items-end gap-3">
            <a
              href="/api/backend/v1/atlas/ttdd/exportar"
              download="ttdd.csv"
              className={buttonClass("secondary")}
            >
              <Download size={16} aria-hidden="true" /> Baixar a TTDD atual (CSV)
            </a>
            <div className="flex flex-col gap-1">
              <label htmlFor={`${id}-arquivo`} className="text-sm font-medium text-foreground">
                Arquivo da nova TTDD (.csv ou .json)
              </label>
              <input
                id={`${id}-arquivo`}
                type="file"
                accept=".csv,.json,text/csv,application/json"
                onChange={(e) => void escolher(e.target.files?.[0])}
                className="text-sm"
              />
            </div>
            <Button
              onClick={() => void simular()}
              disabled={!arquivo}
              loading={pending && !impacto}
            >
              <ListChecks size={16} aria-hidden="true" /> Simular
            </Button>
          </div>
        </CardContent>
      </Card>

      {impacto && (
        <Card>
          <CardHeader>
            <CardTitle as="h2">
              2. Impacto {impacto.aplicada ? "(aplicado)" : "(simulação — nada foi gravado)"}
            </CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-5 pb-5 pt-3 text-sm">
            <ul className="grid gap-2 sm:grid-cols-5" aria-label="Totais por situação">
              {SITUACOES.map((s) => (
                <li
                  key={s.valor}
                  className="rounded-lg border border-surface-border p-3 text-center"
                >
                  <span className="block text-2xl font-bold text-foreground">
                    {impacto.totais[s.valor] ?? 0}
                  </span>
                  <Badge tone={s.tom}>{s.titulo}</Badge>
                </li>
              ))}
            </ul>
            <p className="text-xs text-muted">
              Órgãos na carga:{" "}
              {impacto.orgaos.map((o) => `${o.prefixo} — ${fonteTTDD(o)}`).join(" · ")}
            </p>

            {impacto.procedimentos.length > 0 && (
              <section
                aria-labelledby={`${id}-procs`}
                className="rounded-md border border-warning/40 bg-warning/10 p-3"
              >
                <h3
                  id={`${id}-procs`}
                  className="flex items-center gap-2 font-semibold text-foreground"
                >
                  <AlertTriangle size={16} aria-hidden="true" className="text-warning" />{" "}
                  Procedimentos afetados
                </h3>
                <p className="mt-1 text-xs text-muted">
                  Apontam para uma série alterada ou revogada: depois de aplicar, revise cada um e
                  publique uma nova versão se preciso.
                </p>
                <ul className="mt-2 flex flex-col gap-1">
                  {impacto.procedimentos.map((p) => (
                    <li key={p.id}>
                      <Link
                        href={`/atlas/procedimentos/${p.id}`}
                        className="font-mono text-primary hover:underline"
                      >
                        {p.codigo_processual} v{p.versao}
                      </Link>{" "}
                      — série {p.codigo_ttdd} {p.situacao === "REVOGADA" ? "revogada" : "alterada"}
                      {!p.ativo && " (inativo)"}
                    </li>
                  ))}
                </ul>
              </section>
            )}

            {(["ALTERADA", "REVOGADA", "RESTABELECIDA", "NOVA"] as const).map((sit) => {
              const lista = grupo(sit);
              if (lista.length === 0) return null;
              const titulo = SITUACOES.find((s) => s.valor === sit)!.titulo;
              return (
                <details
                  key={sit}
                  open={sit !== "NOVA" || lista.length <= 20}
                  className="rounded-md border border-surface-border"
                >
                  <summary className="cursor-pointer px-3 py-2 font-semibold text-foreground">
                    {titulo} ({lista.length})
                  </summary>
                  <ul className="divide-y divide-surface-border">
                    {lista.slice(0, 500).map((s) => (
                      <li key={s.codigo} className="px-3 py-2">
                        <span className="font-mono text-xs font-bold text-primary">{s.codigo}</span>{" "}
                        <span className="text-foreground">{s.descritor}</span>
                        <span className="block text-xs text-muted">
                          {sit === "ALTERADA"
                            ? `Antes: ${resumo(s.antes)} · Depois: ${resumo(s.depois)}`
                            : sit === "REVOGADA"
                              ? `Prazos atuais: ${resumo(s.antes)}`
                              : resumo(s.depois)}
                        </span>
                        {sit === "ALTERADA" &&
                          s.antes &&
                          s.depois &&
                          s.antes.observacoes !== s.depois.observacoes && (
                            <span className="block text-xs text-muted">
                              Observações: “{s.antes.observacoes || "—"}” → “
                              {s.depois.observacoes || "—"}”
                            </span>
                          )}
                      </li>
                    ))}
                  </ul>
                  {lista.length > 500 && (
                    <p className="px-3 py-2 text-xs text-muted">… e mais {lista.length - 500}.</p>
                  )}
                </details>
              );
            })}

            {impacto.aplicada ? (
              <p className="flex items-center gap-2 font-medium text-success">
                <CheckCircle2 size={16} aria-hidden="true" /> TTDD atualizada.{" "}
                <Link href="/atlas/ttdd" className="text-primary hover:underline">
                  Ver a tabela
                </Link>
              </p>
            ) : mudancas.length === 0 ? (
              <p className="text-muted">Nada muda: a tabela enviada é igual à que está em vigor.</p>
            ) : (
              <div className="flex flex-col gap-3 border-t border-surface-border pt-4">
                <h3 className="font-semibold text-foreground">3. Aplicar</h3>
                <label className="flex items-start gap-2">
                  <input
                    type="checkbox"
                    checked={conferi}
                    onChange={(e) => setConferi(e.target.checked)}
                    className="mt-0.5 h-4 w-4 accent-primary"
                  />
                  Conferi o impacto acima e a nova tabela corresponde à publicação oficial no Diário
                  Oficial.
                </label>
                <div>
                  <Button onClick={() => void aplicar()} disabled={!conferi} loading={pending}>
                    Aplicar a nova TTDD
                  </Button>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
