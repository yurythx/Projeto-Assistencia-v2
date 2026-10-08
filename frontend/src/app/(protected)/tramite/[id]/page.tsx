"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import {
  Archive,
  ArrowLeft,
  CheckCheck,
  Download,
  FilePen,
  Forward,
  Paperclip,
  PenTool,
  RotateCcw,
  UserMinus,
  UserPlus,
} from "lucide-react";
import { useState, type FormEvent } from "react";

import { UserPicker, type PickedUser } from "@/components/iam/UserPicker";
import { ConfirmButton } from "@/components/nexus/ConfirmButton";
import { DataState } from "@/components/nexus/DataState";
import { Markdown } from "@/components/nexus/Markdown";
import { fmtBytes, fmtDateTime, useAction } from "@/components/nexus/useAction";
import { DOC_STATUS, PROCESSO_STATUS, SIGILO } from "@/components/tramite/labels";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { AtlasDoProcesso, usePecasDoAtlas } from "@/components/tramite/AtlasDoProcesso";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { chave, urlArquivoModelo } from "@/lib/atlas/modelos";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { Documento, OrgTree, ProcessoView, UploadTicket } from "@/lib/nexus/types";
import { mimeOf, putToTicket } from "@/lib/nexus/upload";

type Acao = "tramitar" | "concluir" | "arquivar" | "reabrir";
const ACAO: Record<Acao, { title: string; path: string; ok: string }> = {
  tramitar: { title: "Tramitar processo", path: "tramitar", ok: "Processo tramitado" },
  concluir: { title: "Concluir processo", path: "concluir", ok: "Processo concluído" },
  arquivar: { title: "Arquivar processo", path: "arquivar", ok: "Processo arquivado" },
  reabrir: { title: "Reabrir processo", path: "reabrir", ok: "Processo reaberto" },
};

function DespachoForm({
  processo,
  acao,
  onDone,
}: {
  processo: ProcessoView;
  acao: Acao;
  onDone: () => void;
}) {
  const tree = useApiQuery<OrgTree[]>(acao === "tramitar" ? "v1/iam/org-tree" : null);
  const { run, pending } = useAction();
  const unidades = (tree.data ?? [])
    .filter((e) => e.ativo)
    .flatMap((e) => e.unidades.filter((u) => u.ativo && u.id !== processo.unidade_atual_id));

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    const ok = await run(
      () =>
        apiClient.post(`v1/tramite/processos/${processo.id}/${ACAO[acao].path}`, {
          despacho: String(fd.get("despacho") ?? "").trim(),
          para_unidade_id: acao === "tramitar" ? String(fd.get("para") ?? "") : undefined,
        }),
      ACAO[acao].ok,
    );
    if (ok) onDone();
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-3">
      {acao === "tramitar" && processo.sigilo === "sigiloso" && (
        <p role="note" className="rounded-md bg-warning/10 p-3 text-xs text-foreground">
          Processo <strong>sigiloso</strong>: só quem tem credencial nominal o enxerga. Conceda
          acesso às pessoas da unidade de destino (painel &ldquo;Acesso nominal&rdquo;){" "}
          <strong>antes</strong> de tramitar — depois disso sua unidade não atua mais no processo.
        </p>
      )}
      {acao === "tramitar" && (
        <Select
          id="desp-para"
          name="para"
          label="Unidade de destino *"
          required
          placeholder="Selecione…"
          options={unidades.map((u) => ({
            value: u.id,
            label: `${u.sigla ? `${u.sigla} — ` : ""}${u.nome}`,
          }))}
        />
      )}
      <Textarea
        id="desp-texto"
        name="despacho"
        label="Despacho *"
        rows={4}
        required
        minLength={3}
        maxLength={5000}
      />
      <div className="flex justify-end">
        <Button type="submit" loading={pending}>
          Confirmar
        </Button>
      </div>
    </form>
  );
}

