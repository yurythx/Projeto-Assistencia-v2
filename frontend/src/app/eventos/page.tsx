import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { CalendarDays, MapPin } from "lucide-react";

import { PublicShell } from "@/components/layout/PublicShell";
import { PublicApiError, publicGet } from "@/lib/api/publicServer";
import { APP_URL } from "@/lib/env";
import type { CalendarEvent } from "@/lib/nexus/types";

const description = "Agenda pública de eventos institucionais.";

export const metadata: Metadata = { title: "Agenda", description, alternates: { canonical: `${APP_URL}/eventos` } };

export default async function EventosPage() {
  const from = new Date();
  from.setHours(0, 0, 0, 0);
  const to = new Date(from.getTime() + 90 * 86400_000);
  let events: CalendarEvent[];
  try {
    events = (await publicGet<CalendarEvent[]>(`calendar/public/events?from=${from.toISOString()}&to=${to.toISOString()}`, 300)).data;
  } catch (err) {
    if (err instanceof PublicApiError && err.status === 404) notFound();
    throw err;
  }

  return (
    <PublicShell>
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-8 px-6 py-12">
        <div>
          <p className="dateline">Próximos 90 dias</p>
          <h1 className="mt-2 text-3xl font-bold">Agenda</h1>
          <p className="mt-1 text-muted">{description}</p>
        </div>
        {events.length === 0 ? (
          <p className="rounded-xl border border-dashed border-surface-border p-10 text-center text-muted">Nenhum evento público programado.</p>
        ) : (
          <ol className="flex flex-col gap-3">
            {events.map((e) => (
              <li key={e.id} className="flex gap-4 rounded-xl border border-surface-border bg-surface p-4">
                <div className="flex w-16 shrink-0 flex-col items-center justify-center rounded-lg bg-primary/10 py-2 text-primary">
                  <span className="text-xs uppercase">{new Date(e.starts_at).toLocaleDateString("pt-BR", { month: "short" })}</span>
                  <span className="text-2xl font-bold">{new Date(e.starts_at).getDate()}</span>
                </div>
                <div className="text-sm">
                  <h2 className="text-base font-semibold">{e.title}</h2>
                  <p className="flex items-center gap-1 text-muted">
                    <CalendarDays size={14} aria-hidden="true" />
                    {e.all_day
                      ? "Dia inteiro"
                      : `${new Date(e.starts_at).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" })} – ${new Date(e.ends_at).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" })}`}
                  </p>
                  {(e.room_name || e.location) && (
                    <p className="flex items-center gap-1 text-muted">
                      <MapPin size={14} aria-hidden="true" /> {e.room_name || e.location}
                    </p>
                  )}
                  {e.description && <p className="mt-1">{e.description}</p>}
                </div>
              </li>
            ))}
          </ol>
        )}
      </div>
    </PublicShell>
  );
}
