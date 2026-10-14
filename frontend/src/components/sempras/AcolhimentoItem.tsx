"use client";

import { useRef, useState, type FormEvent } from "react";

import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { mimeOf, putToTicket } from "@/lib/nexus/upload";
import type { UploadTicket } from "@/lib/nexus/types";
import {
  CATEGORIAS_DIAGNOSTICO,
  MOTIVOS_ACOLHIMENTO,
  STATUS_ACOLHIMENTO,
  TIPOS_DOCUMENTO_CASA_ABRIGO,
  TIPOS_PIA_META,
  TIPOS_TERMO,
  type Acolhimento,
  type CategoriaDiagnostico,
  type DocumentoCasaAbrigo,
  type TipoDocumentoCasaAbrigo,
  type TipoPiaMeta,
  type TipoTermo,
} from "@/lib/sempras/types";

function hoje(): string {
  return new Date().toISOString().slice(0, 10);
}

/**
 * Um acolhimento institucional da Casa Abrigo — com os irmãos e os
 * diagnósticos já aninhados (vêm no mesmo prontuário). O desligamento é
 * uma ação própria, nunca a edição geral (RN-14 do legado de referência:
 * lá, dava pra desligar pelo PUT comum, sem passar pela equipe técnica).
 */
export function AcolhimentoItem({ acolhimento, nomeUnidade, onChanged }: { acolhimento: Acolhimento; nomeUnidade: string; onChanged: () => void }) {
  // Identificador legível para rótulos de lista/aria — nunca o UUID cru
  // (ruim de ouvir num leitor de tela).
  const referencia = `acolhimento de ${fmtDateTime(acolhimento.data_entrada)}`;
  const { run: runDesligar, pending: pendingDesligar } = useAction();
  const [desligando, setDesligando] = useState(false);
  const [motivoDesligamento, setMotivoDesligamento] = useState("");

  const { run: runIrmao, pending: pendingIrmao } = useAction();
  const [nomeIrmao, setNomeIrmao] = useState("");
  const [irmaoAcolhido, setIrmaoAcolhido] = useState(false);
  const { run: runRemoverIrmao } = useAction();

  const { run: runDiagnostico, pending: pendingDiagnostico } = useAction();
  const [categoriaDiagnostico, setCategoriaDiagnostico] = useState<CategoriaDiagnostico>("doenca");
  const [textoDiagnostico, setTextoDiagnostico] = useState("");
  const { run: runRemoverDiagnostico } = useAction();

  async function confirmarDesligamento(e: FormEvent) {
    e.preventDefault();
    const ok = await runDesligar(
      () => apiClient.post(`v1/sempras/casa-abrigo/${acolhimento.id}/desligar`, { motivo: motivoDesligamento.trim() }),
      "Acolhimento desligado",
    );
    if (ok) {
      setDesligando(false);
      setMotivoDesligamento("");
      onChanged();
    }
  }

  async function adicionarIrmao(e: FormEvent) {
    e.preventDefault();
    const ok = await runIrmao(
      () => apiClient.post(`v1/sempras/casa-abrigo/${acolhimento.id}/irmaos`, { nome: nomeIrmao.trim(), esta_acolhido: irmaoAcolhido }),
      "Irmão adicionado",
    );
    if (ok) {
      setNomeIrmao("");
      setIrmaoAcolhido(false);
      onChanged();
    }
  }

  async function removerIrmao(id: string) {
    const ok = await runRemoverIrmao(() => apiClient.delete(`v1/sempras/casa-abrigo-irmaos/${id}`), "Irmão removido");
    if (ok) onChanged();
  }

  async function adicionarDiagnostico(e: FormEvent) {
    e.preventDefault();
    const ok = await runDiagnostico(
      () => apiClient.post(`v1/sempras/casa-abrigo/${acolhimento.id}/diagnosticos`, { categoria: categoriaDiagnostico, diagnostico: textoDiagnostico.trim() }),
      "Diagnóstico adicionado",
    );
    if (ok) {
      setTextoDiagnostico("");
      onChanged();
    }
  }

  async function removerDiagnostico(id: string) {
    const ok = await runRemoverDiagnostico(() => apiClient.delete(`v1/sempras/casa-abrigo-diagnosticos/${id}`), "Diagnóstico removido");
    if (ok) onChanged();
  }

  const { run: runVisitante, pending: pendingVisitante } = useAction();
  const [nomeVisitante, setNomeVisitante] = useState("");
  const [dataVisita, setDataVisita] = useState(hoje);
  const { run: runRemoverVisitante } = useAction();

  async function adicionarVisitante(e: FormEvent) {
    e.preventDefault();
    const ok = await runVisitante(
      () => apiClient.post(`v1/sempras/casa-abrigo/${acolhimento.id}/visitantes`, {
        nome_visitante: nomeVisitante.trim(), data_visita: new Date(dataVisita).toISOString(),
      }),
      "Visita registrada",
    );
    if (ok) {
      setNomeVisitante("");
      onChanged();
    }
  }

  async function removerVisitante(id: string) {
    const ok = await runRemoverVisitante(() => apiClient.delete(`v1/sempras/casa-abrigo-visitantes/${id}`), "Visita removida");
    if (ok) onChanged();
  }

  const { run: runTermo, pending: pendingTermo } = useAction();
  const [tipoTermo, setTipoTermo] = useState<TipoTermo>("passeio");
  const [dataTermo, setDataTermo] = useState(hoje);
  const { run: runRemoverTermo } = useAction();

  async function registrarTermo(e: FormEvent) {
    e.preventDefault();
    const ok = await runTermo(
      () => apiClient.post(`v1/sempras/casa-abrigo/${acolhimento.id}/termos`, {
        tipo_termo: tipoTermo, data_termo: new Date(dataTermo).toISOString(),
      }),
      "Termo registrado",
    );
    if (ok) onChanged();
  }

  async function removerTermo(id: string) {
    const ok = await runRemoverTermo(() => apiClient.delete(`v1/sempras/casa-abrigo-termos/${id}`), "Termo removido");
    if (ok) onChanged();
  }

  const { run: runPia, pending: pendingPia } = useAction();
  const [escolaPia, setEscolaPia] = useState(acolhimento.pia?.escola ?? "");
  const [vacinacaoPia, setVacinacaoPia] = useState(acolhimento.pia?.vacinacao_atualizada ?? false);

  async function salvarPia(e: FormEvent) {
    e.preventDefault();
    const ok = await runPia(
      () => apiClient.put(`v1/sempras/casa-abrigo/${acolhimento.id}/pia`, {
        escola: escolaPia.trim(), vacinacao_atualizada: vacinacaoPia,
      }),
      "PIA atualizado",
    );
    if (ok) onChanged();
  }

  const { run: runPiaMeta, pending: pendingPiaMeta } = useAction();
  const [tipoPiaMeta, setTipoPiaMeta] = useState<TipoPiaMeta>("plano");
  const [descricaoPiaMeta, setDescricaoPiaMeta] = useState("");
  const { run: runConcluirPiaMeta } = useAction();
  const { run: runRemoverPiaMeta } = useAction();

  async function adicionarPiaMeta(e: FormEvent) {
    e.preventDefault();
    const ok = await runPiaMeta(
      () => apiClient.post(`v1/sempras/casa-abrigo/${acolhimento.id}/pia-metas`, { tipo: tipoPiaMeta, descricao: descricaoPiaMeta.trim() }),
      "Meta do PIA adicionada",
    );
    if (ok) {
      setDescricaoPiaMeta("");
      onChanged();
    }
  }

  async function alternarConclusaoPiaMeta(id: string, concluida: boolean) {
    const ok = await runConcluirPiaMeta(
      () => apiClient.post(`v1/sempras/casa-abrigo-pia-metas/${id}/concluir`, { concluida: !concluida }),
      !concluida ? "Meta concluída" : "Meta reaberta",
    );
    if (ok) onChanged();
  }

  async function removerPiaMeta(id: string) {
    const ok = await runRemoverPiaMeta(() => apiClient.delete(`v1/sempras/casa-abrigo-pia-metas/${id}`), "Meta removida");
    if (ok) onChanged();
  }

  const { run: runDocumento } = useAction();
  const [tipoDocumento, setTipoDocumento] = useState<TipoDocumentoCasaAbrigo>("acolhido");
  const [enviando, setEnviando] = useState(false);
  const { run: runRemoverDocumento } = useAction();
  const fileInput = useRef<HTMLInputElement>(null);

  async function enviarDocumento(file: File) {
    setEnviando(true);
    await runDocumento(async () => {
      const { data } = await apiClient.post<{ documento: DocumentoCasaAbrigo; upload: UploadTicket }>(
        `v1/sempras/casa-abrigo/${acolhimento.id}/documentos/uploads`,
        { tipo: tipoDocumento, filename: file.name, content_type: mimeOf(file), size: file.size },
      );
      await putToTicket(data.upload, file);
      await apiClient.post(`v1/sempras/casa-abrigo-documentos/${data.documento.id}/confirm`);
    }, "Documento enviado");
    setEnviando(false);
    onChanged();
  }

  async function baixarDocumento(id: string) {
    const res = await runDocumento(() => apiClient.get<{ url: string }>(`v1/sempras/casa-abrigo-documentos/${id}/download`));
    if (res) window.location.assign(res.data.url);
  }

  async function removerDocumento(id: string) {
    const ok = await runRemoverDocumento(() => apiClient.delete(`v1/sempras/casa-abrigo-documentos/${id}`), "Documento removido");
    if (ok) onChanged();
  }

  return (
    <li className="flex flex-col gap-3 p-3 text-sm">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="font-medium">{acolhimento.motivos.map((m) => MOTIVOS_ACOLHIMENTO[m]).join(", ")}</p>
          <p className="text-xs text-muted">
            {nomeUnidade} · desde {fmtDateTime(acolhimento.data_entrada)}
            {acolhimento.motivo_desligamento && ` · ${acolhimento.motivo_desligamento}`}
          </p>
        </div>
        <span className="flex items-center gap-2">
          <Badge tone={STATUS_ACOLHIMENTO[acolhimento.status].tone}>{STATUS_ACOLHIMENTO[acolhimento.status].label}</Badge>
          {acolhimento.status === "acolhido" && (
            <Button size="sm" variant="danger" onClick={() => setDesligando(true)}>
              Desligar
            </Button>
          )}
        </span>
      </div>

      <div className="rounded-lg border border-surface-border p-3">
        <h3 className="text-xs font-medium text-muted">Irmãos</h3>
        {acolhimento.irmaos.length > 0 && (
          <ul aria-label={`Irmãos do ${referencia}`} className="mt-2 flex flex-col gap-1">
            {acolhimento.irmaos.map((irmao) => (
              <li key={irmao.id} className="flex items-center justify-between gap-2">
                <span>
                  {irmao.nome} {irmao.esta_acolhido && <Badge tone="warning">Acolhido</Badge>}
                </span>
                <Button size="sm" variant="ghost" aria-label={`Remover ${irmao.nome}`} onClick={() => void removerIrmao(irmao.id)}>
                  Remover
                </Button>
              </li>
            ))}
          </ul>
        )}
        <form onSubmit={adicionarIrmao} className="mt-2 grid gap-2 sm:grid-cols-[1fr_auto_auto] sm:items-end">
          <Input id={`irmao-nome-${acolhimento.id}`} label="Nome do irmão" maxLength={200} value={nomeIrmao}
            onChange={(e) => setNomeIrmao(e.target.value)} autoComplete="off" />
          <label className="flex items-center gap-2 pb-2">
            <input type="checkbox" checked={irmaoAcolhido} onChange={(e) => setIrmaoAcolhido(e.target.checked)} /> Também acolhido
          </label>
          <Button type="submit" size="sm" loading={pendingIrmao} disabled={nomeIrmao.trim().length < 2}>
            Adicionar irmão
          </Button>
        </form>
      </div>

      <div className="rounded-lg border border-surface-border p-3">
        <h3 className="text-xs font-medium text-muted">Diagnósticos</h3>
        {acolhimento.diagnosticos.length > 0 && (
          <ul aria-label={`Diagnósticos do ${referencia}`} className="mt-2 flex flex-col gap-1">
            {acolhimento.diagnosticos.map((d) => (
              <li key={d.id} className="flex items-center justify-between gap-2">
                <span>
                  {CATEGORIAS_DIAGNOSTICO[d.categoria]}: {d.diagnostico}
                </span>
                <Button size="sm" variant="ghost" aria-label={`Remover diagnóstico ${d.diagnostico}`} onClick={() => void removerDiagnostico(d.id)}>
                  Remover
                </Button>
              </li>
            ))}
          </ul>
        )}
        <form onSubmit={adicionarDiagnostico} className="mt-2 grid gap-2 sm:grid-cols-[1fr_2fr_auto] sm:items-end">
          <Select id={`diagnostico-categoria-${acolhimento.id}`} label="Categoria" value={categoriaDiagnostico}
            onChange={(e) => setCategoriaDiagnostico(e.target.value as CategoriaDiagnostico)}
            options={Object.entries(CATEGORIAS_DIAGNOSTICO).map(([value, label]) => ({ value, label }))} />
          <Input id={`diagnostico-texto-${acolhimento.id}`} label="Diagnóstico" maxLength={255} value={textoDiagnostico}
            onChange={(e) => setTextoDiagnostico(e.target.value)} autoComplete="off" />
          <Button type="submit" size="sm" loading={pendingDiagnostico} disabled={textoDiagnostico.trim().length < 2}>
            Adicionar diagnóstico
          </Button>
        </form>
      </div>

      <div className="rounded-lg border border-surface-border p-3">
        <h3 className="text-xs font-medium text-muted">Visitantes</h3>
        {acolhimento.visitantes.length > 0 && (
          <ul aria-label={`Visitantes do ${referencia}`} className="mt-2 flex flex-col gap-1">
            {acolhimento.visitantes.map((v) => (
              <li key={v.id} className="flex items-center justify-between gap-2">
                <span>{v.nome_visitante} · {fmtDateTime(v.data_visita)}</span>
                <Button size="sm" variant="ghost" aria-label={`Remover visita de ${v.nome_visitante}`} onClick={() => void removerVisitante(v.id)}>
                  Remover
                </Button>
              </li>
            ))}
          </ul>
        )}
        <form onSubmit={adicionarVisitante} className="mt-2 grid gap-2 sm:grid-cols-[1fr_auto_auto] sm:items-end">
          <Input id={`visitante-nome-${acolhimento.id}`} label="Nome do visitante" maxLength={200} value={nomeVisitante}
            onChange={(e) => setNomeVisitante(e.target.value)} autoComplete="off" />
          <Input id={`visitante-data-${acolhimento.id}`} label="Data da visita" type="date" value={dataVisita}
            onChange={(e) => setDataVisita(e.target.value)} />
          <Button type="submit" size="sm" loading={pendingVisitante} disabled={nomeVisitante.trim().length < 2}>
            Anotar visita
          </Button>
        </form>
      </div>

      <div className="rounded-lg border border-surface-border p-3">
        <h3 className="text-xs font-medium text-muted">Termos (advertência, passeio, saída)</h3>
        {acolhimento.termos.length > 0 && (
          <ul aria-label={`Termos do ${referencia}`} className="mt-2 flex flex-col gap-1">
            {acolhimento.termos.map((t) => (
              <li key={t.id} className="flex items-center justify-between gap-2">
                <span>{TIPOS_TERMO[t.tipo_termo]} · {fmtDateTime(t.data_termo)}</span>
                <Button size="sm" variant="ghost" aria-label={`Remover termo de ${TIPOS_TERMO[t.tipo_termo]}`} onClick={() => void removerTermo(t.id)}>
                  Remover
                </Button>
              </li>
            ))}
          </ul>
        )}
        <form onSubmit={registrarTermo} className="mt-2 grid gap-2 sm:grid-cols-[1fr_auto_auto] sm:items-end">
          <Select id={`termo-tipo-${acolhimento.id}`} label="Tipo de termo" value={tipoTermo}
            onChange={(e) => setTipoTermo(e.target.value as TipoTermo)}
            options={Object.entries(TIPOS_TERMO).map(([value, label]) => ({ value, label }))} />
          <Input id={`termo-data-${acolhimento.id}`} label="Data do termo" type="date" value={dataTermo}
            onChange={(e) => setDataTermo(e.target.value)} />
          <Button type="submit" size="sm" loading={pendingTermo}>
            Lançar termo
          </Button>
        </form>
      </div>

      <div className="rounded-lg border border-surface-border p-3">
        <h3 className="text-xs font-medium text-muted">PIA — Plano Individual de Atendimento</h3>
        <form onSubmit={salvarPia} className="mt-2 grid gap-2 sm:grid-cols-[1fr_auto_auto] sm:items-end">
          <Input id={`pia-escola-${acolhimento.id}`} label="Escola" maxLength={200} value={escolaPia}
            onChange={(e) => setEscolaPia(e.target.value)} autoComplete="off" />
          <label className="flex items-center gap-2 pb-2">
            <input type="checkbox" checked={vacinacaoPia} onChange={(e) => setVacinacaoPia(e.target.checked)} /> Vacinação atualizada
          </label>
          <Button type="submit" size="sm" loading={pendingPia}>
            Salvar PIA
          </Button>
        </form>

        <h4 className="mt-3 text-xs font-medium text-muted">Metas (plano e intervenção)</h4>
        {acolhimento.pia_metas.length > 0 && (
          <ul aria-label={`Metas do PIA do ${referencia}`} className="mt-2 flex flex-col gap-1">
            {acolhimento.pia_metas.map((m) => (
              <li key={m.id} className="flex items-center justify-between gap-2">
                <span>
                  {TIPOS_PIA_META[m.tipo]}: {m.descricao} {m.concluida && <Badge tone="success">Concluída</Badge>}
                </span>
                <span className="flex gap-1">
                  <Button size="sm" variant="ghost" aria-label={`${m.concluida ? "Reabrir" : "Concluir"} meta ${m.descricao}`}
                    onClick={() => void alternarConclusaoPiaMeta(m.id, m.concluida)}>
                    {m.concluida ? "Reabrir" : "Concluir"}
                  </Button>
                  <Button size="sm" variant="ghost" aria-label={`Remover meta ${m.descricao}`} onClick={() => void removerPiaMeta(m.id)}>
                    Remover
                  </Button>
                </span>
              </li>
            ))}
          </ul>
        )}
        <form onSubmit={adicionarPiaMeta} className="mt-2 grid gap-2 sm:grid-cols-[1fr_2fr_auto] sm:items-end">
          <Select id={`pia-meta-tipo-${acolhimento.id}`} label="Tipo de meta" value={tipoPiaMeta}
            onChange={(e) => setTipoPiaMeta(e.target.value as TipoPiaMeta)}
            options={Object.entries(TIPOS_PIA_META).map(([value, label]) => ({ value, label }))} />
          <Input id={`pia-meta-descricao-${acolhimento.id}`} label="Descrição da meta" maxLength={1000} value={descricaoPiaMeta}
            onChange={(e) => setDescricaoPiaMeta(e.target.value)} autoComplete="off" />
          <Button type="submit" size="sm" loading={pendingPiaMeta} disabled={descricaoPiaMeta.trim().length < 3}>
            Adicionar meta
          </Button>
        </form>
      </div>

      <div className="rounded-lg border border-surface-border p-3">
        <h3 className="text-xs font-medium text-muted">Documentos (PDF)</h3>
        {acolhimento.documentos.length > 0 && (
          <ul aria-label={`Documentos do ${referencia}`} className="mt-2 flex flex-col gap-1">
            {acolhimento.documentos.map((doc) => (
              <li key={doc.id} className="flex items-center justify-between gap-2">
                <span>
                  {doc.nome_original} · {TIPOS_DOCUMENTO_CASA_ABRIGO[doc.tipo]}
                  {doc.status === "pending" && <Badge tone="warning">Enviando</Badge>}
                </span>
                <span className="flex gap-1">
                  {doc.status === "ready" && (
                    <Button size="sm" variant="ghost" aria-label={`Baixar documento ${doc.nome_original}`} onClick={() => void baixarDocumento(doc.id)}>
                      Baixar
                    </Button>
                  )}
                  <Button size="sm" variant="ghost" aria-label={`Remover documento ${doc.nome_original}`} onClick={() => void removerDocumento(doc.id)}>
                    Remover
                  </Button>
                </span>
              </li>
            ))}
          </ul>
        )}
        <div className="mt-2 grid gap-2 sm:grid-cols-[1fr_auto] sm:items-end">
          <Select id={`documento-tipo-${acolhimento.id}`} label="Tipo de documento" value={tipoDocumento}
            onChange={(e) => setTipoDocumento(e.target.value as TipoDocumentoCasaAbrigo)}
            options={Object.entries(TIPOS_DOCUMENTO_CASA_ABRIGO).map(([value, label]) => ({ value, label }))} />
          <input ref={fileInput} type="file" accept="application/pdf" className="sr-only" id={`documento-arquivo-${acolhimento.id}`}
            onChange={(e) => { const f = e.target.files?.[0]; if (f) void enviarDocumento(f); e.target.value = ""; }} />
          <Button size="sm" loading={enviando} onClick={() => fileInput.current?.click()}>
            {enviando ? "Enviando…" : "Enviar PDF"}
          </Button>
        </div>
      </div>

      <Dialog open={desligando} onClose={() => setDesligando(false)} title="Desligar acolhimento"
        description="Restrito à equipe técnica.">
        <form onSubmit={confirmarDesligamento} className="flex flex-col gap-4">
          <Textarea id={`desligamento-motivo-${acolhimento.id}`} label="Motivo do desligamento *" rows={3} maxLength={2000}
            value={motivoDesligamento} onChange={(e) => setMotivoDesligamento(e.target.value)} required />
          <div>
            <Button type="submit" variant="danger" loading={pendingDesligar} disabled={motivoDesligamento.trim().length < 3}>
              Confirmar
            </Button>
          </div>
        </form>
      </Dialog>
    </li>
  );
}
