"use client";

import Link from "next/link";
import {
  CheckCheck,
  ClipboardCheck,
  Download,
  ExternalLink,
  Info,
  FileText,
  HeartHandshake,
  History,
  MessageSquarePlus,
  Play,
  Printer,
  Save,
  Send,
  Sparkles,
  Trash2,
  Upload,
  User,
  Users,
  UserX,
  X,
} from "lucide-react";
import { useRef, useState, type FormEvent } from "react";

import { CentroPopAtendimentoForm } from "@/components/sempras/CentroPopAtendimentoForm";
import { CrasAtendimentoForm } from "@/components/sempras/CrasAtendimentoForm";
import { CrasPaifCard } from "@/components/sempras/CrasPaifCard";
import { DataState } from "@/components/nexus/DataState";
import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import { useTopic } from "@/components/realtime/RealtimeProvider";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { mimeOf, putToTicket } from "@/lib/nexus/upload";
import type { UploadTicket } from "@/lib/nexus/types";
import {
  ACOMPANHAMENTOS,
  CAMPOS_CIDADAO,
  DESTINOS_EXTERNOS,
  P,
  STATUS_BENEFICIO,
  STATUS_ENCAMINHAMENTO_EXTERNO,
  TIPOS,
  TIPOS_BENEFICIO,
  TIPOS_DOCUMENTO_CENTRO_POP,
  TIPOS_UNIDADE,
  oferta,
  type Cidadao,
  type DestinoExterno,
  type DocumentoCentroPop,
  type FilaItem,
  type Prontuario,
  type TipoDocumentoCentroPop,
  type Unidade,
} from "@/lib/sempras/types";

interface FichaModalProps {
  cidadaoId: string | null;
  /** A unidade em que se atende: decide as abas (ADR 031 §4). */
  unidade: Unidade;
  onClose: () => void;
  onCidadaoUpdated?: () => void;
}

type TabKey =
  | "atendimento_cras"
  | "atendimento_pop"
  | "paif"
  | "cadastro"
  | "evolucoes"
  | "beneficios"
  | "documentos"
  | "encaminhamento";

/**
 * Modal dinâmico da Ficha do Munícipe:
 * - Atendimento Técnico especializado do Centro POP (População de Rua / Migrantes / Decreto 7.053/2009);
 * - Atendimento Técnico oficial do CRAS (MDS / SUAS);
 * - Gestão e acompanhamento integral do PAIF (vínculos, indicadores e desligamentos);
 * - Ações diretas de atendimento da fila (Iniciar, Concluir, Marcar Ausente);
 * - Visualização e edição cadastral com botão "Salvar alterações";
 * - Registro e histórico de evoluções técnicas;
 * - Consulta de benefícios eventuais e rede de apoio;
 * - Upload de documentos para o MinIO (PDF / identificação / laudos);
 * - Emissão e impressão de Guia de Encaminhamento Intersetorial.
 */
