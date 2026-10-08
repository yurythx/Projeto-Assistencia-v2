"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { ArrowLeft, Ban, CheckCircle2, ExternalLink, PenTool, ShieldCheck, XCircle } from "lucide-react";
import { useState, type FormEvent } from "react";

import { ConfirmButton } from "@/components/nexus/ConfirmButton";
import { DataState } from "@/components/nexus/DataState";
import { fmtDateTime, useAction } from "@/components/nexus/useAction";
import { ENVELOPE_STATUS, SIGNER_STATUS } from "@/components/signum/envelopeStatus";
import { FileHash } from "@/components/signum/FileHash";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient, ApiError } from "@/lib/api/client";
import { useApiQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { Challenge, Envelope } from "@/lib/nexus/types";

/**
 * Cerimônia de assinatura (skill §6.2 — Signum):
 *  1. o signatário escolhe o documento e o navegador calcula o SHA-256;
 *  2. o hash precisa bater com o do envelope (integridade);
 *  3. o servidor emite um desafio de uso único (nonce com validade curta);
 *  4. a senha é reconferida (conta local: Argon2id; federada: Keycloak → AD)
 *     e a assinatura registra hash, método e horário.
 */
function SignCeremony({ envelope, onDone }: { envelope: Envelope; onDone: () => void }) {
  const { run, pending } = useAction();
  const [hash, setHash] = useState("");
  const [challenge, setChallenge] = useState<Challenge | null>(null);
  const [password, setPassword] = useState("");
  const matches = hash === envelope.document_sha256;

  async function start() {
    const res = await run(() => apiClient.post<Challenge>(`v1/signum/envelopes/${envelope.id}/challenge`));
    if (res) setChallenge(res.data);
  }

  async function sign(e: FormEvent) {
    e.preventDefault();
    if (!challenge) return;
    const res = await run(
      () =>
        apiClient
          .post(`v1/signum/envelopes/${envelope.id}/sign`, {
            challenge_id: challenge.challenge_id,
            nonce: challenge.nonce,
            password,
            confirm_document_sha256: hash,
          })
          .catch((err: unknown) => {
            // Senha errada não consome o desafio (tenta de novo com ele).
            // Qualquer outra recusa — desafio expirado ou já usado, excesso
            // de tentativas — o invalida: libera gerar outro em vez de
            // travar a cerimônia.
            if (!(err instanceof ApiError && err.code === "SIGNUM_REAUTH_FAILED")) setChallenge(null);
            throw err;
          }),
      "Documento assinado",
    );
    setPassword("");
    if (res) onDone();
  }

  return (
    <div className="flex flex-col gap-4">
      <ol className="flex flex-col gap-4">
        <li className="flex flex-col gap-2">
          <p className="text-sm font-semibold">1. Confira o documento</p>
          <FileHash id="sign-file" label="Selecione o arquivo que você vai assinar" onHash={(h) => setHash(h)} />
          {hash && (
            <p role="status" className={`flex items-center gap-1 text-sm ${matches ? "text-success" : "text-danger"}`}>
              {matches ? <CheckCircle2 size={14} aria-hidden="true" /> : <XCircle size={14} aria-hidden="true" />}
              {matches ? "Documento íntegro — idêntico ao do envelope." : "Este arquivo NÃO é o documento do envelope."}
            </p>
          )}
        </li>
        <li className="flex flex-col gap-2">
          <p className="text-sm font-semibold">2. Inicie a cerimônia</p>
          <Button variant="secondary" className="self-start" disabled={!matches || !!challenge} loading={pending && !challenge} onClick={() => void start()}>
            Gerar desafio de assinatura
          </Button>
          {challenge && <p className="text-xs text-muted">Desafio válido até {fmtDateTime(challenge.expires_at)}.</p>}
        </li>
        <li>
          <form onSubmit={sign} className="flex flex-col gap-2">
            <p className="text-sm font-semibold">3. Reautentique-se</p>
            <Input
              id="sign-password"
              type="password"
              label="Sua senha"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              disabled={!challenge}
              required
              autoComplete="current-password"
            />
            <Button type="submit" className="self-end" disabled={!challenge || !password} loading={pending && !!challenge}>
              <PenTool size={16} aria-hidden="true" className="mr-1" /> Assinar
            </Button>
          </form>
        </li>
      </ol>
    </div>
  );
}

export default function EnvelopePage() {
  const { id } = useParams<{ id: string }>();
  const { me, can } = useNexus();
  const env = useApiQuery<Envelope>(`v1/signum/envelopes/${id}`);
  const { run, pending } = useAction();
  const [mode, setMode] = useState<"sign" | "refuse" | null>(null);
  const [reason, setReason] = useState("");
  const e = env.data;

  const mySigner = e?.signers.find((s) => s.user_id === me?.id);
  const firstPending = e?.signers.filter((s) => s.status === "pending").sort((a, b) => a.position - b.position)[0];
  const myTurn = e?.status === "pending" && mySigner?.status === "pending" && (!e.sequential || firstPending?.id === mySigner.id);
  const canCancel = e?.status === "pending" && (e.created_by === me?.id || can("signum:manage"));

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6">
      <Link href="/signum" className="inline-flex items-center gap-1 text-sm text-muted hover:text-foreground">
        <ArrowLeft size={14} aria-hidden="true" /> Envelopes
      </Link>
      <DataState loading={env.isLoading} error={env.error} empty={!e}>
        {e && (
          <>
            <header className="flex flex-col gap-2">
              <div className="flex flex-wrap items-center gap-2">
                <h1 className="text-2xl font-semibold">{e.title}</h1>
                <Badge tone={ENVELOPE_STATUS[e.status].tone}>{ENVELOPE_STATUS[e.status].label}</Badge>
              </div>
              {e.description && <p className="text-muted">{e.description}</p>}
              <p className="text-xs text-muted">
                Aberto por {e.created_by_name} em {fmtDateTime(e.created_at)}
                {e.completed_at && ` · concluído em ${fmtDateTime(e.completed_at)}`}
                {e.sequential && " · assinatura sequencial"}
              </p>
              <p className="break-all font-mono text-[11px] text-muted">SHA-256 do documento: {e.document_sha256}</p>
            </header>

            <div className="flex flex-wrap gap-2">
              {myTurn && (
                <>
                  <Button onClick={() => setMode("sign")}>
                    <PenTool size={16} aria-hidden="true" className="mr-1" /> Assinar
                  </Button>
                  <Button variant="secondary" onClick={() => setMode("refuse")}>
                    <XCircle size={16} aria-hidden="true" className="mr-1" /> Recusar
                  </Button>
                </>
              )}
              {canCancel && (
                <ConfirmButton
                  variant="ghost"
                  size="md"
                  title="Cancelar envelope?"
                  description="Assinaturas já feitas permanecem registradas, mas o envelope não poderá ser concluído."
                  confirmLabel="Cancelar envelope"
                  onConfirm={() => run(() => apiClient.post(`v1/signum/envelopes/${e.id}/cancel`), "Envelope cancelado").then(() => env.mutate())}
                >
                  <Ban size={16} aria-hidden="true" className="mr-1" /> Cancelar envelope
                </ConfirmButton>
              )}
              <a href={`/verificar/${e.id}`} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 rounded-lg px-4 py-2 text-sm text-primary hover:underline">
                <ShieldCheck size={16} aria-hidden="true" /> Página pública de verificação <ExternalLink size={12} aria-hidden="true" />
              </a>
            </div>

            <Card>
              <CardHeader>
                <CardTitle as="h2">Signatários</CardTitle>
              </CardHeader>
              <CardContent className="pb-4 pt-3">
                <ol className="flex flex-col divide-y divide-surface-border">
                  {[...e.signers]
                    .sort((a, b) => a.position - b.position)
                    .map((s) => (
                      <li key={s.id} className="flex flex-wrap items-start justify-between gap-2 py-3 text-sm">
                        <div>
                          <p className="font-medium">
                            <span className="mr-2 font-mono text-xs text-muted">{s.position}.</span>
                            {s.name}
                          </p>
                          {s.signed_at && (
                            <p className="text-xs text-muted">
                              {fmtDateTime(s.signed_at)} · método: {s.method}
                            </p>
                          )}
                          {s.signature_hash && <p className="break-all font-mono text-[10px] text-muted">{s.signature_hash}</p>}
                          {s.reason && <p className="text-xs text-danger">Motivo: {s.reason}</p>}
                        </div>
                        <Badge tone={SIGNER_STATUS[s.status].tone}>{SIGNER_STATUS[s.status].label}</Badge>
                      </li>
                    ))}
                </ol>
              </CardContent>
            </Card>

            <Dialog open={mode === "sign"} onClose={() => setMode(null)} title="Assinar documento" size="lg">
              {mode === "sign" && (
                <SignCeremony
                  envelope={e}
                  onDone={() => {
                    setMode(null);
                    void env.mutate();
                  }}
                />
              )}
            </Dialog>
            <Dialog
              open={mode === "refuse"}
              onClose={() => setMode(null)}
              title="Recusar assinatura"
              footer={
                <Button
                  variant="danger"
                  className="ml-auto"
                  loading={pending}
                  disabled={reason.trim().length < 3}
                  onClick={() =>
                    void run(() => apiClient.post(`v1/signum/envelopes/${e.id}/refuse`, { reason: reason.trim() }), "Recusa registrada").then((ok) => {
                      if (ok === undefined) return;
                      setMode(null);
                      void env.mutate();
                    })
                  }
                >
                  Recusar
                </Button>
              }
            >
              <Textarea id="refuse-reason" label="Motivo *" rows={3} maxLength={1000} value={reason} onChange={(ev) => setReason(ev.target.value)} />
            </Dialog>
          </>
        )}
      </DataState>
    </div>
  );
}
