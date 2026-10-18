"use client";

import Link from "next/link";
import { useState, type FormEvent } from "react";
import { Download, FileText, History, Pencil, Plus, Printer, UserMinus } from "lucide-react";

import { useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Table, TableBody, TableHead, TableRow } from "@/components/ui/Table";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import {
  TIPOS_DESLIGAMENTO,
  baixarArquivo,
  dataISO,
  fmtDia,
  hojeISO,
  linhaCSV,
  type AcolhidoResumo,
  type PainelCasaAbrigo,
  type TipoDesligamento,
} from "@/lib/sempras/types";

import { LinhaDoTempo } from "./LinhaDoTempo";

const vazio = (v: string) => !v.trim();

/**
 * Relatório de institucionalizados (o do sistema da unidade): quem está
 * acolhido, idade, entrada, processo e guia (em destaque quando faltam),
 * documentos, totais por faixa etária, impressão e CSV — mais o cadastro,
 * a edição, a linha do tempo e o desligamento (ação própria da equipe técnica).
 */
export function Institucionalizados({
  unidadeId,
  painel,
  onNovo,
  onEditar,
  onChanged,
}: {
  unidadeId: string;
  painel: PainelCasaAbrigo;
  onNovo: () => void;
  onEditar: (acolhimentoId: string) => void;
  onChanged: () => void;
}) {
  const [filtro, setFiltro] = useState("");
  const [verDesligados, setVerDesligados] = useState(false);
  const lista = (verDesligados ? painel.desligados : painel.acolhidos).filter((a) =>
    `${a.nome} ${a.nome_social}`.toLowerCase().includes(filtro.trim().toLowerCase()),
  );

  const [desligando, setDesligando] = useState<AcolhidoResumo | null>(null);
  const [historia, setHistoria] = useState<AcolhidoResumo | null>(null);
  const [tipo, setTipo] = useState<TipoDesligamento>("reintegracao_familia_natural");
  const [motivo, setMotivo] = useState("");
  const [dataSaida, setDataSaida] = useState(hojeISO());
  const [guia, setGuia] = useState("");
  const [dataGuia, setDataGuia] = useState("");
  const { run, pending } = useAction();

  async function desligar(e: FormEvent) {
    e.preventDefault();
    if (!desligando) return;
    const ok = await run(
      () => apiClient.post(`v1/sempras/casa-abrigo/${desligando.acolhimento_id}/desligar`, {
        tipo, motivo: motivo.trim(), data_saida: dataISO(dataSaida), guia: guia.trim(), data_guia: dataGuia ? dataISO(dataGuia) : null,
      }),
      "Acolhimento desligado",
    );
    if (ok) {
      setDesligando(null);
      setMotivo("");
      setGuia("");
      setDataGuia("");
      onChanged();
    }
  }

  function exportarCSV() {
    const linhas = [
      linhaCSV(["Nome completo", "Idade", "Nascimento", "Data de entrada", "Processo", "Guia de acolhimento"]),
      ...lista.map((a) =>
        linhaCSV([a.nome_social || a.nome, a.idade, fmtDia(a.data_nascimento), fmtDia(a.data_entrada), a.processo_judicial || "N/I",
          a.guia_acolhimento || "N/I"]),
      ),
    ];
    baixarArquivo(`institucionalizados_${hojeISO()}.csv`, linhas.join("\r\n"), "text/csv;charset=utf-8;");
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-3 print:hidden">
        <div className="min-w-64 flex-1">
          <Input id="institucionalizados-filtro" label="Buscar por nome" value={filtro} onChange={(e) => setFiltro(e.target.value)} />
        </div>
        <div className="flex flex-wrap gap-2">
          <Button type="button" size="sm" variant="secondary" onClick={() => setVerDesligados(!verDesligados)}>
            {verDesligados ? "Ver acolhidos" : `Ver desligados (${painel.desligados.length})`}
          </Button>
          <Button type="button" size="sm" variant="secondary" onClick={exportarCSV} disabled={lista.length === 0}>
            <Download size={14} className="mr-1" aria-hidden="true" /> Exportar CSV
          </Button>
          <Button type="button" size="sm" variant="secondary" onClick={() => window.print()}>
            <Printer size={14} className="mr-1" aria-hidden="true" /> Imprimir / PDF
          </Button>
          <Button type="button" size="sm" onClick={onNovo}>
            <Plus size={14} className="mr-1" aria-hidden="true" /> Novo acolhimento
          </Button>
        </div>
      </div>

      <Table caption={verDesligados ? "Desligados da unidade" : "Relatório de institucionalizados"} captionVisible>
        <TableHead>
          <TableRow>
            <th className="px-3 py-2">Nome completo</th>
            <th className="px-3 py-2">Idade</th>
            <th className="px-3 py-2">Nascimento</th>
            <th className="px-3 py-2">Entrada</th>
            <th className="px-3 py-2">{verDesligados ? "Saída" : "Processo"}</th>
            <th className="px-3 py-2">{verDesligados ? "Tipo" : "Guia"}</th>
            <th className="px-3 py-2 print:hidden">Ações</th>
          </TableRow>
        </TableHead>
        <TableBody>
          {lista.length === 0 && (
            <TableRow>
              <td colSpan={7} className="px-3 py-6 text-center text-muted">
                Ninguém nesta lista.
              </td>
            </TableRow>
          )}
          {lista.map((a) => (
            <TableRow key={a.acolhimento_id}>
              <td className="px-3 py-2 font-medium">
                <Link href={`/sempras/cidadaos/${a.cidadao_id}?unidade=${unidadeId}`} className="text-primary underline">
                  {a.nome_social || a.nome}
                </Link>
              </td>
              <td className="px-3 py-2">{a.idade} anos</td>
              <td className="px-3 py-2">{fmtDia(a.data_nascimento)}</td>
              <td className="px-3 py-2">{fmtDia(a.data_entrada)}</td>
              {verDesligados ? (
                <>
                  <td className="px-3 py-2">{fmtDia(a.data_desligamento)}</td>
                  <td className="px-3 py-2">{a.desligamento_tipo ? TIPOS_DESLIGAMENTO[a.desligamento_tipo] : "—"}</td>
                </>
              ) : (
                <>
                  <td className={`px-3 py-2 ${vazio(a.processo_judicial) ? "font-semibold text-warning" : ""}`}>{a.processo_judicial || "N/I"}</td>
                  <td className={`px-3 py-2 ${vazio(a.guia_acolhimento) ? "font-semibold text-warning" : ""}`}>{a.guia_acolhimento || "N/I"}</td>
                </>
              )}
              <td className="px-3 py-2 print:hidden">
                <span className="flex flex-wrap items-center gap-1">
                  {a.documento_acolhido && <Badge tone="success"><FileText size={12} className="mr-0.5 inline" aria-hidden="true" />Ac.</Badge>}
                  {a.documento_responsavel && <Badge tone="success"><FileText size={12} className="mr-0.5 inline" aria-hidden="true" />Resp.</Badge>}
                  <Button type="button" size="sm" variant="ghost" aria-label={`Linha do tempo de ${a.nome}`} onClick={() => setHistoria(a)}>
                    <History size={14} aria-hidden="true" />
                  </Button>
                  <Button type="button" size="sm" variant="ghost" aria-label={`Editar ${a.nome}`} onClick={() => onEditar(a.acolhimento_id)}>
                    <Pencil size={14} aria-hidden="true" />
                  </Button>
                  {a.status === "acolhido" && (
                    <Button type="button" size="sm" variant="ghost" aria-label={`Desligar ${a.nome}`} onClick={() => setDesligando(a)}>
                      <UserMinus size={14} aria-hidden="true" />
                    </Button>
                  )}
                </span>
              </td>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      {!verDesligados && (
        <dl className="grid grid-cols-3 gap-3 rounded-xl border border-dashed border-primary/40 bg-primary/5 p-4 text-center">
          <div>
            <dt className="text-xs font-semibold uppercase text-muted">Total institucionalizados</dt>
            <dd className="text-2xl font-bold">{String(painel.ativos).padStart(2, "0")}</dd>
          </div>
          <div>
            <dt className="text-xs font-semibold uppercase text-muted">Crianças (0 a 11 anos)</dt>
            <dd className="text-2xl font-bold">{String(painel.criancas).padStart(2, "0")}</dd>
          </div>
          <div>
            <dt className="text-xs font-semibold uppercase text-muted">Adolescentes (12 a 17 anos)</dt>
            <dd className="text-2xl font-bold">{String(painel.adolescentes).padStart(2, "0")}</dd>
          </div>
        </dl>
      )}

      {historia && <LinhaDoTempo acolhido={historia} onFechar={() => setHistoria(null)} />}

      <Dialog open={desligando !== null} onClose={() => setDesligando(null)} title="Desligar acolhimento"
        description={desligando ? `${desligando.nome_social || desligando.nome} — só a equipe técnica desliga.` : undefined}>
        <form onSubmit={desligar} className="flex flex-col gap-3">
          <Select id="desligar-tipo" label="Tipo de desligamento" value={tipo} onChange={(e) => setTipo(e.target.value as TipoDesligamento)}
            options={Object.entries(TIPOS_DESLIGAMENTO).map(([value, label]) => ({ value, label }))} />
          <Textarea id="desligar-motivo" label="Motivo *" rows={3} value={motivo} onChange={(e) => setMotivo(e.target.value)} required />
          <div className="grid gap-3 sm:grid-cols-3">
            <Input id="desligar-data" label="Data da saída *" type="date" max={hojeISO()} value={dataSaida} onChange={(e) => setDataSaida(e.target.value)} required />
            <Input id="desligar-guia" label="Nº da guia de desligamento" value={guia} onChange={(e) => setGuia(e.target.value)} />
            <Input id="desligar-data-guia" label="Data da guia" type="date" value={dataGuia} onChange={(e) => setDataGuia(e.target.value)} />
          </div>
          <div>
            <Button type="submit" variant="danger" loading={pending} disabled={motivo.trim().length < 3 || !dataSaida}>
              Confirmar desligamento
            </Button>
          </div>
        </form>
      </Dialog>
    </div>
  );
}
