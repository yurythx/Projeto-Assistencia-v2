"use client";

import Link from "next/link";
import { useState, type FormEvent } from "react";
import { CheckCircle2 } from "lucide-react";

import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { ApiError } from "@/lib/api/client";
import { publicPost } from "@/lib/api/publicClient";

const CATEGORIES = [
  { value: "duvida", label: "Dúvida" },
  { value: "sugestao", label: "Sugestão" },
  { value: "reclamacao", label: "Reclamação" },
  { value: "elogio", label: "Elogio" },
  { value: "outro", label: "Outro" },
];

/**
 * Formulário público do plugin Contato (POST /contact/messages, anônimo,
 * rate-limit por IP no backend). Consentimento LGPD explícito (art. 7º, I)
 * e campo honeypot "website" invisível para humanos.
 */
export function ContactForm({ defaultServiceSlug }: { defaultServiceSlug?: string }) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [protocol, setProtocol] = useState<string | null>(null);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    setPending(true);
    setError(null);
    try {
      const out = await publicPost<{ protocol: string }>("v1/contact/messages", {
        name: String(fd.get("name") ?? "").trim(),
        email: String(fd.get("email") ?? "").trim(),
        phone: String(fd.get("phone") ?? "").trim(),
        subject: String(fd.get("subject") ?? "").trim(),
        category: String(fd.get("category") ?? "duvida"),
        message: String(fd.get("message") ?? "").trim(),
        consent: fd.get("consent") === "on",
        website: String(fd.get("website") ?? ""),
      });
      setProtocol(out.protocol);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Não foi possível enviar sua mensagem. Tente novamente.");
    } finally {
      setPending(false);
    }
  }

  if (protocol) {
    return (
      <div role="status" className="flex flex-col items-center gap-3 py-6 text-center">
        <CheckCircle2 size={36} className="text-success" aria-hidden="true" />
        <h2 className="text-xl font-semibold">Mensagem recebida</h2>
        <p className="text-sm text-muted">
          Guarde o número de protocolo: <strong className="font-mono text-foreground">{protocol}</strong>
        </p>
      </div>
    );
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-4" noValidate={false}>
      {error && (
        <p role="alert" className="rounded-lg bg-danger/10 p-3 text-sm text-danger">
          {error}
        </p>
      )}
      <div className="grid gap-4 sm:grid-cols-2">
        <Input id="contact-name" name="name" label="Nome *" autoComplete="name" required minLength={2} maxLength={150} />
        <Input id="contact-email" name="email" type="email" label="E-mail *" autoComplete="email" required maxLength={200} />
        <Input id="contact-phone" name="phone" type="tel" label="Telefone" autoComplete="tel" maxLength={30} />
        <Select id="contact-category" name="category" label="Tipo" options={CATEGORIES} defaultValue="duvida" />
      </div>
      <Input
        id="contact-subject"
        name="subject"
        label="Assunto *"
        required
        minLength={3}
        maxLength={200}
        defaultValue={defaultServiceSlug ? `Serviço: ${defaultServiceSlug}` : undefined}
      />
      <Textarea id="contact-message" name="message" label="Mensagem *" rows={6} required minLength={10} maxLength={5000} />

      {/* Honeypot: fora da tela e fora da ordem de tabulação. */}
      <div aria-hidden="true" className="absolute -left-[9999px] h-0 w-0 overflow-hidden">
        <label htmlFor="contact-website">Não preencha este campo</label>
        <input id="contact-website" name="website" type="text" tabIndex={-1} autoComplete="off" />
      </div>

      <label className="flex items-start gap-2 text-sm">
        <input type="checkbox" name="consent" required className="mt-1 h-4 w-4 accent-primary" />
        <span>
          Autorizo o tratamento dos meus dados pessoais exclusivamente para responder a esta mensagem, conforme a{" "}
          <Link href="/privacidade" className="text-primary underline underline-offset-2">
            Política de Privacidade
          </Link>{" "}
          (LGPD, art. 7º, I).
        </span>
      </label>

      <div className="flex justify-end">
        <Button type="submit" loading={pending}>
          Enviar mensagem
        </Button>
      </div>
    </form>
  );
}
