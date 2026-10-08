import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ArrowRight, LayoutGrid } from "lucide-react";

import { PublicShell } from "@/components/layout/PublicShell";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/Card";
import { PublicApiError } from "@/lib/api/publicServer";
import { listPublishedServices } from "@/lib/catalog/catalog";
import { APP_URL } from "@/lib/env";
import { ServiceIcon } from "@/lib/catalog/icons";

const description = "Carta de serviços da instituição: o que é oferecido, para quem, requisitos, etapas, prazos e canais de atendimento.";

export const metadata: Metadata = {
  title: "Serviços",
  description,
  alternates: { canonical: `${APP_URL}/servicos` },
  openGraph: { title: "Serviços", description, type: "website", url: `${APP_URL}/servicos` },
};

export default async function ServicosPage() {
  // Módulo "catalog" desativado no Kernel → o Guard responde 404 e a
  // página inteira deixa de existir (não só a lista vazia).
  let services: Awaited<ReturnType<typeof listPublishedServices>>;
  try {
    services = await listPublishedServices();
  } catch (err) {
    if (err instanceof PublicApiError && err.status === 404) notFound();
    throw err;
  }

  return (
    <PublicShell>
      <div className="mx-auto flex w-full max-w-5xl flex-col gap-10 px-6 py-12">
        <div className="flex flex-col gap-2">
          <p className="dateline">Central de serviços</p>
          <h1 className="text-3xl font-bold text-foreground sm:text-4xl">Nossos serviços</h1>
          <p className="max-w-2xl text-muted">{description}</p>
        </div>

        {services.length === 0 ? (
          <div className="flex flex-col items-center gap-3 rounded-xl border border-dashed border-surface-border py-16 text-center text-muted">
            <LayoutGrid size={32} aria-hidden="true" />
            <p className="max-w-md">
              Nenhum serviço publicado ainda. O catálogo é mantido pela equipe responsável na área administrativa (permissão catalog:manage).
            </p>
          </div>
        ) : (
          <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
            {services.map((svc) => (
                <Link key={svc.id} href={`/servicos/${svc.slug}`} className="block h-full">
                  <Card className="flex h-full flex-col transition-shadow hover:shadow-md">
                    <CardHeader className="flex flex-col gap-3">
                      <span className="inline-flex h-10 w-10 items-center justify-center rounded-lg bg-accent/10 text-accent">
                        <ServiceIcon name={svc.icon} size={20} aria-hidden="true" />
                      </span>
                      {svc.category && <span className="text-xs font-medium text-muted">{svc.category}</span>}
                      <CardTitle as="h2" className="text-lg">
                        {svc.title}
                      </CardTitle>
                    </CardHeader>
                    <CardContent className="flex flex-1 flex-col gap-3">
                      <CardDescription className="flex-1">{svc.summary}</CardDescription>
                      <span className="inline-flex items-center gap-1 text-sm font-semibold text-accent">
                        Saiba mais <ArrowRight size={14} aria-hidden="true" />
                      </span>
                    </CardContent>
                  </Card>
                </Link>
            ))}
          </div>
        )}
      </div>
    </PublicShell>
  );
}
