"use client";

import { CheckCircle2, Circle, Download, ExternalLink, Tags } from "lucide-react";
import Link from "next/link";
import { useId, useState, type FormEvent } from "react";

import { destinacao, fase } from "@/components/atlas/labels";
import { useAction } from "@/components/nexus/useAction";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { apiClient } from "@/lib/api/client";
import { useApiPage, useApiQuery, withQuery } from "@/lib/api/swr";
import { chave, urlArquivoModelo } from "@/lib/atlas/modelos";
import { calcularDestinacao, dataBR } from "@/lib/atlas/temporalidade";
import type {
  ClassificacaoTTDD,
  Documento,
  ModeloDocumento,
  Processo,
  Workflow,
} from "@/lib/nexus/types";

/** Peça exigida pelo procedimento, com o modelo para baixar (o da peça ou,
 * sem ele, o primeiro modelo ativo da série — ADR 024). */
export interface PecaAtlas {
  nome: string;
  etapa: number;
  obrigatoria: boolean;
  modelo?: { id: string; nome: string; arquivo: string };
}

/** Carrega o procedimento do Atlas do processo e monta as peças. O plugin
 * Atlas pode estar desligado: sem resposta, não há peças (sem erro). */
export function usePecasDoAtlas(p: Pick<Processo, "atlas_procedimento_id" | "codigo_ttdd">) {
  const wf = useApiQuery<Workflow>(
    p.atlas_procedimento_id ? `v1/atlas/workflows/${p.atlas_procedimento_id}` : null,
  );
  const serie = wf.data?.codigo_ttdd || p.codigo_ttdd;
  const daSerie = useApiQuery<ModeloDocumento[]>(
    serie ? `v1/atlas/ttdd/${encodeURIComponent(serie)}/modelos` : null,
  );
  const reserva = (daSerie.data ?? []).find((m) => m.ativo);
  const pecas: PecaAtlas[] = (wf.data?.etapas ?? []).flatMap((e) =>
    e.documentos.map((d) => ({
      nome: d.nome_documento,
      etapa: e.ordem,
      obrigatoria: d.obrigatorio,
      modelo: d.modelo
        ? { id: d.modelo.id, nome: d.modelo.nome, arquivo: d.modelo.arquivo_nome }
        : reserva
          ? { id: reserva.id, nome: reserva.nome, arquivo: reserva.atual.arquivo_nome }
          : undefined,
    })),
  );
  return { procedimento: wf.data, procedimentoAusente: !!wf.error, pecas };
}

/** A peça já está no processo? Compara o tipo ou o título do documento
 * com o nome da peça (sem acento e caixa). */
export function pecaJuntada(peca: string, docs: Documento[]): boolean {
  const k = chave(peca);
  return docs.some(
    (d) => d.status !== "cancelado" && (chave(d.tipo) === k || chave(d.titulo) === k),
  );
}

/** Situação da guarda do processo encerrado pela série da TTDD. */
export function situacaoGuarda(
  serie: ClassificacaoTTDD,
  encerradoEm: string | undefined,
  hoje = new Date(),
): string {
  if (!encerradoEm) return "A guarda começa a contar no encerramento do processo.";
  const c = calcularDestinacao(serie, new Date(encerradoEm));
  if (!c.possivel) return c.motivo;
  if (hoje < c.fimCorrente) return `Fase corrente até ${dataBR(c.fimCorrente)}.`;
  if (!c.semIntermediaria && hoje < c.fimIntermediaria)
    return `No arquivo intermediário até ${dataBR(c.fimIntermediaria)}.`;
  switch (serie.destinacao_final) {
    case "ELIMINACAO":
      return `Prazo de guarda cumprido em ${dataBR(c.fimIntermediaria)}: pode ser eliminado, com a aprovação da CCPAD.`;
    case "GUARDA_PERMANENTE":
      return `Prazo cumprido em ${dataBR(c.fimIntermediaria)}: recolher ao arquivo permanente.`;
    default:
      return `Prazo cumprido em ${dataBR(c.fimIntermediaria)}; a TTDD não define a destinação final.`;
  }
}

/** Classificar o processo: procedimento do Atlas e série da TTDD. */
export function ClassificarProcesso({
  processo,
  onDone,
}: {
  processo: Processo;
  onDone: (p: Processo) => void;
}) {
  const id = useId();
  const { run, pending } = useAction();
  const lista = useApiPage<Workflow>(withQuery("v1/atlas/workflows", { page_size: 100 }));
  const procedimentos = lista.data?.items ?? [];
  const [procedimento, setProcedimento] = useState(processo.atlas_procedimento_id ?? "");
  const [serie, setSerie] = useState(processo.codigo_ttdd ?? "");

  async function salvar(e: FormEvent) {
    e.preventDefault();
    const res = await run(
      () =>
        apiClient.put<Processo>(`v1/tramite/processos/${processo.id}/classificacao`, {
          atlas_procedimento_id: procedimento || null,
          codigo_ttdd: serie.trim(),
        }),
      "Classificação salva",
    );
    if (res) onDone(res.data);
  }

  return (
    <form onSubmit={(e) => void salvar(e)} className="flex flex-col gap-4">
      <Select
        id={`${id}-proc`}
        label="Procedimento do Atlas"
        value={procedimento}
        onChange={(e) => {
          setProcedimento(e.target.value);
          const w = procedimentos.find((x) => x.id === e.target.value);
          if (w) setSerie(w.codigo_ttdd);
        }}
        options={[
          { value: "", label: "Nenhum" },
          ...procedimentos.map((w) => ({
            value: w.id,
            label: `${w.codigo_processual} — ${w.titulo}`,
          })),
        ]}
      />
      <Input
        id={`${id}-serie`}
        label="Série da TTDD (guarda do processo)"
        placeholder="Ex.: 2.0.02.00.07"
        value={serie}
        maxLength={32}
        onChange={(e) => setSerie(e.target.value)}
      />
      <div>
        <Button type="submit" loading={pending}>
          Salvar classificação
        </Button>
      </div>
    </form>
  );
}

