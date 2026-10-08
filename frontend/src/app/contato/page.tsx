import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { Mail } from "lucide-react";

import { PublicShell } from "@/components/layout/PublicShell";
import { ContactForm } from "@/components/contact/ContactForm";
import { getFeatures } from "@/lib/features/getFeatures";
import { MODULE } from "@/lib/features/modules";
import { APP_URL } from "@/lib/env";

const description = "Envie sua dúvida, sugestão, reclamação ou elogio — você recebe um número de protocolo para acompanhamento.";

export const metadata: Metadata = {
  title: "Contato",
  description,
  alternates: { canonical: `${APP_URL}/contato` },
  openGraph: { title: "Contato", description, type: "website", url: `${APP_URL}/contato` },
};

export default async function ContatoPage({
  searchParams,
}: {
  searchParams: Promise<{ servico?: string }>;
}) {
  const { servico } = await searchParams;

  // Achado real (auditoria "módulo desativado não pode ficar acessível"):
  // diferente de /blog e /servicos, esta página nunca faz um GET que
  // pudesse 404 sozinho — só renderiza <ContactForm> (POST no submit).
  // Com o módulo desativado, o formulário continuava aparecendo inteiro
  // e "normal", só falhando confuso ao enviar (o backend já barra o
  // POST com 404). Checa aqui, na entrada da página, igual às outras.
  const { features } = await getFeatures();
  if (!features[MODULE.contact]) notFound();

  return (
    <PublicShell>
      <div className="mx-auto flex w-full max-w-2xl flex-col gap-8 px-6 py-14">
        <div className="flex flex-col gap-3">
          <p className="dateline">Contato</p>
          <h1 className="text-3xl font-bold text-foreground sm:text-4xl">Fale com a gente</h1>
          <p className="text-muted">{description}</p>
        </div>

        <div className="rounded-xl border border-surface-border bg-surface p-6 sm:p-8">
          <ContactForm defaultServiceSlug={servico} />
        </div>

        <p className="flex items-center gap-2 text-sm text-muted">
          <Mail size={16} aria-hidden="true" />
          Prefere e-mail? As mensagens enviadas aqui chegam à nossa equipe automaticamente.
        </p>
      </div>
    </PublicShell>
  );
}