function DocumentoForm({
  processo,
  doc,
  onDone,
}: {
  processo: ProcessoView;
  doc?: Documento;
  onDone: () => void;
}) {
  const { run, pending } = useAction();
  // Peças do procedimento do Atlas (ADR 026): sugerem o tipo e o modelo.
  const { pecas } = usePecasDoAtlas(processo);
  const [tipo, setTipo] = useState(doc?.tipo ?? "");
  const pecaDoTipo = pecas.find((x) => chave(x.nome) === chave(tipo));
  const [origem, setOrigem] = useState<"redigido" | "anexo">(doc?.origem ?? "redigido");
  const [conteudo, setConteudo] = useState(doc?.conteudo ?? "");
  const [file, setFile] = useState<File | null>(null);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    const titulo = String(fd.get("titulo") ?? "").trim();
    const tipo = String(fd.get("tipo") ?? "").trim();
    const ok = await run(async () => {
      if (doc) return apiClient.put(`v1/tramite/documentos/${doc.id}`, { titulo, conteudo });
      let objectKey = "";
      if (origem === "anexo") {
        if (!file) throw new Error("selecione o arquivo");
        const { data: ticket } = await apiClient.post<UploadTicket>(
          `v1/tramite/processos/${processo.id}/uploads`,
          { filename: file.name, content_type: mimeOf(file) },
        );
        await putToTicket(ticket, file);
        objectKey = ticket.object_key;
      }
      return apiClient.post(`v1/tramite/processos/${processo.id}/documentos`, {
        tipo,
        titulo,
        conteudo: origem === "redigido" ? conteudo : "",
        object_key: objectKey,
      });
    }, "Documento salvo");
    if (ok) onDone();
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-3">
      {!doc && (
        <div role="radiogroup" aria-label="Origem do documento" className="flex gap-4 text-sm">
          <label className="flex items-center gap-2">
            <input
              type="radio"
              name="origem"
              checked={origem === "redigido"}
              onChange={() => setOrigem("redigido")}
              className="accent-primary"
            />{" "}
            Redigir no sistema
          </label>
          <label className="flex items-center gap-2">
            <input
              type="radio"
              name="origem"
              checked={origem === "anexo"}
              onChange={() => setOrigem("anexo")}
              className="accent-primary"
            />{" "}
            Anexar arquivo
          </label>
        </div>
      )}
      <div className="grid gap-3 sm:grid-cols-[1fr_12rem]">
        <Input
          id="doc-titulo"
          name="titulo"
          label="Título *"
          required
          maxLength={300}
          defaultValue={doc?.titulo}
        />
        <Input
          id="doc-tipo"
          name="tipo"
          label="Tipo"
          maxLength={60}
          value={tipo}
          onChange={(e) => setTipo(e.target.value)}
          placeholder="Ofício, Parecer…"
          disabled={!!doc}
          list={pecas.length ? "doc-tipo-pecas" : undefined}
        />
        {pecas.length > 0 && (
          <datalist id="doc-tipo-pecas">
            {pecas.map((x) => (
              <option key={`${x.etapa}-${x.nome}`} value={x.nome} />
            ))}
          </datalist>
        )}
      </div>
      {!doc && pecaDoTipo?.modelo && (
        <p className="flex flex-wrap items-center gap-2 rounded-md bg-primary/5 px-3 py-2 text-sm">
          Esta peça tem modelo no Atlas:
          <a
            href={urlArquivoModelo(pecaDoTipo.modelo.id)}
            download={pecaDoTipo.modelo.arquivo}
            className="inline-flex items-center gap-1 font-medium text-primary hover:underline"
          >
            <Download size={14} aria-hidden="true" /> Baixar {pecaDoTipo.modelo.nome}
          </a>
        </p>
      )}
      {origem === "redigido" ? (
        <Textarea
          id="doc-conteudo"
          label="Conteúdo (Markdown)"
          rows={12}
          maxLength={200000}
          value={conteudo}
          onChange={(e) => setConteudo(e.target.value)}
          className="font-mono"
        />
      ) : (
        <div className="flex flex-col gap-1">
          <label htmlFor="doc-file" className="text-sm font-medium">
            Arquivo *
          </label>
          <input
            id="doc-file"
            type="file"
            required
            accept=".pdf,.doc,.docx,.odt,.xls,.xlsx,.ods,.png,.jpg,.jpeg,.txt"
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
            className="text-sm"
          />
          <p className="text-xs text-muted">
            O SHA-256 do arquivo é calculado no servidor após o envio.
          </p>
        </div>
      )}
      <div className="flex justify-end">
        <Button type="submit" loading={pending}>
          Salvar documento
        </Button>
      </div>
    </form>
  );
}

