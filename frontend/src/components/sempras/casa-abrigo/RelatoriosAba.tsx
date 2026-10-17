"use client";

import { useState, type FormEvent } from "react";
import { Download, FileDown, Pencil, Printer, Settings2, Trash2 } from "lucide-react";

import { DataState } from "@/components/nexus/DataState";
import { useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import {
  MOTIVOS_ACOLHIMENTO,
  STATUS_RELATORIO,
  TIPOS_DESLIGAMENTO,
  TIPOS_RELATORIO,
  baixarArquivo,
  dataISO,
  diaDe,
  fmtDia,
  hojeISO,
  linhaCSV,
  nomeRelatorio,
  type AcolhidoResumo,
  type EstatisticasCasaAbrigo,
  type MotivoAcolhimento,
  type RelatorioCasaAbrigo,
  type StatusRelatorio,
  type TipoDesligamento,
  type TipoRelatorio,
} from "@/lib/sempras/types";

import { Abas, SeletorAcolhido } from "./campos";

type Periodo = "mensal" | "trimestral" | "anual";

/** O intervalo [de, ate] do período que contém o mês de referência. */
export function intervalo(periodo: Periodo, mes: string): { de: string; ate: string; rotulo: string } {
  const [a, m] = mes.split("-").map(Number) as [number, number];
  const p = (n: number) => String(n).padStart(2, "0");
  const fimDoMes = (ano: number, mm: number) => new Date(ano, mm, 0).getDate();
  if (periodo === "anual") return { de: `${a}-01-01`, ate: `${a}-12-31`, rotulo: `Ano de ${a}` };
  if (periodo === "trimestral") {
    const t = Math.floor((m - 1) / 3);
    const ini = t * 3 + 1;
    return { de: `${a}-${p(ini)}-01`, ate: `${a}-${p(ini + 2)}-${fimDoMes(a, ini + 2)}`, rotulo: `${t + 1}º trimestre de ${a}` };
  }
  return { de: `${mes}-01`, ate: `${mes}-${fimDoMes(a, m)}`, rotulo: `Mês ${p(m)}/${a}` };
}

/** Barra horizontal com quantidade e percentual. */
function Barra({ rotulo, valor, total }: { rotulo: string; valor: number; total: number }) {
  const pct = total > 0 ? Math.round((valor / total) * 100) : 0;
  return (
    <li className="flex flex-col gap-1 text-sm">
      <span className="flex justify-between gap-2">
        <span>{rotulo}</span>
        <span className="font-semibold">
          {valor} ({pct}%)
        </span>
      </span>
      <span className="h-2 overflow-hidden rounded-full bg-surface-border" aria-hidden="true">
        <span className="block h-full bg-primary" style={{ width: `${pct}%` }} />
      </span>
    </li>
  );
}

/** Relatório estatístico do período (mensal, trimestral ou anual). */
function Estatistico({ unidadeId }: { unidadeId: string }) {
  const [periodo, setPeriodo] = useState<Periodo>("mensal");
  const [mes, setMes] = useState(hojeISO().slice(0, 7));
  const { de, ate, rotulo } = intervalo(periodo, mes);
  const est = useApiQuery<EstatisticasCasaAbrigo>(`v1/sempras/unidades/${unidadeId}/casa-abrigo/estatisticas?de=${de}&ate=${ate}`);
  const e = est.data;
  const motivos = e ? (Object.entries(e.motivos) as [MotivoAcolhimento, number][]).sort((x, y) => y[1] - x[1]) : [];
  const totalMotivos = motivos.reduce((s, [, n]) => s + n, 0);

  function exportar() {
    if (!e) return;
    const linhas = [
      linhaCSV(["Indicador", "Quantidade"]),
      linhaCSV(["Período", `${fmtDia(e.de)} a ${fmtDia(e.ate)}`]),
      linhaCSV(["Entradas", e.entradas]),
      linhaCSV(["Saídas", e.saidas]),
      linhaCSV(["Reintegrações familiares", e.reintegracoes]),
      linhaCSV(["Adoções", e.adocoes]),
      linhaCSV(["Transferências", e.transferencias]),
      linhaCSV(["Acolhidos no fim do período", e.ativos_no_fim]),
      ...motivos.map(([k, n]) => linhaCSV([`Motivo: ${MOTIVOS_ACOLHIMENTO[k]}`, n])),
    ];
    baixarArquivo(`estatistico_casa_abrigo_${de}_${ate}.csv`, linhas.join("\r\n"), "text/csv;charset=utf-8;");
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end gap-3 print:hidden">
        <Select id="estatistico-periodo" label="Período" value={periodo} onChange={(ev) => setPeriodo(ev.target.value as Periodo)}
          options={[{ value: "mensal", label: "Mensal" }, { value: "trimestral", label: "Trimestral" }, { value: "anual", label: "Anual" }]} />
        <Input id="estatistico-mes" label="Mês de referência" type="month" value={mes} onChange={(ev) => ev.target.value && setMes(ev.target.value)} />
        <Button type="button" size="sm" variant="secondary" onClick={() => window.print()}>
          <Printer size={14} className="mr-1" aria-hidden="true" /> Imprimir
        </Button>
        <Button type="button" size="sm" variant="secondary" onClick={exportar} disabled={!e}>
          <Download size={14} className="mr-1" aria-hidden="true" /> Exportar CSV
        </Button>
      </div>
      <p className="rounded-lg bg-primary/5 p-3 text-center text-sm font-semibold text-primary">Exibindo: {rotulo}</p>
      <DataState loading={est.isLoading} error={est.error} onRetry={() => void est.mutate()} empty={!e}>
        {e && (
          <div className="flex flex-col gap-4">
            <dl className="grid gap-3 sm:grid-cols-3">
              {[["Reintegrações familiares", e.reintegracoes], ["Processos de adoção", e.adocoes], ["Transferências", e.transferencias]].map(([t, n]) => (
                <div key={t} className="rounded-xl border border-surface-border p-4">
                  <dt className="text-xs text-muted">{t}</dt>
                  <dd className="text-2xl font-bold">{String(n).padStart(2, "0")}</dd>
                </div>
              ))}
            </dl>
            <div className="grid gap-4 md:grid-cols-2">
              <section aria-label="Motivos de acolhimento" className="rounded-xl border border-surface-border p-4">
                <h3 className="mb-3 text-sm font-semibold">Motivos de acolhimento (entradas)</h3>
                {motivos.length === 0 ? (
                  <p className="text-sm text-muted">Nenhuma entrada no período.</p>
                ) : (
                  <ul className="flex flex-col gap-2">
                    {motivos.map(([k, n]) => (
                      <Barra key={k} rotulo={MOTIVOS_ACOLHIMENTO[k]} valor={n} total={totalMotivos} />
                    ))}
                  </ul>
                )}
              </section>
              <section aria-label="Entradas e saídas" className="flex flex-col gap-3 rounded-xl border border-surface-border p-4">
                <h3 className="text-sm font-semibold">Entradas vs saídas</h3>
                <ul className="flex flex-col gap-2">
                  <Barra rotulo="Entradas" valor={e.entradas} total={e.entradas + e.saidas} />
                  <Barra rotulo="Saídas" valor={e.saidas} total={e.entradas + e.saidas} />
                </ul>
                <h3 className="text-sm font-semibold">Saídas por tipo</h3>
                <ul className="text-sm">
                  {(Object.entries(e.saidas_por_tipo) as [TipoDesligamento, number][]).map(([k, n]) => (
                    <li key={k}>
                      {TIPOS_DESLIGAMENTO[k]}: <strong>{n}</strong>
                    </li>
                  ))}
                  {Object.keys(e.saidas_por_tipo).length === 0 && <li className="text-muted">Nenhuma saída no período.</li>}
                </ul>
                <p className="text-sm text-muted">
                  No fim do período: {e.ativos_no_fim} acolhidos ({e.criancas} crianças, {e.adolescentes} adolescentes; {e.meninas} meninas,{" "}
                  {e.meninos} meninos).
                </p>
              </section>
            </div>
          </div>
        )}
      </DataState>
    </div>
  );
}

const nomeArquivo = (r: RelatorioCasaAbrigo, ext: string) =>
  `${nomeRelatorio(r).replace(/[^\p{L}\p{N} ]/gu, "").trim().replace(/ +/g, "_")}_${hojeISO()}.${ext}`;

/** Editor do conteúdo de um relatório (o entregue fica só para leitura, R8). */
function Editor({ relatorio, onFechar, onSalvo }: { relatorio: RelatorioCasaAbrigo; onFechar: () => void; onSalvo: () => void }) {
  const [conteudo, setConteudo] = useState(relatorio.conteudo);
  const entregue = relatorio.status === "entregue";
  const { run, pending } = useAction();
  const titulo = nomeRelatorio(relatorio);
  const sub = `Entrega: ${fmtDia(relatorio.data_entrega)}${relatorio.acolhido_nome ? ` · ${relatorio.acolhido_nome}` : ""}`;

  async function salvar() {
    const ok = await run(() => apiClient.put(`v1/sempras/casa-abrigo-relatorios/${relatorio.id}`, {
      acolhimento_id: relatorio.acolhimento_id, tipo: relatorio.tipo, tipo_outro: relatorio.tipo_outro, status: relatorio.status,
      data_entrega: relatorio.data_entrega, conteudo }), "Conteúdo salvo");
    if (ok) onSalvo();
  }
  function txt() {
    const linha = "=".repeat(60);
    baixarArquivo(nomeArquivo(relatorio, "txt"), `${linha}\n${titulo.toUpperCase()}\n${sub}\nGerado em: ${fmtDia(hojeISO())}\n${linha}\n\n${conteudo}`,
      "text/plain;charset=utf-8;");
  }
  function doc() {
    const esc = (s: string) => s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
    const html = `<html><head><meta charset="utf-8"><title>${esc(titulo)}</title></head><body style="font-family:Arial;font-size:12pt;line-height:1.8">` +
      `<h1>${esc(titulo)}</h1><p>${esc(sub)}</p><pre style="white-space:pre-wrap;font-family:Arial">${esc(conteudo)}</pre></body></html>`;
    baixarArquivo(nomeArquivo(relatorio, "doc"), html, "application/msword");
  }

  return (
    <Dialog open onClose={onFechar} title={titulo} description={sub} size="3xl"
      footer={
        <div className="flex w-full flex-wrap items-center gap-2">
          <span className="text-xs text-muted">{conteudo.length} caracteres</span>
          <div className="flex-1" />
          <Button type="button" size="sm" variant="secondary" onClick={txt}>
            <FileDown size={14} className="mr-1" aria-hidden="true" /> Exportar .TXT
          </Button>
          <Button type="button" size="sm" variant="secondary" onClick={doc}>
            <FileDown size={14} className="mr-1" aria-hidden="true" /> Exportar .DOC
          </Button>
          <Button type="button" size="sm" variant="secondary" onClick={() => window.print()}>
            <Printer size={14} className="mr-1" aria-hidden="true" /> Imprimir / PDF
          </Button>
          {!entregue && (
            <Button type="button" size="sm" loading={pending} onClick={() => void salvar()}>
              Salvar
            </Button>
          )}
        </div>
      }>
      <Textarea id="relatorio-conteudo" label="Conteúdo do relatório" rows={18} value={conteudo} readOnly={entregue}
        onChange={(e) => setConteudo(e.target.value)} placeholder="Escreva o relatório aqui…" />
      {entregue && <p className="mt-2 text-xs text-muted">Relatório entregue: o conteúdo não muda mais.</p>}
      <div className="folha-impressao hidden p-8 font-sans text-black print:block">
        <h1 className="border-b-2 border-black pb-2 text-xl font-bold">{titulo}</h1>
        <p className="mb-4 text-sm">{sub}</p>
        <pre className="whitespace-pre-wrap font-sans">{conteudo}</pre>
      </div>
    </Dialog>
  );
}

/** Relatórios escritos: tipo, etapa, prazo, conteúdo e exportação. */
function Escritos({ unidadeId, acolhidos }: { unidadeId: string; acolhidos: AcolhidoResumo[] }) {
  const lista = useApiQuery<RelatorioCasaAbrigo[]>(`v1/sempras/unidades/${unidadeId}/casa-abrigo/relatorios`);
  const [tipo, setTipo] = useState<TipoRelatorio>("mensal");
  const [tipoOutro, setTipoOutro] = useState("");
  const [status, setStatus] = useState<StatusRelatorio>("em_elaboracao");
  const [entrega, setEntrega] = useState("");
  const [acolhimentoId, setAcolhimentoId] = useState("");
  const [editando, setEditando] = useState<RelatorioCasaAbrigo | null>(null);
  const [info, setInfo] = useState<RelatorioCasaAbrigo | null>(null);
  const [infoStatus, setInfoStatus] = useState<StatusRelatorio>("em_elaboracao");
  const [infoData, setInfoData] = useState("");
  const [excluindo, setExcluindo] = useState<RelatorioCasaAbrigo | null>(null);
  const { run, pending } = useAction();
  const recarregar = () => void lista.mutate();

  async function registrar(e: FormEvent) {
    e.preventDefault();
    const ok = await run(() => apiClient.post(`v1/sempras/unidades/${unidadeId}/casa-abrigo/relatorios`, {
      tipo, tipo_outro: tipoOutro.trim(), status, data_entrega: dataISO(entrega), acolhimento_id: acolhimentoId || null, conteudo: "" }),
    "Relatório registrado");
    if (ok) {
      setTipoOutro("");
      setEntrega("");
      recarregar();
    }
  }
  async function salvarInfo(e: FormEvent) {
    e.preventDefault();
    if (!info) return;
    const ok = await run(() => apiClient.put(`v1/sempras/casa-abrigo-relatorios/${info.id}`, {
      acolhimento_id: info.acolhimento_id, tipo: info.tipo, tipo_outro: info.tipo_outro, status: infoStatus,
      data_entrega: dataISO(infoData), conteudo: info.conteudo }), "Relatório atualizado");
    if (ok) {
      setInfo(null);
      recarregar();
    }
  }
  async function excluir() {
    if (!excluindo) return;
    const ok = await run(() => apiClient.delete(`v1/sempras/casa-abrigo-relatorios/${excluindo.id}`), "Relatório excluído");
    if (ok) {
      setExcluindo(null);
      recarregar();
    }
  }
  const hoje = hojeISO();
  const atraso = (r: RelatorioCasaAbrigo) => r.status !== "entregue" && diaDe(r.data_entrega) < hoje;

  return (
    <div className="flex flex-col gap-4">
      <form onSubmit={registrar} className="grid gap-3 rounded-xl border border-surface-border p-4 sm:grid-cols-5 sm:items-end">
        <Select id="relatorio-tipo" label="Tipo do relatório" value={tipo} onChange={(e) => setTipo(e.target.value as TipoRelatorio)}
          options={Object.entries(TIPOS_RELATORIO).map(([value, label]) => ({ value, label }))} />
        {tipo === "outro" && <Input id="relatorio-tipo-outro" label="Nome do relatório *" value={tipoOutro} onChange={(e) => setTipoOutro(e.target.value)} />}
        <Select id="relatorio-etapa" label="Etapa atual" value={status} onChange={(e) => setStatus(e.target.value as StatusRelatorio)}
          options={Object.entries(STATUS_RELATORIO).map(([value, s]) => ({ value, label: s.label }))} />
        <Input id="relatorio-entrega" label="Data de entrega *" type="date" value={entrega} onChange={(e) => setEntrega(e.target.value)} />
        <SeletorAcolhido id="relatorio-acolhido" acolhidos={acolhidos} valor={acolhimentoId} onChange={setAcolhimentoId} label="Acolhido (opcional)" vazio="Nenhum" />
        <Button type="submit" loading={pending} disabled={!entrega || (tipo === "outro" && !tipoOutro.trim())}>
          Registrar
        </Button>
      </form>

      <DataState loading={lista.isLoading} error={lista.error} onRetry={recarregar} empty={(lista.data ?? []).length === 0}
        emptyTitle="Nenhum relatório registrado ainda">
        <ul aria-label="Relatórios escritos" className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
          {(lista.data ?? []).map((r) => (
            <li key={r.id} className="flex flex-wrap items-center justify-between gap-2 p-3 text-sm">
              <span className="flex flex-wrap items-center gap-2">
                <strong>{nomeRelatorio(r)}</strong>
                <Badge tone={STATUS_RELATORIO[r.status].tone}>{STATUS_RELATORIO[r.status].label}</Badge>
                {r.conteudo.trim() ? <Badge tone="success">Escrito ({r.conteudo.length} caracteres)</Badge> : <Badge tone="danger">Sem conteúdo</Badge>}
                {r.acolhido_nome && <span className="text-muted">· {r.acolhido_nome}</span>}
              </span>
              <span className="flex flex-wrap items-center gap-1">
                <span className={atraso(r) ? "font-semibold text-danger" : "text-muted"}>
                  {atraso(r) ? "Atrasado · " : ""}entrega {fmtDia(r.data_entrega)}
                </span>
                <Button type="button" size="sm" variant="secondary" onClick={() => setEditando(r)}>
                  <Pencil size={14} className="mr-1" aria-hidden="true" /> {r.status === "entregue" ? "Ler" : "Escrever"}
                </Button>
                {r.status !== "entregue" && (
                  <>
                    <Button type="button" size="sm" variant="ghost" aria-label={`Editar etapa e prazo de ${nomeRelatorio(r)}`}
                      onClick={() => { setInfo(r); setInfoStatus(r.status); setInfoData(diaDe(r.data_entrega)); }}>
                      <Settings2 size={14} aria-hidden="true" />
                    </Button>
                    <Button type="button" size="sm" variant="ghost" aria-label={`Excluir ${nomeRelatorio(r)}`} onClick={() => setExcluindo(r)}>
                      <Trash2 size={14} aria-hidden="true" />
                    </Button>
                  </>
                )}
              </span>
            </li>
          ))}
        </ul>
      </DataState>

      {editando && <Editor relatorio={editando} onFechar={() => setEditando(null)} onSalvo={() => { setEditando(null); recarregar(); }} />}

      <Dialog open={info !== null} onClose={() => setInfo(null)} title="Editar etapa e prazo">
        <form onSubmit={salvarInfo} className="flex flex-col gap-3">
          <Select id="relatorio-info-etapa" label="Etapa do relatório" value={infoStatus} onChange={(e) => setInfoStatus(e.target.value as StatusRelatorio)}
            options={Object.entries(STATUS_RELATORIO).map(([value, s]) => ({ value, label: s.label }))} />
          <Input id="relatorio-info-data" label="Data de entrega" type="date" value={infoData} onChange={(e) => setInfoData(e.target.value)} />
          {infoStatus === "entregue" && <p className="text-xs text-warning">Depois de entregue, o relatório não muda nem pode ser excluído.</p>}
          <div>
            <Button type="submit" loading={pending} disabled={!infoData}>
              Salvar alterações
            </Button>
          </div>
        </form>
      </Dialog>

      <Dialog open={excluindo !== null} onClose={() => setExcluindo(null)} title="Confirmar exclusão"
        description="O conteúdo escrito será perdido para sempre.">
        <Button type="button" variant="danger" loading={pending} onClick={() => void excluir()}>
          Excluir relatório
        </Button>
      </Dialog>
    </div>
  );
}

/** Relatórios da Casa Abrigo: estatístico por período e relatórios escritos. */
export function RelatoriosAba({ unidadeId, acolhidos }: { unidadeId: string; acolhidos: AcolhidoResumo[] }) {
  const [aba, setAba] = useState<"estatistico" | "escritos">("estatistico");
  return (
    <div className="flex flex-col gap-4">
      <Abas rotulo="Relatórios" abas={[{ key: "estatistico", label: "Estatístico" }, { key: "escritos", label: "Relatórios escritos" }]}
        atual={aba} onChange={setAba} />
      {aba === "estatistico" ? <Estatistico unidadeId={unidadeId} /> : <Escritos unidadeId={unidadeId} acolhidos={acolhidos} />}
    </div>
  );
}
