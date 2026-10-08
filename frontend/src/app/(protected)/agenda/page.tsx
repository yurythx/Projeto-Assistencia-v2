"use client";

import { Ban, ChevronLeft, ChevronRight, DoorOpen, MapPin, Pencil, Plus, Trash2 } from "lucide-react";
import { useMemo, useState, type FormEvent } from "react";

import { UnidadeDonaSelect } from "@/components/iam/UnidadeDonaSelect";
import { PublicoAlvoPicker, PublicoBadge, TODOS, publicoVazio } from "@/components/iam/PublicoAlvo";
import { ConfirmButton } from "@/components/nexus/ConfirmButton";
import { DataState } from "@/components/nexus/DataState";
import { PageHeader } from "@/components/nexus/PageHeader";
import { useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Textarea } from "@/components/ui/Textarea";
import { apiClient } from "@/lib/api/client";
import { useApiQuery, withQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import { unidadesGeridas } from "@/lib/nexus/permissions";
import type { CalendarEvent, OrgTree, Room } from "@/lib/nexus/types";

const VIS_LABEL: Record<CalendarEvent["visibility"], string> = { public: "Pública", internal: "Interna", private: "Privada" };

/** "AAAA-MM-DDTHH:mm" no fuso local — formato de <input type="datetime-local">. */
function toLocalInput(iso?: string): string {
  const d = iso ? new Date(iso) : new Date();
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function EventForm({ event, rooms, onDone }: { event?: CalendarEvent; rooms: Room[]; onDone: () => void }) {
  const { run, pending } = useAction();
  const defaultStart = useMemo(() => {
    const d = new Date();
    d.setMinutes(0, 0, 0);
    d.setHours(d.getHours() + 1);
    return d.toISOString();
  }, []);
  const defaultEnd = useMemo(() => new Date(new Date(defaultStart).getTime() + 3600_000).toISOString(), [defaultStart]);
  const [publico, setPublico] = useState(event?.publico ?? TODOS);
  const restrito = !publicoVazio(publico);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const fd = new FormData(e.currentTarget);
    const payload = {
      title: String(fd.get("title") ?? "").trim(),
      description: String(fd.get("description") ?? "").trim(),
      location: String(fd.get("location") ?? "").trim(),
      room_id: String(fd.get("room_id") ?? "") || null,
      starts_at: new Date(String(fd.get("starts_at"))).toISOString(),
      ends_at: new Date(String(fd.get("ends_at"))).toISOString(),
      all_day: fd.get("all_day") === "on",
      visibility: String(fd.get("visibility") ?? "internal"),
      publico,
    };
    const ok = await run(
      () => (event ? apiClient.put(`v1/calendar/events/${event.id}`, payload) : apiClient.post("v1/calendar/events", payload)),
      "Evento salvo",
    );
    if (ok) onDone();
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-3">
      <Input id="ev-title" name="title" label="Título *" required maxLength={200} defaultValue={event?.title} />
      <div className="grid gap-3 sm:grid-cols-2">
        <Input id="ev-start" name="starts_at" type="datetime-local" label="Início *" required defaultValue={toLocalInput(event?.starts_at ?? defaultStart)} />
        <Input id="ev-end" name="ends_at" type="datetime-local" label="Término *" required defaultValue={toLocalInput(event?.ends_at ?? defaultEnd)} />
      </div>
      <div className="grid gap-3 sm:grid-cols-2">
        <Select
          id="ev-room"
          name="room_id"
          label="Sala (reserva)"
          placeholder="Sem reserva de sala"
          defaultValue={event?.room_id ?? ""}
          // Sala desativada não recebe reserva nova, mas o evento que já está
          // nela continua lá: sem a opção, salvar a edição apagava a reserva.
          options={rooms
            .filter((r) => r.active || r.id === event?.room_id)
            .map((r) => ({ value: r.id, label: `${r.name}${r.capacity ? ` · ${r.capacity} lugares` : ""}${r.active ? "" : " (inativa)"}` }))}
        />
        <Input id="ev-location" name="location" label="Local / link" maxLength={200} defaultValue={event?.location} />
      </div>
      <div className="grid gap-3 sm:grid-cols-2">
        {/* Com público-alvo (ADR 014) o evento não vai ao site: sem "Pública". */}
        <Select
          key={`vis-${restrito}`}
          id="ev-vis"
          name="visibility"
          label="Visibilidade"
          defaultValue={event?.visibility ?? "internal"}
          options={[
            { value: "internal", label: restrito ? "Interna (só o público-alvo)" : "Interna (servidores)" },
            ...(restrito ? [] : [{ value: "public", label: "Pública (site institucional)" }]),
            { value: "private", label: "Privada (só eu)" },
          ]}
        />
        <label className="flex items-center gap-2 self-end pb-2 text-sm">
          <input type="checkbox" name="all_day" defaultChecked={event?.all_day} className="h-4 w-4 accent-primary" /> Dia inteiro
        </label>
      </div>
      <PublicoAlvoPicker idPrefix="ev-publico" value={publico} onChange={setPublico} />
      <Textarea id="ev-desc" name="description" label="Descrição" rows={3} maxLength={5000} defaultValue={event?.description} />
      <p className="text-xs text-muted">Conflitos de horário na mesma sala são recusados pelo servidor.</p>
      <div className="flex justify-end">
        <Button type="submit" loading={pending}>
          Salvar evento
        </Button>
      </div>
    </form>
  );
}

function RoomsManager({ rooms, gere, onChanged }: { rooms: Room[]; gere: (unidade?: string) => boolean; onChanged: () => void }) {
  const { run, pending } = useAction();
  const [edit, setEdit] = useState<Room | null>(null);

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = e.currentTarget;
    const fd = new FormData(form);
    const payload = {
      name: String(fd.get("name") ?? "").trim(),
      location: String(fd.get("location") ?? "").trim(),
      capacity: Number(fd.get("capacity") ?? 0) || 0,
      resources: String(fd.get("resources") ?? "").split(",").map((s) => s.trim()).filter(Boolean),
      active: fd.get("active") === "on",
      unidade_id: String(fd.get("unidade_id") ?? "") || null,
    };
    const ok = await run(() => (edit ? apiClient.put(`v1/calendar/rooms/${edit.id}`, payload) : apiClient.post("v1/calendar/rooms", payload)), "Sala salva");
    if (ok) {
      setEdit(null);
      form.reset();
      onChanged();
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <ul className="flex flex-col divide-y divide-surface-border rounded-lg border border-surface-border">
        {rooms.length === 0 && <li className="p-3 text-sm text-muted">Nenhuma sala cadastrada.</li>}
        {rooms.map((r) => (
          <li key={r.id} className="flex items-center justify-between gap-2 px-3 py-2 text-sm">
            <span>
              <strong>{r.name}</strong> {r.location && <span className="text-muted">· {r.location}</span>} · {r.capacity} lugares
              {!r.active && <Badge tone="warning" className="ml-1">Inativa</Badge>}
              {r.resources.length > 0 && <span className="block text-xs text-muted">{r.resources.join(", ")}</span>}
            </span>
            {gere(r.unidade_id) && (
              <span className="flex gap-1">
                <Button variant="ghost" size="sm" aria-label={`Editar ${r.name}`} onClick={() => setEdit(r)}>
                  <Pencil size={14} aria-hidden="true" />
                </Button>
                <ConfirmButton aria-label={`Excluir ${r.name}`} title={`Excluir a sala "${r.name}"?`} confirmLabel="Excluir" onConfirm={() => run(() => apiClient.delete(`v1/calendar/rooms/${r.id}`), "Sala excluída").then(onChanged)}>
                  <Trash2 size={14} aria-hidden="true" />
                </ConfirmButton>
              </span>
            )}
          </li>
        ))}
      </ul>
      <form key={edit?.id ?? "new"} onSubmit={submit} className="flex flex-col gap-3 rounded-lg border border-dashed border-surface-border p-3">
        <p className="text-sm font-medium">{edit ? `Editar ${edit.name}` : "Nova sala"}</p>
        <div className="grid gap-3 sm:grid-cols-3">
          <Input id="room-name" name="name" label="Nome *" required maxLength={120} defaultValue={edit?.name} />
          <Input id="room-location" name="location" label="Localização" maxLength={200} defaultValue={edit?.location} />
          <Input id="room-capacity" name="capacity" type="number" min={0} label="Capacidade" defaultValue={edit?.capacity ?? 0} />
        </div>
        <UnidadeDonaSelect id="room-unidade" permission="calendar:manage" defaultValue={edit?.unidade_id} />
        <Input id="room-resources" name="resources" label="Recursos (separados por vírgula)" defaultValue={edit?.resources.join(", ")} placeholder="projetor, videoconferência" />
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" name="active" defaultChecked={edit?.active ?? true} className="h-4 w-4 accent-primary" /> Ativa
        </label>
        <div className="flex justify-end gap-2">
          {edit && (
            <Button type="button" variant="ghost" onClick={() => setEdit(null)}>
              Cancelar
            </Button>
          )}
          <Button type="submit" loading={pending}>
            Salvar sala
          </Button>
        </div>
      </form>
    </div>
  );
}

export default function AgendaPage() {
  const { me, can } = useNexus();
  const [month, setMonth] = useState(() => {
    const d = new Date();
    return new Date(d.getFullYear(), d.getMonth(), 1);
  });
  const [roomFilter, setRoomFilter] = useState("");
  const [editing, setEditing] = useState<CalendarEvent | "new" | null>(null);
  const [roomsOpen, setRoomsOpen] = useState(false);
  const { run } = useAction();

  const from = month.toISOString();
  const to = new Date(month.getFullYear(), month.getMonth() + 1, 1).toISOString();
  const events = useApiQuery<CalendarEvent[]>(withQuery("v1/calendar/events", { from, to, room_id: roomFilter }));
  const rooms = useApiQuery<Room[]>("v1/calendar/rooms");
  // calendar:manage vale na unidade dona da sala (ADR 013); sala e evento
  // sem dona são institucionais — só a gestão global.
  const tree = useApiQuery<OrgTree[]>(can("calendar:manage") ? "v1/iam/org-tree" : null);
  const geridas = unidadesGeridas(me, tree.data, "calendar:manage");
  const gere = (unidade?: string) => can("calendar:manage") && (geridas.global || (!!unidade && geridas.unidades.has(unidade)));

  const byDay = useMemo(() => {
    const map = new Map<string, CalendarEvent[]>();
    for (const e of events.data ?? []) {
      const key = new Date(e.starts_at).toLocaleDateString("pt-BR", { weekday: "long", day: "2-digit", month: "long" });
      map.set(key, [...(map.get(key) ?? []), e]);
    }
    return [...map.entries()];
  }, [events.data]);

  const canEdit = (e: CalendarEvent) => e.organizer_id === me?.id || gere(rooms.data?.find((r) => r.id === e.room_id)?.unidade_id);
  const monthLabel = month.toLocaleDateString("pt-BR", { month: "long", year: "numeric" });

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        eyebrow="Agenda"
        title="Eventos e reservas de salas"
        actions={
          <>
            {can("calendar:manage") && (
              <Button variant="secondary" onClick={() => setRoomsOpen(true)}>
                <DoorOpen size={16} aria-hidden="true" className="mr-1" /> Salas
              </Button>
            )}
            <Button onClick={() => setEditing("new")}>
              <Plus size={16} aria-hidden="true" className="mr-1" /> Novo evento
            </Button>
          </>
        }
      />

      <div className="flex flex-wrap items-end justify-between gap-3">
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="sm" aria-label="Mês anterior" onClick={() => setMonth(new Date(month.getFullYear(), month.getMonth() - 1, 1))}>
            <ChevronLeft size={16} aria-hidden="true" />
          </Button>
          <h2 className="min-w-44 text-center text-lg font-semibold capitalize" aria-live="polite">
            {monthLabel}
          </h2>
          <Button variant="ghost" size="sm" aria-label="Próximo mês" onClick={() => setMonth(new Date(month.getFullYear(), month.getMonth() + 1, 1))}>
            <ChevronRight size={16} aria-hidden="true" />
          </Button>
        </div>
        <Select
          id="agenda-room"
          label="Sala"
          value={roomFilter}
          onChange={(e) => setRoomFilter(e.target.value)}
          options={[{ value: "", label: "Todas" }, ...(rooms.data ?? []).map((r) => ({ value: r.id, label: r.name }))]}
        />
      </div>

      <DataState loading={events.isLoading} error={events.error} onRetry={() => void events.mutate()} empty={byDay.length === 0} emptyTitle="Nenhum evento neste mês">
        <ol className="flex flex-col gap-5">
          {byDay.map(([day, list]) => (
            <li key={day}>
              <h3 className="mb-2 text-sm font-semibold capitalize text-muted">{day}</h3>
              <ul className="flex flex-col gap-2">
                {list.map((e) => (
                  <li key={e.id} className={`flex flex-wrap items-start justify-between gap-3 rounded-lg border border-surface-border bg-surface p-3 ${e.status === "cancelled" ? "opacity-60" : ""}`}>
                    <div className="flex gap-3">
                      <div className="w-24 shrink-0 text-sm font-mono">
                        {e.all_day
                          ? "Dia inteiro"
                          : `${new Date(e.starts_at).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" })}–${new Date(e.ends_at).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" })}`}
                      </div>
                      <div>
                        <p className={`font-medium ${e.status === "cancelled" ? "line-through" : ""}`}>{e.title}</p>
                        <p className="flex flex-wrap items-center gap-x-2 text-xs text-muted">
                          {(e.room_name || e.location) && (
                            <span className="inline-flex items-center gap-1">
                              <MapPin size={12} aria-hidden="true" /> {e.room_name ?? e.location}
                            </span>
                          )}
                          <span>{e.organizer_name}</span>
                          <Badge>{VIS_LABEL[e.visibility]}</Badge>
                          <PublicoBadge publico={e.publico} />
                          {e.status === "cancelled" && <Badge tone="danger">Cancelado</Badge>}
                        </p>
                        {e.description && <p className="mt-1 text-sm text-muted">{e.description}</p>}
                      </div>
                    </div>
                    {canEdit(e) && e.status !== "cancelled" && (
                      <span className="flex gap-1">
                        <Button variant="ghost" size="sm" aria-label={`Editar ${e.title}`} onClick={() => setEditing(e)}>
                          <Pencil size={14} aria-hidden="true" />
                        </Button>
                        <ConfirmButton
                          aria-label={`Cancelar ${e.title}`}
                          title="Cancelar evento?"
                          description="O evento continua visível como cancelado e a sala é liberada."
                          confirmLabel="Cancelar evento"
                          onConfirm={() => run(() => apiClient.post(`v1/calendar/events/${e.id}/cancel`), "Evento cancelado").then(() => events.mutate())}
                        >
                          <Ban size={14} aria-hidden="true" />
                        </ConfirmButton>
                        <ConfirmButton
                          aria-label={`Excluir ${e.title}`}
                          title="Excluir evento?"
                          confirmLabel="Excluir"
                          onConfirm={() => run(() => apiClient.delete(`v1/calendar/events/${e.id}`), "Evento excluído").then(() => events.mutate())}
                        >
                          <Trash2 size={14} aria-hidden="true" />
                        </ConfirmButton>
                      </span>
                    )}
                  </li>
                ))}
              </ul>
            </li>
          ))}
        </ol>
      </DataState>

      <Dialog open={editing !== null} onClose={() => setEditing(null)} title={editing === "new" ? "Novo evento" : "Editar evento"} size="lg">
        {editing && (
          <EventForm
            key={editing === "new" ? "new" : editing.id}
            event={editing === "new" ? undefined : editing}
            rooms={rooms.data ?? []}
            onDone={() => {
              setEditing(null);
              void events.mutate();
            }}
          />
        )}
      </Dialog>
      <Dialog open={roomsOpen} onClose={() => setRoomsOpen(false)} title="Salas de reunião" size="xl">
        {roomsOpen && <RoomsManager rooms={rooms.data ?? []} gere={gere} onChanged={() => void rooms.mutate()} />}
      </Dialog>
    </div>
  );
}