function AssinaturaForm({ doc, onDone }: { doc: Documento; onDone: () => void }) {
  const { run, pending } = useAction();
  const [signers, setSigners] = useState<PickedUser[]>([]);
  const [sequential, setSequential] = useState(false);
  return (
    <div className="flex flex-col gap-3">
      <p className="text-sm text-muted">
        Um envelope do Signum é aberto com o SHA-256 do documento; o conteúdo fica bloqueado para
        edição.
      </p>
      <UserPicker
        idPrefix="ass-signers"
        label="Signatários *"
        value={signers}
        onChange={setSigners}
      />
      <label className="flex items-center gap-2 text-sm">
        <input
          type="checkbox"
          checked={sequential}
          onChange={(e) => setSequential(e.target.checked)}
          className="h-4 w-4 accent-primary"
        />{" "}
        Assinatura sequencial
      </label>
      <Button
        className="self-end"
        disabled={signers.length === 0}
        loading={pending}
        onClick={() =>
          void run(
            () =>
              apiClient.post(`v1/tramite/documentos/${doc.id}/assinatura`, {
                signer_ids: signers.map((s) => s.id),
                sequential,
              }),
            "Assinatura solicitada",
          ).then((ok) => ok !== undefined && onDone())
        }
      >
        <PenTool size={16} aria-hidden="true" className="mr-1" /> Solicitar assinatura
      </Button>
    </div>
  );
}

function DocumentoView({ id }: { id: string }) {
  const doc = useApiQuery<Documento>(`v1/tramite/documentos/${id}`);
  const d = doc.data;
  if (!d)
    return (
      <DataState loading={doc.isLoading} error={doc.error} empty={false}>
        {null}
      </DataState>
    );
  return (
    <div className="flex flex-col gap-3">
      <p className="text-xs text-muted">
        {d.tipo && `${d.tipo} · `}
        {fmtDateTime(d.created_at)}
        {d.sha256 && <span className="block break-all font-mono">SHA-256: {d.sha256}</span>}
      </p>
      {d.origem === "redigido" ? (
        <div className="rounded-lg border border-surface-border p-4">
          <Markdown source={d.conteudo ?? ""} />
        </div>
      ) : (
        d.download_url && (
          <a
            href={d.download_url}
            className="inline-flex items-center gap-1 self-start rounded-lg bg-primary px-4 py-2 text-sm text-primary-foreground"
          >
            <Download size={16} aria-hidden="true" /> Baixar ({fmtBytes(d.size_bytes)})
          </a>
        )
      )}
      {d.envelope_id && (
        <Link href={`/signum/${d.envelope_id}`} className="text-sm text-primary hover:underline">
          Ver envelope de assinatura →
        </Link>
      )}
    </div>
  );
}

