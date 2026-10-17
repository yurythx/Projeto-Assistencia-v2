"use client";

import { useEffect, useRef, useState, type FormEvent, type PointerEvent } from "react";
import { Eraser, Printer } from "lucide-react";

import { DataState } from "@/components/nexus/DataState";
import { useAction } from "@/components/nexus/useAction";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Table, TableBody, TableHead, TableRow } from "@/components/ui/Table";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import {
  TIPOS_TERMO,
  TITULOS_TERMO,
  dataISO,
  fmtDia,
  hojeISO,
  iniciais,
  type AcolhidoResumo,
  type Termo,
  type TipoTermo,
} from "@/lib/sempras/types";

import { SeletorAcolhido } from "./campos";

/** O documento oficial que vai para a impressão (com a assinatura da tela). */
interface Documento {
  tipo: TipoTermo;
  acolhido: string;
  responsavel: string;
  parentesco: string;
  texto: string;
  data: string;
  assinatura: string;
}

/** Quadro de assinatura (mouse, caneta ou dedo). A assinatura não é
 * guardada — só vai para o documento impresso (ADR 032 §3). */
function QuadroAssinatura({ onChange }: { onChange: (dataUrl: string) => void }) {
  const ref = useRef<HTMLCanvasElement>(null);
  const desenhando = useRef(false);
  const ponto = (e: PointerEvent<HTMLCanvasElement>) => {
    const r = e.currentTarget.getBoundingClientRect();
    return [e.clientX - r.left, e.clientY - r.top] as const;
  };
  const ctx = () => ref.current?.getContext("2d") ?? null;
  function inicio(e: PointerEvent<HTMLCanvasElement>) {
    desenhando.current = true;
    const c = ctx();
    if (!c) return;
    c.lineWidth = 2;
    c.lineCap = "round";
    c.strokeStyle = "#000";
    c.beginPath();
    c.moveTo(...ponto(e));
  }
  function traco(e: PointerEvent<HTMLCanvasElement>) {
    if (!desenhando.current) return;
    const c = ctx();
    if (!c) return;
    c.lineTo(...ponto(e));
    c.stroke();
  }
  function fim() {
    if (!desenhando.current) return;
    desenhando.current = false;
    onChange(ref.current?.toDataURL() ?? "");
  }
  function limpar() {
    const c = ctx();
    if (c && ref.current) c.clearRect(0, 0, ref.current.width, ref.current.height);
    onChange("");
  }
  return (
    <div className="flex flex-col items-center gap-2">
      <span className="text-sm font-medium">Assinatura do responsável (mouse ou toque)</span>
      <canvas
        ref={ref}
        width={500}
        height={150}
        aria-label="Quadro de assinatura"
        className="max-w-full touch-none rounded-lg border border-dashed border-surface-border bg-white"
        onPointerDown={inicio}
        onPointerMove={traco}
        onPointerUp={fim}
        onPointerLeave={fim}
      />
      <Button type="button" size="sm" variant="ghost" onClick={limpar}>
        <Eraser size={14} className="mr-1" aria-hidden="true" /> Limpar quadro
      </Button>
    </div>
  );
}

/** Folha impressa: cabeçalho da Prefeitura, termo e assinatura. */
function FolhaTermo({ doc }: { doc: Documento }) {
  return (
    <div className="folha-impressao hidden flex-col items-center gap-4 p-8 text-center font-sans text-black print:flex">
      <p className="text-lg font-bold">Prefeitura Municipal de Rondonópolis</p>
      <p>Secretaria de Promoção e Assistência Social — Casa Abrigo</p>
      <hr className="w-full border-black" />
      <h1 className="my-4 text-xl font-bold uppercase">{TITULOS_TERMO[doc.tipo]}</h1>
      <p className="text-justify leading-relaxed">
        Declaramos, para os devidos fins de direito, referente ao(à) acolhido(a) <strong>{doc.acolhido}</strong>
        {doc.responsavel && (
          <>
            , que a retirada fica sob responsabilidade de <strong>{doc.responsavel}</strong>
            {doc.parentesco && ` (${doc.parentesco})`}
          </>
        )}
        .
      </p>
      {doc.texto && (
        <p className="w-full text-justify">
          <strong>Motivo/descrição:</strong> {doc.texto}
        </p>
      )}
      <div className="mt-10 flex flex-col items-center">
        {doc.assinatura ? (
          // eslint-disable-next-line @next/next/no-img-element -- imagem gerada no navegador (data URL), não otimizável
          <img src={doc.assinatura} alt="Assinatura do responsável" className="w-72 border-b border-black" />
        ) : (
          <div className="h-16 w-72 border-b border-black" />
        )}
        <span>Assinatura do responsável</span>
      </div>
      <p className="mt-6">Rondonópolis, {fmtDia(doc.data)}.</p>
    </div>
  );
}