function Guarda({ codigo, encerradoEm }: { codigo: string; encerradoEm?: string }) {
  const serie = useApiQuery<ClassificacaoTTDD>(`v1/atlas/ttdd/${encodeURIComponent(codigo)}`);
  const c = serie.data;
  return (
    <section aria-labelledby="tr-atlas-guarda" className="flex flex-col gap-1">
      <h3 id="tr-atlas-guarda" className="text-xs font-semibold uppercase tracking-wide text-muted">
        Guarda (TTDD)
      </h3>
      <Link
        href={`/atlas/ttdd/${encodeURIComponent(codigo)}`}
        className="text-primary hover:underline"
      >
        <span className="font-mono">{codigo}</span>
        {c && ` — ${c.descritor}`}
      </Link>
      {c && (
        <>
          <p className="text-xs text-muted">
            Corrente: {fase(c.fase_corrente_anos, c.fase_corrente_condicao, true)} · Intermediária:{" "}
            {fase(c.fase_interm_anos, c.fase_interm_condicao, false)} · Destinação:{" "}
            {destinacao(c.destinacao_final).label.toLowerCase()}
          </p>
          <p className="text-sm text-foreground">{situacaoGuarda(c, encerradoEm)}</p>
        </>
      )}
    </section>
  );
}

/** Cartão "Atlas" do processo (ADR 026): o procedimento seguido, o checklist
 * das peças com o modelo para baixar e a guarda pela série da TTDD. */
export function AtlasDoProcesso({
  processo,
  documentos,
  podeClassificar,
  onClassificado,
}: {
  processo: Processo;
  documentos: Documento[];
  podeClassificar: boolean;
  onClassificado: (p: Processo) => void;
}) {
  const { procedimento, procedimentoAusente, pecas } = usePecasDoAtlas(processo);
  const [classificando, setClassificando] = useState(false);
  const juntadas = pecas.filter((x) => pecaJuntada(x.nome, documentos)).length;
  const serie = procedimento?.codigo_ttdd || processo.codigo_ttdd;

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle as="h2">Atlas</CardTitle>
        {podeClassificar && (
          <Button size="sm" variant="ghost" onClick={() => setClassificando(true)}>
            <Tags size={14} aria-hidden="true" className="mr-1" /> Classificar
          </Button>
        )}
      </CardHeader>
      <CardContent className="flex flex-col gap-4 pb-4 pt-3 text-sm">
        {!processo.atlas_procedimento_id && !processo.codigo_ttdd && (
          <p className="text-muted">
            Processo sem classificação: ligue-o a um procedimento do Atlas para ver as peças
            exigidas, os modelos e a guarda.
          </p>
        )}
        {procedimentoAusente && (
          <p className="text-muted">O procedimento ligado não está mais em vigor no Atlas.</p>
        )}
        {procedimento && (
          <section aria-labelledby="tr-atlas-pecas" className="flex flex-col gap-2">
            <Link
              href={`/atlas/procedimentos/${procedimento.id}`}
              className="inline-flex items-center gap-1 font-medium text-primary hover:underline"
            >
              {procedimento.codigo_processual} — {procedimento.titulo}
              <ExternalLink size={12} aria-hidden="true" />
            </Link>
            <h3
              id="tr-atlas-pecas"
              className="text-xs font-semibold uppercase tracking-wide text-muted"
            >
              Peças exigidas ({juntadas}/{pecas.length} no processo)
            </h3>
            <ul className="flex flex-col gap-1.5">
              {pecas.map((x) => {
                const ok = pecaJuntada(x.nome, documentos);
                return (
                  <li
                    key={`${x.etapa}-${x.nome}`}
                    className="flex items-start justify-between gap-2"
                  >
                    <span className="flex items-start gap-1.5">
                      {ok ? (
                        <CheckCircle2
                          size={14}
                          aria-label="no processo"
                          className="mt-0.5 shrink-0 text-success"
                        />
                      ) : (
                        <Circle
                          size={14}
                          aria-label="pendente"
                          className="mt-0.5 shrink-0 text-muted"
                        />
                      )}
                      <span>
                        {x.nome}
                        <span className="block text-xs text-muted">
                          Etapa {x.etapa} · {x.obrigatoria ? "obrigatória" : "opcional"}
                        </span>
                      </span>
                    </span>
                    {x.modelo && (
                      <a
                        href={urlArquivoModelo(x.modelo.id)}
                        download={x.modelo.arquivo}
                        aria-label={`Baixar o modelo ${x.modelo.nome}`}
                        className="inline-flex shrink-0 items-center gap-1 text-xs font-medium text-primary hover:underline"
                      >
                        <Download size={12} aria-hidden="true" /> Modelo
                      </a>
                    )}
                  </li>
                );
              })}
            </ul>
          </section>
        )}
        {serie && <Guarda codigo={serie} encerradoEm={processo.concluido_at} />}
      </CardContent>
      <Dialog
        open={classificando}
        onClose={() => setClassificando(false)}
        title="Classificar o processo"
        description="O procedimento do Atlas traz as peças e os modelos; a série da TTDD define a guarda."
      >
        {classificando && (
          <ClassificarProcesso
            processo={processo}
            onDone={(p) => {
              setClassificando(false);
              onClassificado(p);
            }}
          />
        )}
      </Dialog>
    </Card>
  );
}
