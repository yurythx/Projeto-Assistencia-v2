import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ArrowLeft, Clock, Globe, Mail, MapPin, Phone, Wallet } from "lucide-react";

import { PublicShell } from "@/components/layout/PublicShell";
import { Markdown } from "@/components/nexus/Markdown";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { getPublishedService } from "@/lib/catalog/catalog";
import { ServiceIcon } from "@/lib/catalog/icons";
import { APP_URL } from "@/lib/env";
import { safeResourceUrl } from "@/lib/security/safe-url";
import type { ServiceChannel } from "@/lib/nexus/types";

type Props = { params: Promise<{ slug: string }> };

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const svc = await getPublishedService((await params).slug);
  if (!svc) return { title: "Serviço não encontrado" };
  return {
    title: svc.title,
    description: svc.summary,
    alternates: { canonical: `${APP_URL}/servicos/${svc.slug}` },
  };
}

const CHANNEL_ICON = { online: Globe, presencial: MapPin, telefone: Phone, email: Mail } as const;

function ChannelValue({ c }: { c: ServiceChannel }) {
  if (c.type === "online") {
    const href = safeResourceUrl(c.value);
    return href ? (
      <a href={href} target="_blank" rel="noopener noreferrer" className="text-primary underline underline-offset-2">
        {c.value}
      </a>
    ) : (
      <span>{c.value}</span>
    );
  }
  if (c.type === "email") return <a href={`mailto:${c.value}`} className="text-primary underline underline-offset-2">{c.value}</a>;
  if (c.type === "telefone") return <a href={`tel:${c.value.replace(/[^\d+]/g, "")}`} className="text-primary underline underline-offset-2">{c.value}</a>;
  return <span>{c.value}</span>;
}

export default async function ServicoPage({ params }: Props) {
  const svc = await getPublishedService((await params).slug);
  if (!svc) notFound();

  return (
    <PublicShell>
      <article className="mx-auto flex w-full max-w-4xl flex-col gap-8 px-6 py-12">
        <Link href="/servicos" className="inline-flex items-center gap-1 text-sm text-muted hover:text-foreground">
          <ArrowLeft size={14} aria-hidden="true" /> Todos os serviços
        </Link>

        <header className="flex items-start gap-4">
          <span className="inline-flex h-12 w-12 shrink-0 items-center justify-center rounded-lg bg-accent/10 text-accent">
            <ServiceIcon name={svc.icon} size={24} aria-hidden="true" />
          </span>
          <div className="flex flex-col gap-2">
            {svc.category && <p className="dateline">{svc.category}</p>}
            <h1 className="text-3xl font-bold text-foreground">{svc.title}</h1>
            <p className="text-muted">{svc.summary}</p>
          </div>
        </header>

        <dl className="grid gap-4 sm:grid-cols-3">
          {svc.audience && (
            <div className="rounded-lg border border-surface-border p-4">
              <dt className="text-xs font-semibold uppercase tracking-wider text-muted">Público</dt>
              <dd className="mt-1 text-sm">{svc.audience}</dd>
            </div>
          )}
          {svc.sla && (
            <div className="rounded-lg border border-surface-border p-4">
              <dt className="flex items-center gap-1 text-xs font-semibold uppercase tracking-wider text-muted"><Clock size={12} aria-hidden="true" /> Prazo</dt>
              <dd className="mt-1 text-sm">{svc.sla}</dd>
            </div>
          )}
          <div className="rounded-lg border border-surface-border p-4">
            <dt className="flex items-center gap-1 text-xs font-semibold uppercase tracking-wider text-muted"><Wallet size={12} aria-hidden="true" /> Custo</dt>
            <dd className="mt-1 text-sm">{svc.cost || "Gratuito"}</dd>
          </div>
        </dl>

        {svc.description && <Markdown source={svc.description} />}

        {svc.requirements.length > 0 && (
          <section aria-labelledby="req">
            <h2 id="req" className="text-xl font-semibold">O que é necessário</h2>
            <ul className="mt-3 ml-6 list-disc space-y-1">{svc.requirements.map((r) => <li key={r}>{r}</li>)}</ul>
          </section>
        )}

        {svc.steps.length > 0 && (
          <section aria-labelledby="steps">
            <h2 id="steps" className="text-xl font-semibold">Etapas</h2>
            <ol className="mt-3 ml-6 list-decimal space-y-1">{svc.steps.map((s) => <li key={s}>{s}</li>)}</ol>
          </section>
        )}

        {svc.channels.length > 0 && (
          <Card>
            <CardHeader>
              <CardTitle as="h2">Canais de atendimento</CardTitle>
            </CardHeader>
            <CardContent>
              <ul className="flex flex-col gap-3 pb-4">
                {svc.channels.map((c, i) => {
                  const Icon = CHANNEL_ICON[c.type];
                  return (
                    <li key={i} className="flex items-start gap-3 text-sm">
                      <Icon size={16} aria-hidden="true" className="mt-0.5 text-muted" />
                      <span className="flex flex-col">
                        <span className="font-medium">{c.label}</span>
                        <ChannelValue c={c} />
                      </span>
                    </li>
                  );
                })}
              </ul>
            </CardContent>
          </Card>
        )}

        {svc.responsible_unidade && <p className="text-sm text-muted">Unidade responsável: {svc.responsible_unidade}</p>}

        <Link href={`/contato?servico=${encodeURIComponent(svc.slug)}`} className="text-sm font-semibold text-primary hover:underline">
          Dúvidas sobre este serviço? Fale conosco →
        </Link>
      </article>
    </PublicShell>
  );
}