/**
 * Gerador de termos oficiais (advertência, passeio, saída): registra o
 * termo, monta o documento com o cabeçalho da Prefeitura e a assinatura
 * feita na tela e manda imprimir. O histórico mostra só as iniciais
 * (ECA art. 143), como o 4.0.
 */
export function TermosAba({ unidadeId, acolhidos }: { unidadeId: string; acolhidos: AcolhidoResumo[] }) {
  const lista = useApiQuery<Termo[]>(`v1/sempras/unidades/${unidadeId}/casa-abrigo/termos`);
  const [tipo, setTipo] = useState<TipoTermo>("advertencia");
  const [acolhimentoId, setAcolhimentoId] = useState("");
  const [responsavel, setResponsavel] = useState("");
  const [parentesco, setParentesco] = useState("");
  const [texto, setTexto] = useState("");
  const [data, setData] = useState(hojeISO());
  const [assinatura, setAssinatura] = useState("");
  const [doc, setDoc] = useState<Documento | null>(null);
  const { run, pending } = useAction();
  const escolhido = acolhidos.find((a) => a.acolhimento_id === acolhimentoId);
  const comRetirada = tipo === "passeio" || tipo === "saida";

  useEffect(() => {
    if (doc) window.print();
  }, [doc]);

  async function gerar(e: FormEvent) {
    e.preventDefault();
    if (!escolhido) return;
    const ok = await run(
      () => apiClient.post(`v1/sempras/casa-abrigo/${acolhimentoId}/termos`, {
        tipo_termo: tipo, responsavel_retirada: comRetirada ? responsavel.trim() : "", parentesco_responsavel: comRetirada ? parentesco.trim() : "",
        motivo_observacao: texto.trim(), data_termo: dataISO(data),
      }),
      "Termo registrado",
    );
    if (ok) {
      setDoc({ tipo, acolhido: escolhido.nome_social || escolhido.nome, responsavel: comRetirada ? responsavel.trim() : "",
        parentesco: comRetirada ? parentesco.trim() : "", texto: texto.trim(), data, assinatura });
      setTexto("");
      void lista.mutate();
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <form onSubmit={gerar} className="flex flex-col gap-4 rounded-xl border border-surface-border p-4 print:hidden">
        <h2 className="text-base font-semibold">Gerador de termos oficiais</h2>
        <div className="grid gap-3 sm:grid-cols-3">
          <Select id="termo-tipo" label="Tipo de documento" value={tipo} onChange={(e) => setTipo(e.target.value as TipoTermo)}
            options={Object.entries(TITULOS_TERMO).map(([value, label]) => ({ value, label }))} />
          <SeletorAcolhido id="termo-acolhido" acolhidos={acolhidos} valor={acolhimentoId} onChange={setAcolhimentoId} />
          <Input id="termo-data" label="Data" type="date" value={data} onChange={(e) => setData(e.target.value)} />
          {comRetirada && (
            <>
              <Input id="termo-responsavel" label="Responsável pela retirada" value={responsavel} onChange={(e) => setResponsavel(e.target.value)} />
              <Input id="termo-parentesco" label="Parentesco" value={parentesco} onChange={(e) => setParentesco(e.target.value)} />
            </>
          )}
        </div>
        <Textarea id="termo-texto" label="Descrição / motivo / observações" rows={3} value={texto} onChange={(e) => setTexto(e.target.value)} />
        <QuadroAssinatura onChange={setAssinatura} />
        <div className="flex justify-end">
          <Button type="submit" loading={pending} disabled={!escolhido || !data}>
            <Printer size={14} className="mr-1" aria-hidden="true" /> Salvar e gerar documento
          </Button>
        </div>
      </form>

      {doc && <FolhaTermo doc={doc} />}

      <section aria-label="Histórico de termos" className="flex flex-col gap-3 print:hidden">
        <h2 className="text-base font-semibold">Histórico de termos gerados</h2>
        <DataState loading={lista.isLoading} error={lista.error} onRetry={() => void lista.mutate()} empty={(lista.data ?? []).length === 0}
          emptyTitle="Nenhum termo gerado ainda">
          <Table caption="Termos gerados">
            <TableHead>
              <TableRow>
                <th className="px-3 py-2">Data</th>
                <th className="px-3 py-2">Tipo</th>
                <th className="px-3 py-2">Acolhido</th>
                <th className="px-3 py-2">Observação</th>
              </TableRow>
            </TableHead>
            <TableBody>
              {(lista.data ?? []).map((t) => (
                <TableRow key={t.id}>
                  <td className="px-3 py-2">{fmtDia(t.data_termo)}</td>
                  <td className="px-3 py-2">{TIPOS_TERMO[t.tipo_termo]}</td>
                  <td className="px-3 py-2">{iniciais(t.acolhido_nome ?? "")}</td>
                  <td className="px-3 py-2">{t.motivo_observacao || "—"}</td>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </DataState>
      </section>
    </div>
  );
}