export default function ProcessoPage() {
  const { id } = useParams<{ id: string }>();
  const { can } = useNexus();
  const proc = useApiQuery<ProcessoView>(`v1/tramite/processos/${id}`);
  const { run, pending } = useAction();
  const [acao, setAcao] = useState<Acao | null>(null);
  const [docDialog, setDocDialog] = useState<
    { kind: "novo" } | { kind: "editar" | "ver" | "assinar"; doc: Documento } | null
  >(null);
  const [acesso, setAcesso] = useState<PickedUser[]>([]);
  const p = proc.data;
  const refresh = () => void proc.mutate();
  const aberto = p && (p.status === "aberto" || p.status === "em_tramitacao");

  return (
    <div className="flex flex-col gap-6">
      <Link
        href="/tramite"
        className="inline-flex items-center gap-1 text-sm text-muted hover:text-foreground"
      >
        <ArrowLeft size={14} aria-hidden="true" /> Processos
      </Link>
      <DataState loading={proc.isLoading} error={proc.error} empty={!p}>
        {p && (
          <>
            <header className="flex flex-col gap-2">
              <p className="font-mono text-sm text-muted">{p.numero}</p>
              <h1 className="text-2xl font-semibold">{p.assunto}</h1>
              <div className="flex flex-wrap gap-2">
                <Badge tone={PROCESSO_STATUS[p.status].tone}>
                  {PROCESSO_STATUS[p.status].label}
                </Badge>
                <Badge tone={SIGILO[p.sigilo].tone}>{SIGILO[p.sigilo].label}</Badge>
                <Badge>{p.tipo}</Badge>
              </div>
              <dl className="mt-2 grid gap-2 text-sm sm:grid-cols-3">
                <div>
                  <dt className="text-xs text-muted">Interessado</dt>
                  <dd>{p.interessado || "—"}</dd>
                </div>
                <div>
                  <dt className="text-xs text-muted">Origem</dt>
                  <dd>{p.unidade_origem}</dd>
                </div>
                <div>
                  <dt className="text-xs text-muted">Unidade atual</dt>
                  <dd className="font-medium">{p.unidade_atual}</dd>
                </div>
                <div>
                  <dt className="text-xs text-muted">Aberto por</dt>
                  <dd>
                    {p.created_by_name} · {fmtDateTime(p.created_at)}
                  </dd>
                </div>
                {p.concluido_at && (
                  <div>
                    <dt className="text-xs text-muted">Concluído</dt>
                    <dd>{fmtDateTime(p.concluido_at)}</dd>
                  </div>
                )}
              </dl>
              {p.descricao && <p className="whitespace-pre-line text-sm">{p.descricao}</p>}
            </header>

            <div className="flex flex-wrap gap-2">
              {aberto && p.can_route && (
                <Button onClick={() => setAcao("tramitar")}>
                  <Forward size={16} aria-hidden="true" className="mr-1" /> Tramitar
                </Button>
              )}
              {aberto && p.can_act && (
                <Button variant="secondary" onClick={() => setAcao("concluir")}>
                  <CheckCheck size={16} aria-hidden="true" className="mr-1" /> Concluir
                </Button>
              )}
              {p.status === "concluido" && p.can_act && (
                <Button variant="secondary" onClick={() => setAcao("arquivar")}>
                  <Archive size={16} aria-hidden="true" className="mr-1" /> Arquivar
                </Button>
              )}
              {!aberto && can("tramite:manage") && (
                <Button variant="secondary" onClick={() => setAcao("reabrir")}>
                  <RotateCcw size={16} aria-hidden="true" className="mr-1" /> Reabrir
                </Button>
              )}
            </div>

            <div className="grid gap-6 lg:grid-cols-[1fr_22rem]">
              <Card>
                <CardHeader className="flex flex-row items-center justify-between">
                  <CardTitle as="h2">Documentos</CardTitle>
                  {aberto && p.can_act && (
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => setDocDialog({ kind: "novo" })}
                    >
                      <FilePen size={14} aria-hidden="true" className="mr-1" /> Novo documento
                    </Button>
                  )}
                </CardHeader>
                <CardContent className="pb-4 pt-3">
                  {p.documentos.length === 0 ? (
                    <p className="text-sm text-muted">Nenhum documento.</p>
                  ) : (
                    <ol className="flex flex-col divide-y divide-surface-border">
                      {p.documentos.map((d, i) => (
                        <li
                          key={d.id}
                          className="flex flex-wrap items-center justify-between gap-2 py-3 text-sm"
                        >
                          <button
                            type="button"
                            className="flex items-center gap-2 text-left hover:underline"
                            onClick={() => setDocDialog({ kind: "ver", doc: d })}
                          >
                            <span className="font-mono text-xs text-muted">{i + 1}.</span>
                            {d.origem === "anexo" ? (
                              <Paperclip size={14} aria-hidden="true" />
                            ) : (
                              <FilePen size={14} aria-hidden="true" />
                            )}
                            <span className="font-medium">{d.titulo}</span>
                            {d.tipo && <span className="text-xs text-muted">({d.tipo})</span>}
                          </button>
                          <span className="flex items-center gap-2">
                            <Badge tone={DOC_STATUS[d.status].tone}>
                              {DOC_STATUS[d.status].label}
                            </Badge>
                            {aberto && p.can_act && d.status === "rascunho" && (
                              <>
                                {d.origem === "redigido" && (
                                  <Button
                                    variant="ghost"
                                    size="sm"
                                    onClick={() => setDocDialog({ kind: "editar", doc: d })}
                                  >
                                    Editar
                                  </Button>
                                )}
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => setDocDialog({ kind: "assinar", doc: d })}
                                >
                                  <PenTool size={14} aria-hidden="true" className="mr-1" /> Assinar
                                </Button>
                              </>
                            )}
                          </span>
                        </li>
                      ))}
                    </ol>
                  )}
                </CardContent>
              </Card>

              <div className="flex flex-col gap-6">
                <AtlasDoProcesso
                  processo={p}
                  documentos={p.documentos}
                  podeClassificar={p.can_act}
                  onClassificado={refresh}
                />
                <Card>
                  <CardHeader>
                    <CardTitle as="h2">Andamento</CardTitle>
                  </CardHeader>
                  <CardContent className="pb-4 pt-3">
                    <ol className="relative flex flex-col gap-4 border-l border-surface-border pl-4">
                      {p.movimentos.map((m) => (
                        <li key={m.id} className="text-sm">
                          <span
                            aria-hidden="true"
                            className="absolute -left-1.5 mt-1.5 h-3 w-3 rounded-full border-2 border-surface bg-primary"
                          />
                          <p className="font-medium capitalize">{m.acao.replace(/_/g, " ")}</p>
                          <p className="text-xs text-muted">
                            {fmtDateTime(m.created_at)} · {m.actor_name ?? "sistema"}
                            {m.para_unidade && ` · ${m.de_unidade ?? ""} → ${m.para_unidade}`}
                          </p>
                          {m.despacho && <p className="mt-1 whitespace-pre-line">{m.despacho}</p>}
                        </li>
                      ))}
                    </ol>
                  </CardContent>
                </Card>

                {p.sigilo !== "publico" && (
                  <Card>
                    <CardHeader>
                      <CardTitle as="h2">Acesso nominal</CardTitle>
                    </CardHeader>
                    <CardContent className="flex flex-col gap-3 pb-4 pt-3 text-sm">
                      <ul className="flex flex-col gap-1">
                        {p.acessos.length === 0 && (
                          <li className="text-muted">Somente as unidades envolvidas.</li>
                        )}
                        {p.acessos.map((a) => (
                          <li key={a.user_id} className="flex items-center justify-between gap-2">
                            <span>
                              {a.name}{" "}
                              <span className="text-xs text-muted">
                                · {fmtDateTime(a.granted_at)}
                              </span>
                            </span>
                            {p.can_act && (
                              <ConfirmButton
                                title={`Revogar o acesso de ${a.name}?`}
                                description="A pessoa deixa de ver o processo. A revogação fica registrada no histórico."
                                confirmLabel="Revogar"
                                aria-label={`Revogar acesso de ${a.name}`}
                                onConfirm={() =>
                                  run(
                                    () =>
                                      apiClient.delete(
                                        `v1/tramite/processos/${p.id}/acessos/${a.user_id}`,
                                      ),
                                    `Acesso de ${a.name} revogado`,
                                  ).then(refresh)
                                }
                              >
                                <UserMinus size={14} aria-hidden="true" />
                              </ConfirmButton>
                            )}
                          </li>
                        ))}
                      </ul>
                      {p.can_act && (
                        <>
                          <UserPicker
                            idPrefix="acesso"
                            label="Conceder acesso a"
                            value={acesso}
                            onChange={setAcesso}
                          />
                          <Button
                            size="sm"
                            variant="secondary"
                            className="self-end"
                            disabled={acesso.length === 0}
                            loading={pending}
                            onClick={async () => {
                              for (const u of acesso)
                                await run(
                                  () =>
                                    apiClient.post(`v1/tramite/processos/${p.id}/acessos`, {
                                      user_id: u.id,
                                    }),
                                  `Acesso concedido a ${u.name}`,
                                );
                              setAcesso([]);
                              refresh();
                            }}
                          >
                            <UserPlus size={14} aria-hidden="true" className="mr-1" /> Conceder
                          </Button>
                        </>
                      )}
                    </CardContent>
                  </Card>
                )}
              </div>
            </div>

            <Dialog
              open={acao !== null}
              onClose={() => setAcao(null)}
              title={acao ? ACAO[acao].title : ""}
              size="lg"
            >
              {acao && (
                <DespachoForm
                  processo={p}
                  acao={acao}
                  onDone={() => {
                    setAcao(null);
                    refresh();
                  }}
                />
              )}
            </Dialog>
            <Dialog
              open={docDialog !== null}
              onClose={() => setDocDialog(null)}
              size="xl"
              title={
                !docDialog
                  ? ""
                  : docDialog.kind === "novo"
                    ? "Novo documento"
                    : docDialog.kind === "editar"
                      ? "Editar documento"
                      : docDialog.kind === "assinar"
                        ? "Solicitar assinatura"
                        : docDialog.doc.titulo
              }
            >
              {docDialog?.kind === "novo" && (
                <DocumentoForm
                  processo={p}
                  onDone={() => {
                    setDocDialog(null);
                    refresh();
                  }}
                />
              )}
              {docDialog?.kind === "editar" && (
                <DocumentoForm
                  processo={p}
                  doc={docDialog.doc}
                  onDone={() => {
                    setDocDialog(null);
                    refresh();
                  }}
                />
              )}
              {docDialog?.kind === "assinar" && (
                <AssinaturaForm
                  doc={docDialog.doc}
                  onDone={() => {
                    setDocDialog(null);
                    refresh();
                  }}
                />
              )}
              {docDialog?.kind === "ver" && <DocumentoView id={docDialog.doc.id} />}
            </Dialog>
          </>
        )}
      </DataState>
    </div>
  );
}