export function FichaModal({ cidadaoId, unidade, onClose, onCidadaoUpdated }: FichaModalProps) {
  const { can } = useNexus();
  const unidadeId = unidade.id;
  const unidadeNome = unidade.nome;
  const isCentroPop = oferta(unidade, "centro_pop");
  const temBeneficio = oferta(unidade, "beneficio");

  // As abas saem dos serviços da unidade (ADR 031 §4): nunca do nome dela.
  const abas: { key: TabKey; icon: typeof User; mostrar: boolean }[] = [
    { key: "atendimento_pop", icon: Sparkles, mostrar: isCentroPop },
    { key: "atendimento_cras", icon: ClipboardCheck, mostrar: unidade.tipo === "cras" },
    { key: "paif", icon: Users, mostrar: oferta(unidade, "paif") },
    { key: "evolucoes", icon: History, mostrar: true },
    { key: "beneficios", icon: HeartHandshake, mostrar: true },
    { key: "documentos", icon: FileText, mostrar: isCentroPop },
    { key: "encaminhamento", icon: Send, mostrar: isCentroPop },
    { key: "cadastro", icon: User, mostrar: true },
  ];
  const abasVisiveis = abas.filter((a) => a.mostrar);
  const [tab, setTab] = useState<TabKey>(abasVisiveis[0]!.key);
  const [textoEvolucao, setTextoEvolucao] = useState("");
  const [tipoEvolucao, setTipoEvolucao] = useState("social");

  const activeTab: TabKey = abasVisiveis.some((a) => a.key === tab) ? tab : abasVisiveis[0]!.key;

  // Encaminhamento Externo
  const [destinoExterno, setDestinoExterno] = useState<DestinoExterno>("saude_upa_hospital");
  const [motivoExterno, setMotivoExterno] = useState("");
  const [guiaImpressao, setGuiaImpressao] = useState<{
    destino: string;
    motivo: string;
    data: string;
  } | null>(null);

  // Upload Documento
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [tipoDoc, setTipoDoc] = useState<TipoDocumentoCentroPop>("identificacao");

  const { run: runSalvar, pending: pendingSalvar } = useAction();
  const { run: runEvolucao, pending: pendingEvolucao } = useAction();
  const { run: runAtendimento, pending: pendingAtendimento } = useAction();
  const { run: runDoc, pending: pendingDoc } = useAction();
  const { run: runEncExterno, pending: pendingEncExterno } = useAction();

  // Consulta do prontuário do cidadão
  const ficha = useApiQuery<Prontuario>(
    cidadaoId ? `v1/sempras/cidadaos/${cidadaoId}/prontuario` : null,
    { revalidateOnFocus: false }
  );

  // Consulta da fila da unidade para controle de atendimento em tempo real
  const fila = useApiQuery<FilaItem[]>(
    unidadeId ? `v1/sempras/unidades/${unidadeId}/fila` : null
  );
  useTopic(`sempras:fila:${unidadeId}`, () => void fila.mutate());

  if (!cidadaoId) return null;

  const cidadao = ficha.data?.cidadao;
  const registros = ficha.data?.registros ?? [];
  const beneficios = ficha.data?.beneficios ?? [];
  const redeApoio = ficha.data?.rede_apoio ?? [];
  const documentos = ficha.data?.centro_pop_documentos ?? [];
  const encaminhamentos = ficha.data?.encaminhamentos_externos ?? [];
  const outrasUnidades = ficha.data?.outras_unidades ?? [];
  const registrosOrigem = ficha.data?.registros_origem ?? [];

  const rotuloAba: Record<TabKey, string> = {
    atendimento_pop: "Atendimento Centro POP",
    atendimento_cras: "Atendimento CRAS",
    paif: `PAIF (${ficha.data?.paif_vinculos?.length ?? 0})`,
    evolucoes: `Evoluções & Histórico (${registros.length})`,
    beneficios: temBeneficio
      ? `Benefícios & Rede (${beneficios.length + redeApoio.length})`
      : `Rede de apoio (${redeApoio.length})`,
    documentos: `Documentos & Anexos (${documentos.length})`,
    encaminhamento: `Guia de Encaminhamento (${encaminhamentos.length})`,
    cadastro: "Dados Cadastrais",
  };

  // Item ativo da fila para este cidadão
  const itemFilaAtivo = fila.data?.find(
    (item) => item.cidadao_id === cidadaoId && ["chamado", "em_atendimento"].includes(item.status)
  );

  const tiposEvolucao = can(P.psicologia) ? TIPOS : TIPOS.filter((t) => t.value === "social");

  async function handleAcaoAtendimento(acao: "iniciar" | "concluir" | "ausente") {
    if (!itemFilaAtivo) return;
    const mensagens: Record<string, string> = {
      iniciar: "Atendimento iniciado com sucesso",
      concluir: "Atendimento concluído com sucesso",
      ausente: "Ausência do munícipe registrada",
    };
    const ok = await runAtendimento(
      () => apiClient.post(`v1/sempras/atendimentos/${itemFilaAtivo.id}/${acao}`),
      mensagens[acao]
    );
    if (ok) {
      void fila.mutate();
      if (acao === "concluir" || acao === "ausente") {
        onClose();
      }
    }
  }

  async function handleSalvarCadastro(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!cidadaoId) return;
    const fd = new FormData(e.currentTarget);
    const body = {
      ...Object.fromEntries(CAMPOS_CIDADAO.map((c) => [c.name, String(fd.get(c.name) ?? "").trim()])),
      anonimo: fd.get("anonimo") === "on",
    };
    const ok = await runSalvar(
      () => apiClient.put<Cidadao>(`v1/sempras/cidadaos/${cidadaoId}`, body),
      "Cadastro do munícipe atualizado com sucesso"
    );
    if (ok) {
      void ficha.mutate();
      onCidadaoUpdated?.();
    }
  }

  async function handleRegistrarEvolucao(e: FormEvent) {
    e.preventDefault();
    if (!cidadaoId || !textoEvolucao.trim()) return;
    const ok = await runEvolucao(
      () =>
        apiClient.post(`v1/sempras/cidadaos/${cidadaoId}/registros`, {
          unidade_id: unidadeId,
          tipo: tipoEvolucao,
          texto: textoEvolucao.trim(),
        }),
      "Evolução técnica registrada com sucesso"
    );
    if (ok) {
      setTextoEvolucao("");
      void ficha.mutate();
    }
  }

  async function handleEnviarDocumento(file: File) {
    if (!cidadaoId || !unidadeId) return;
    const ok = await runDoc(async () => {
      const ticketRes = await apiClient.post<{ upload: UploadTicket; documento: DocumentoCentroPop }>(
        `v1/sempras/cidadaos/${cidadaoId}/centro-pop-documentos/uploads`,
        {
          unidade_id: unidadeId,
          tipo: tipoDoc,
          filename: file.name,
          content_type: mimeOf(file),
          size: file.size,
        }
      );
      await putToTicket(ticketRes.data.upload, file);
      await apiClient.post(`v1/sempras/centro-pop-documentos/${ticketRes.data.documento.id}/confirm`);
    }, "Documento enviado e anexado ao prontuário");
    if (ok) void ficha.mutate();
  }

  async function handleBaixarDocumento(docId: string) {
    const res = await apiClient.get<{ url: string }>(`v1/sempras/centro-pop-documentos/${docId}/download`);
    if (res.data?.url) {
      window.open(res.data.url, "_blank", "noopener,noreferrer");
    }
  }

  async function handleRemoverDocumento(docId: string) {
    const ok = await runDoc(
      () => apiClient.delete(`v1/sempras/centro-pop-documentos/${docId}`),
      "Documento removido"
    );
    if (ok) void ficha.mutate();
  }

  async function handleRegistrarEncaminhamento(e: FormEvent) {
    e.preventDefault();
    if (!cidadaoId || !unidadeId || !motivoExterno.trim()) return;
    const ok = await runEncExterno(
      () =>
        apiClient.post(`v1/sempras/cidadaos/${cidadaoId}/encaminhamentos-externos`, {
          unidade_id: unidadeId,
          destino: destinoExterno,
          prioridade: "normal",
          motivo: motivoExterno.trim(),
        }),
      "Encaminhamento registrado na rede"
    );
    if (ok) {
      setGuiaImpressao({
        destino: DESTINOS_EXTERNOS[destinoExterno] || destinoExterno,
        motivo: motivoExterno.trim(),
        data: new Date().toLocaleDateString("pt-BR"),
      });
      setMotivoExterno("");
      void ficha.mutate();
    }
  }

  const tituloModal = cidadao
    ? `Ficha — ${cidadao.nome_social || cidadao.nome}`
    : "Ficha do Munícipe";

  return (
    <>
      <Dialog
        open={Boolean(cidadaoId)}
        onClose={onClose}
        title={tituloModal}
        size="5xl"
        description="Visualização rápida do prontuário, edição cadastral e registro de atendimento técnico."
        footer={
          <div className="flex w-full flex-wrap items-center justify-between gap-2">
            <Link
              href={`/sempras/cidadaos/${cidadaoId}?unidade=${unidadeId}`}
              className="inline-flex items-center gap-1.5 text-xs text-primary underline hover:text-primary-hover"
              target="_blank"
              rel="noopener noreferrer"
            >
              <ExternalLink size={14} aria-hidden="true" />
              Abrir prontuário completo em nova aba
            </Link>
            <div className="flex items-center gap-2">
              {itemFilaAtivo && (
                <Button
                  variant="primary"
                  size="sm"
                  onClick={() => handleAcaoAtendimento("concluir")}
                  loading={pendingAtendimento}
                >
                  <CheckCheck size={14} className="mr-1" />
                  Concluir atendimento
                </Button>
              )}
              <Button variant="secondary" size="sm" onClick={onClose}>
                Fechar
              </Button>
            </div>
          </div>
        }
      >
        <DataState
          loading={ficha.isLoading}
          error={ficha.error}
          onRetry={() => void ficha.mutate()}
          empty={!cidadao}
          emptyTitle="Munícipe não localizado"
        >
          {cidadao && (
            <div className="flex flex-col gap-4">
              {/* Barra de Atendimento Ativo da Fila (se chamado ou em atendimento) */}
              {itemFilaAtivo && (
                <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-primary/40 bg-primary/10 px-4 py-3 shadow-xs">
                  <div className="flex items-center gap-3">
                    <span className="font-mono text-2xl font-black text-primary">
                      {itemFilaAtivo.senha_exibida}
                    </span>
                    <div>
                      <div className="flex items-center gap-2">
                        <Badge tone={itemFilaAtivo.status === "em_atendimento" ? "success" : "info"}>
                          {itemFilaAtivo.status === "em_atendimento" ? "Em Atendimento" : "Chamado"}
                        </Badge>
                        <span className="text-xs font-semibold text-foreground">
                          {itemFilaAtivo.local ? `Local: ${itemFilaAtivo.local}` : "Sala de Atendimento"}
                        </span>
                      </div>
                      <span className="text-xs text-muted">
                        Tipo: {itemFilaAtivo.tipo === "psicologico" ? "Psicológico" : "Social"}
                      </span>
                    </div>
                  </div>

                  <div className="flex flex-wrap items-center gap-2">
                    {itemFilaAtivo.status === "chamado" && (
                      <Button
                        size="sm"
                        variant="secondary"
                        onClick={() => handleAcaoAtendimento("iniciar")}
                        loading={pendingAtendimento}
                      >
                        <Play size={14} className="mr-1" /> Iniciar atendimento
                      </Button>
                    )}
                    <Button
                      size="sm"
                      variant="primary"
                      onClick={() => handleAcaoAtendimento("concluir")}
                      loading={pendingAtendimento}
                    >
                      <CheckCheck size={14} className="mr-1" /> Concluir atendimento
                    </Button>
                    <Button
                      size="sm"
                      variant="danger"
                      onClick={() => handleAcaoAtendimento("ausente")}
                      loading={pendingAtendimento}
                    >
                      <UserX size={14} className="mr-1" /> Não compareceu
                    </Button>
                  </div>
                </div>
              )}

              {/* Cabeçalho resumido do munícipe */}
              <div className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-surface-border bg-surface-muted/40 p-3">
                <div className="flex flex-col">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-bold text-foreground">
                      {cidadao.nome_social ? `${cidadao.nome_social} (${cidadao.nome})` : cidadao.nome}
                    </span>
                    <Badge tone="info">{TIPOS_UNIDADE[unidade.tipo]?.label ?? unidadeNome}</Badge>
                  </div>
                  <span className="text-xs text-muted">
                    {[
                      cidadao.cpf && `CPF ${cidadao.cpf}`,
                      cidadao.nis && `NIS ${cidadao.nis}`,
                      cidadao.data_nascimento && `Nasc. ${cidadao.data_nascimento}`,
                      cidadao.telefone && `Tel. ${cidadao.telefone}`,
                    ]
                      .filter(Boolean)
                      .join(" · ")}
                  </span>
                </div>
                <Badge tone="info">Prontuário Ativo</Badge>
              </div>

              {/* Acompanhamento em outras unidades: só o resumo, sem conteúdo (ADR 031 §4) */}
              {outrasUnidades.length > 0 && (
                <div className="flex flex-wrap items-center gap-2 rounded-xl border border-info/30 bg-info/5 p-3 text-xs">
                  <Info size={14} className="shrink-0 text-info" aria-hidden="true" />
                  <span className="font-semibold text-foreground">Também acompanhado por:</span>
                  {outrasUnidades.map((o) => (
                    <Badge key={o.unidade_id} tone="neutral">
                      {o.unidade_nome} — {o.ativos.map((a) => ACOMPANHAMENTOS[a] ?? a).join(", ")}
                    </Badge>
                  ))}
                </div>
              )}

              {/* Abas de Navegação — pelos serviços da unidade */}
              <div role="tablist" className="flex flex-wrap gap-1.5 border-b border-surface-border pb-2">
                {abasVisiveis.map(({ key, icon: Icone }) => (
                  <button
                    key={key}
                    type="button"
                    role="tab"
                    aria-selected={activeTab === key}
                    onClick={() => setTab(key)}
                    className={`inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors ${
                      activeTab === key
                        ? "bg-primary text-primary-foreground font-semibold shadow-xs"
                        : "text-muted hover:bg-surface-border/50 hover:text-foreground"
                    }`}
                  >
                    <Icone size={14} aria-hidden="true" />
                    {rotuloAba[key]}
                  </button>
                ))}
              </div>

              {/* Aba Atendimento Especializado Centro POP */}
              {activeTab === "atendimento_pop" && (
                <CentroPopAtendimentoForm
                  cidadao={cidadao}
                  unidadeId={unidadeId}
                  unidadeNome={unidadeNome}
                  perfil={ficha.data?.centro_pop_perfil}
                  itemFilaId={itemFilaAtivo?.id}
                  isEmAtendimento={itemFilaAtivo?.status === "em_atendimento"}
                  onSalvo={() => {
                    void ficha.mutate();
                    void fila.mutate();
                  }}
                  onConcluido={() => {
                    void ficha.mutate();
                    void fila.mutate();
                    onClose();
                  }}
                />
              )}

              {/* Aba Atendimento Técnico CRAS (Formulário Oficial MDS / Protocolo) */}
              {activeTab === "atendimento_cras" && (
                <CrasAtendimentoForm
                  cidadao={cidadao}
                  unidadeId={unidadeId}
                  unidadeNome={unidadeNome}
                  itemFilaId={itemFilaAtivo?.id}
                  isEmAtendimento={itemFilaAtivo?.status === "em_atendimento"}
                  onSalvo={() => {
                    void ficha.mutate();
                    void fila.mutate();
                  }}
                  onConcluido={() => {
                    void ficha.mutate();
                    void fila.mutate();
                    onClose();
                  }}
                />
              )}

              {/* Aba PAIF (Acompanhamento e Inclusão / Desligamento) - estritamente CRAS */}
              {activeTab === "paif" && (
                <CrasPaifCard
                  cidadao={cidadao}
                  unidadeId={unidadeId}
                  unidadeNome={unidadeNome}
                  paifVinculos={ficha.data?.paif_vinculos ?? []}
                  onUpdated={() => {
                    void ficha.mutate();
                  }}
                />
              )}

              {/* Aba: Edição Cadastral */}
              {activeTab === "cadastro" && (
                <form onSubmit={handleSalvarCadastro} className="flex flex-col gap-4">
                  <div className="grid gap-3 sm:grid-cols-2">
                    {CAMPOS_CIDADAO.map((c) => (
                      <Input
                        key={c.name}
                        id={`edit-${c.name}`}
                        name={c.name}
                        label={c.label}
                        type={c.type}
                        maxLength={c.max}
                        defaultValue={String(cidadao[c.name] ?? "")}
                        required={c.name === "nome"}
                        autoComplete="off"
                      />
                    ))}
                    <label className="flex items-center gap-2 text-sm sm:col-span-2">
                      <input
                        type="checkbox"
                        name="anonimo"
                        defaultChecked={cidadao.anonimo}
                        className="rounded border-surface-border"
                      />
                      Cadastro anônimo / Não possui documentos (Centro POP)
                    </label>
                  </div>

                  <div className="flex items-center justify-end gap-2 border-t border-surface-border pt-3">
                    <Button type="submit" loading={pendingSalvar}>
                      <Save size={16} aria-hidden="true" className="mr-1.5" />
                      Salvar alterações
                    </Button>
                  </div>
                </form>
              )}

              {/* Aba 2: Evoluções e Atendimentos */}
              {activeTab === "evolucoes" && (
                <div className="flex flex-col gap-5">
                  {(can(P.atendimento) || can(P.psicologia)) && (
                    <form
                      onSubmit={handleRegistrarEvolucao}
                      className="flex flex-col gap-3 rounded-xl border border-surface-border bg-surface-muted/30 p-4"
                    >
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <span className="text-xs font-semibold text-foreground">
                          Nova Evolução Técnica do Atendimento
                        </span>
                        <div className="w-48">
                          <Select
                            id="modal-evolucao-tipo"
                            value={tipoEvolucao}
                            onChange={(e) => setTipoEvolucao(e.target.value)}
                            options={tiposEvolucao}
                          />
                        </div>
                      </div>
                      <Textarea
                        id="modal-evolucao-texto"
                        placeholder="Descreva as orientações, encaminhamentos ou relato técnico do atendimento..."
                        value={textoEvolucao}
                        onChange={(e) => setTextoEvolucao(e.target.value)}
                        rows={3}
                        required
                      />
                      <div className="flex justify-end">
                        <Button type="submit" size="sm" loading={pendingEvolucao}>
                          <MessageSquarePlus size={15} aria-hidden="true" className="mr-1" />
                          Registrar evolução
                        </Button>
                      </div>
                    </form>
                  )}

                  <div className="flex flex-col gap-2">
                    <span className="text-xs font-semibold uppercase tracking-wider text-muted">
                      Histórico de Evoluções ({registros.length})
                    </span>
                    {registros.length === 0 ? (
                      <p className="rounded-xl border border-dashed border-surface-border p-4 text-center text-xs text-muted">
                        Nenhuma evolução registrada para este munícipe até o momento.
                      </p>
                    ) : (
                      <ul className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
                        {registros.map((r) => (
                          <li key={r.id} className="flex flex-col gap-1.5 p-3 text-sm">
                            <div className="flex flex-wrap items-center justify-between gap-2">
                              <span className="font-semibold text-foreground">
                                {r.autor_nome || "Técnico"}
                              </span>
                              <div className="flex items-center gap-2">
                                <Badge tone={r.tipo === "psicologico" ? "info" : "neutral"}>
                                  {r.tipo === "psicologico" ? "Psicologia" : "Social"}
                                </Badge>
                                <span className="text-xs text-muted">
                                  {fmtDateTime(r.created_at)}
                                </span>
                              </div>
                            </div>
                            <p className="whitespace-pre-wrap text-foreground/90">{r.texto}</p>
                          </li>
                        ))}
                      </ul>
                    )}
                  </div>

                  {/* Evolução social de quem encaminhou o munícipe para cá (ADR 031 §4) */}
                  {registrosOrigem.length > 0 && (
                    <div className="flex flex-col gap-2">
                      <span className="text-xs font-semibold uppercase tracking-wider text-muted">
                        Evolução social da unidade de origem do encaminhamento ({registrosOrigem.length})
                      </span>
                      <ul className="flex flex-col divide-y divide-surface-border rounded-xl border border-info/30 bg-info/5">
                        {registrosOrigem.map((r) => (
                          <li key={r.id} className="flex flex-col gap-1.5 p-3 text-sm">
                            <div className="flex flex-wrap items-center justify-between gap-2">
                              <span className="font-semibold text-foreground">
                                {r.autor_nome || "Técnico"} ·{" "}
                                {outrasUnidades.find((o) => o.unidade_id === r.unidade_id)?.unidade_nome ?? "outra unidade"}
                              </span>
                              <span className="text-xs text-muted">{fmtDateTime(r.created_at)}</span>
                            </div>
                            <p className="whitespace-pre-wrap text-foreground/90">{r.texto}</p>
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}
                </div>
              )}

              {/* Aba 3: Benefícios & Rede */}
              {activeTab === "beneficios" && (
                <div className="flex flex-col gap-5">
                  {temBeneficio && (
                  <div className="flex flex-col gap-2">
                    <span className="text-xs font-semibold uppercase tracking-wider text-muted">
                      Benefícios Eventuais Concedidos ({beneficios.length})
                    </span>
                    {beneficios.length === 0 ? (
                      <p className="rounded-xl border border-dashed border-surface-border p-4 text-center text-xs text-muted">
                        Nenhum benefício eventual registrado.
                      </p>
                    ) : (
                      <ul className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
                        {beneficios.map((b) => (
                          <li key={b.id} className="flex flex-wrap items-center justify-between gap-2 p-3 text-sm">
                            <div>
                              <p className="font-semibold">{TIPOS_BENEFICIO[b.tipo] || b.tipo}</p>
                              <p className="text-xs text-muted">
                                {b.data_solicitacao && `Solicitado em ${fmtDateTime(b.data_solicitacao)}`}
                                {b.observacao && ` · Obs: ${b.observacao}`}
                              </p>
                            </div>
                            <Badge tone={STATUS_BENEFICIO[b.status]?.tone ?? "neutral"}>
                              {STATUS_BENEFICIO[b.status]?.label ?? b.status}
                            </Badge>
                          </li>
                        ))}
                      </ul>
                    )}
                  </div>
                  )}

                  <div className="flex flex-col gap-2">
                    <span className="text-xs font-semibold uppercase tracking-wider text-muted">
                      Rede de Apoio Familiar e Comunitária ({redeApoio.length})
                    </span>
                    {redeApoio.length === 0 ? (
                      <p className="rounded-xl border border-dashed border-surface-border p-4 text-center text-xs text-muted">
                        Nenhum contato da rede de apoio cadastrado.
                      </p>
                    ) : (
                      <ul className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
                        {redeApoio.map((c) => (
                          <li key={c.id} className="flex items-center justify-between p-3 text-sm">
                            <div>
                              <p className="font-semibold">{c.nome}</p>
                              <p className="text-xs text-muted">
                                {c.parentesco} {c.telefone && `· Tel. ${c.telefone}`}
                              </p>
                            </div>
                            <span className="text-xs text-muted">{fmtDateTime(c.created_at)}</span>
                          </li>
                        ))}
                      </ul>
                    )}
                  </div>
                </div>
              )}

              {/* Aba 4: Documentos Anexos */}
              {activeTab === "documentos" && (
                <div className="flex flex-col gap-4">
                  {(can(P.atendimento) || can(P.psicologia)) && (
                    <div className="flex flex-wrap items-end gap-3 rounded-xl border border-surface-border bg-surface-muted/30 p-4">
                      <div className="w-56">
                        <Select
                          id="modal-doc-tipo"
                          label="Tipo de documento"
                          value={tipoDoc}
                          onChange={(e) => setTipoDoc(e.target.value as TipoDocumentoCentroPop)}
                          options={Object.entries(TIPOS_DOCUMENTO_CENTRO_POP).map(([value, label]) => ({
                            value,
                            label,
                          }))}
                        />
                      </div>
                      <input
                        ref={fileInputRef}
                        type="file"
                        accept="application/pdf,image/jpeg,image/png"
                        className="sr-only"
                        id="modal-file-input"
                        onChange={(e) => {
                          const f = e.target.files?.[0];
                          if (f) void handleEnviarDocumento(f);
                          e.target.value = "";
                        }}
                      />
                      <Button
                        size="sm"
                        loading={pendingDoc}
                        onClick={() => fileInputRef.current?.click()}
                      >
                        <Upload size={14} className="mr-1.5" />
                        Selecionar arquivo e anexar
                      </Button>
                      <span className="text-[11px] text-muted">
                        Formatos aceitos: PDF, JPEG, PNG (até 15MB)
                      </span>
                    </div>
                  )}

                  <div className="flex flex-col gap-2">
                    <span className="text-xs font-semibold uppercase tracking-wider text-muted">
                      Documentos Anexados ({documentos.length})
                    </span>
                    {documentos.length === 0 ? (
                      <p className="rounded-xl border border-dashed border-surface-border p-6 text-center text-xs text-muted">
                        Nenhum documento digitalizado anexado ao prontuário.
                      </p>
                    ) : (
                      <ul className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
                        {documentos.map((doc) => (
                          <li key={doc.id} className="flex flex-wrap items-center justify-between gap-3 p-3 text-sm">
                            <div className="flex items-center gap-2.5">
                              <FileText size={18} className="text-primary" />
                              <div>
                                <p className="font-semibold text-foreground">{doc.nome_original}</p>
                                <p className="text-xs text-muted">
                                  {TIPOS_DOCUMENTO_CENTRO_POP[doc.tipo] || doc.tipo} · Enviado em {fmtDateTime(doc.created_at)}
                                </p>
                              </div>
                            </div>
                            <div className="flex items-center gap-2">
                              {doc.status === "ready" && (
                                <Button
                                  size="sm"
                                  variant="secondary"
                                  onClick={() => void handleBaixarDocumento(doc.id)}
                                >
                                  <Download size={14} className="mr-1" /> Baixar
                                </Button>
                              )}
                              <Button
                                size="sm"
                                variant="danger"
                                onClick={() => void handleRemoverDocumento(doc.id)}
                              >
                                <Trash2 size={14} className="mr-1" /> Remover
                              </Button>
                            </div>
                          </li>
                        ))}
                      </ul>
                    )}
                  </div>
                </div>
              )}

              {/* Aba 5: Guia de Encaminhamento */}
              {activeTab === "encaminhamento" && (
                <div className="flex flex-col gap-5">
                  {(can(P.atendimento) || can(P.psicologia)) && (
                    <form
                      onSubmit={handleRegistrarEncaminhamento}
                      className="flex flex-col gap-3 rounded-xl border border-surface-border bg-surface-muted/30 p-4"
                    >
                      <span className="text-xs font-semibold text-foreground">
                        Emitir Encaminhamento para a Rede
                      </span>
                      <div className="grid gap-3 sm:grid-cols-2">
                        <Select
                          id="enc-destino"
                          label="Destino do Encaminhamento"
                          value={destinoExterno}
                          onChange={(e) => setDestinoExterno(e.target.value as DestinoExterno)}
                          options={Object.entries(DESTINOS_EXTERNOS).map(([value, label]) => ({
                            value,
                            label,
                          }))}
                        />
                      </div>
                      <Textarea
                        id="enc-motivo"
                        label="Justificativa Técnica / Motivo do Encaminhamento"
                        placeholder="Informe as demandas identificadas, histórico do caso e orientações ao órgão de destino..."
                        value={motivoExterno}
                        onChange={(e) => setMotivoExterno(e.target.value)}
                        rows={3}
                        required
                      />
                      <div className="flex justify-end">
                        <Button type="submit" size="sm" loading={pendingEncExterno}>
                          <Send size={14} className="mr-1.5" />
                          Registrar e Gerar Guia
                        </Button>
                      </div>
                    </form>
                  )}

                  <div className="flex flex-col gap-2">
                    <span className="text-xs font-semibold uppercase tracking-wider text-muted">
                      Histórico de Encaminhamentos ({encaminhamentos.length})
                    </span>
                    {encaminhamentos.length === 0 ? (
                      <p className="rounded-xl border border-dashed border-surface-border p-4 text-center text-xs text-muted">
                        Nenhum encaminhamento externo registrado.
                      </p>
                    ) : (
                      <ul className="flex flex-col divide-y divide-surface-border rounded-xl border border-surface-border">
                        {encaminhamentos.map((enc) => (
                          <li key={enc.id} className="flex flex-wrap items-center justify-between gap-3 p-3 text-sm">
                            <div>
                              <p className="font-semibold text-foreground">
                                {DESTINOS_EXTERNOS[enc.destino] || enc.destino}
                              </p>
                              <p className="text-xs text-muted">
                                {enc.motivo} · {fmtDateTime(enc.created_at)}
                              </p>
                            </div>
                            <div className="flex items-center gap-2">
                              <Badge tone={STATUS_ENCAMINHAMENTO_EXTERNO[enc.status]?.tone ?? "neutral"}>
                                {STATUS_ENCAMINHAMENTO_EXTERNO[enc.status]?.label ?? enc.status}
                              </Badge>
                              <Button
                                size="sm"
                                variant="secondary"
                                onClick={() =>
                                  setGuiaImpressao({
                                    destino: DESTINOS_EXTERNOS[enc.destino] || enc.destino,
                                    motivo: enc.motivo,
                                    data: new Date(enc.created_at).toLocaleDateString("pt-BR"),
                                  })
                                }
                              >
                                <Printer size={14} className="mr-1" /> Imprimir Guia
                              </Button>
                            </div>
                          </li>
                        ))}
                      </ul>
                    )}
                  </div>
                </div>
              )}
            </div>
          )}
        </DataState>
      </Dialog>

      {/* Modal / Visualização de Impressão da Guia de Encaminhamento */}
      <Dialog
        open={Boolean(guiaImpressao)}
        onClose={() => setGuiaImpressao(null)}
        title="Guia Oficial de Encaminhamento"
        size="lg"
      >
        {guiaImpressao && cidadao && (
          <div className="flex flex-col gap-5">
            <div
              id="guia-encaminhamento-impressao"
              className="flex flex-col gap-4 rounded-xl border border-surface-border bg-surface p-6 print:border-none print:p-0"
            >
              <div className="border-b border-surface-border pb-3 text-center">
                <h3 className="text-sm font-bold uppercase tracking-wider text-foreground">
                  PREFEITURA MUNICIPAL
                </h3>
                <p className="text-xs font-semibold text-muted">
                  SECRETARIA MUNICIPAL DE ASSISTÊNCIA SOCIAL (SEMPRAS)
                </p>
                <p className="text-[11px] text-muted">SISTEMA ÚNICO DE ASSISTÊNCIA SOCIAL — SUAS</p>
                <div className="mt-2 inline-block rounded-md bg-primary/10 px-3 py-1 text-xs font-bold text-primary">
                  GUIA DE ENCAMINHAMENTO INTERSETORIAL
                </div>
              </div>

              <div className="grid gap-2 text-xs">
                <div>
                  <strong>Destino:</strong> {guiaImpressao.destino}
                </div>
                <div>
                  <strong>Data de Emissão:</strong> {guiaImpressao.data}
                </div>
                <div className="border-t border-surface-border pt-2">
                  <strong>Munícipe:</strong> {cidadao.nome_social || cidadao.nome}
                </div>
                <div>
                  <strong>CPF:</strong> {cidadao.cpf || "Não informado"} · <strong>NIS:</strong>{" "}
                  {cidadao.nis || "Não informado"}
                </div>
                <div>
                  <strong>Endereço:</strong> {cidadao.endereco || "Não informado"} ·{" "}
                  <strong>Telefone:</strong> {cidadao.telefone || "Não informado"}
                </div>
              </div>

              <div className="rounded-lg border border-surface-border bg-surface-muted/20 p-3 text-xs">
                <strong>Justificativa Técnica e Solicitação:</strong>
                <p className="mt-1 whitespace-pre-wrap">{guiaImpressao.motivo}</p>
              </div>

              <div className="mt-8 flex justify-around border-t border-surface-border pt-6 text-center text-xs">
                <div>
                  <div className="mb-1 h-8 border-b border-dashed border-surface-border w-44"></div>
                  <span>Assinatura do Munícipe</span>
                </div>
                <div>
                  <div className="mb-1 h-8 border-b border-dashed border-surface-border w-44"></div>
                  <span>Técnico Responsável / Carimbo</span>
                </div>
              </div>
            </div>

            <div className="flex justify-end gap-2 border-t border-surface-border pt-3">
              <Button size="sm" onClick={() => window.print()}>
                <Printer size={15} className="mr-1.5" /> Imprimir Guia (PDF)
              </Button>
              <Button size="sm" variant="secondary" onClick={() => setGuiaImpressao(null)}>
                Fechar
              </Button>
            </div>
          </div>
        )}
      </Dialog>
    </>
  );
}
