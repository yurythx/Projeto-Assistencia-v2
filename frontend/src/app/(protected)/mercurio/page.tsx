"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { Hash, Pencil, Send, Settings2, Trash2, Users, User as UserIcon } from "lucide-react";
import { Suspense, useCallback, useEffect, useRef, useState, type FormEvent, type KeyboardEvent } from "react";

import { RoomsAdmin } from "@/components/mercurio/RoomsAdmin";
import { useFrames, useRealtimeSend, useRealtimeState, useTopic } from "@/components/realtime/RealtimeProvider";
import { DataState } from "@/components/nexus/DataState";
import { useAction } from "@/components/nexus/useAction";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { apiClient } from "@/lib/api/client";
import { useApiQuery, withQuery } from "@/lib/api/swr";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { ChatMessage, ChatRoom } from "@/lib/nexus/types";

const KIND_ICON = { global: Hash, department: Users, direct: UserIcon } as const;
const TYPING_TTL = 4000;

function RoomView({ room, onRead }: { room: ChatRoom; onRead: () => void }) {
  const { me, can } = useNexus();
  const { run } = useAction();
  const send = useRealtimeSend();
  const connection = useRealtimeState();
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [draft, setDraft] = useState("");
  const [editing, setEditing] = useState<ChatMessage | null>(null);
  const [typing, setTyping] = useState<Record<string, { name: string; at: number }>>({});
  // O servidor derrubou a assinatura (sala arquivada ou acesso removido).
  const [dropped, setDropped] = useState(false);
  const bottom = useRef<HTMLDivElement>(null);
  const lastTypingSent = useRef(0);

  const pageUrl = useCallback(
    (before?: string) => withQuery(`v1/mercurio/rooms/${room.id}/messages`, { before, limit: 50 }),
    [room.id],
  );

  /** Mensagens anteriores (API devolve as mais recentes primeiro). */
  async function loadOlder() {
    const { data } = await apiClient.get<ChatMessage[]>(pageUrl(messages[0]?.created_at));
    setHasMore((data ?? []).length === 50);
    setMessages((prev) => [...[...(data ?? [])].reverse(), ...prev]);
  }

  useEffect(() => {
    let active = true;
    apiClient
      .get<ChatMessage[]>(pageUrl())
      .then(({ data }) => {
        if (!active) return;
        setHasMore((data ?? []).length === 50);
        setMessages([...(data ?? [])].reverse());
        void apiClient.post(`v1/mercurio/rooms/${room.id}/read`).then(onRead).catch(() => undefined);
      })
      .catch(() => undefined)
      .finally(() => active && setLoading(false));
    return () => {
      active = false;
    };
  }, [pageUrl, room.id, onRead]);

  useEffect(() => {
    bottom.current?.scrollIntoView({ block: "end" });
  }, [messages.length]);

  useTopic(`mercurio:room:${room.id}`, (f) => {
    if (f.type === "mercurio.message") {
      const m = f.data as ChatMessage;
      setMessages((prev) => (prev.some((x) => x.id === m.id) ? prev : [...prev, m]));
      setTyping((t) => {
        const rest = { ...t };
        delete rest[m.author_id];
        return rest;
      });
      void apiClient.post(`v1/mercurio/rooms/${room.id}/read`).catch(() => undefined);
    } else if (f.type === "mercurio.message.edited") {
      const m = f.data as ChatMessage;
      setMessages((prev) => prev.map((x) => (x.id === m.id ? m : x)));
    } else if (f.type === "mercurio.message.deleted") {
      const { id } = f.data as { id: string };
      setMessages((prev) => prev.map((x) => (x.id === id ? { ...x, deleted: true, body: "" } : x)));
    } else if (f.type === "topic.dropped") {
      setDropped(true);
    } else if (f.type === "mercurio.typing") {
      const t = f.data as { user_id: string; username: string };
      if (t.user_id !== me?.id) setTyping((prev) => ({ ...prev, [t.user_id]: { name: t.username, at: Date.now() } }));
    }
  });

  useEffect(() => {
    const id = setInterval(() => {
      setTyping((t) => {
        const now = Date.now();
        const kept = Object.fromEntries(Object.entries(t).filter(([, v]) => now - v.at < TYPING_TTL));
        return Object.keys(kept).length === Object.keys(t).length ? t : kept;
      });
    }, 1000);
    return () => clearInterval(id);
  }, []);

  async function submit(e?: FormEvent) {
    e?.preventDefault();
    const body = draft.trim();
    if (!body) return;
    if (editing) {
      const res = await run(() => apiClient.patch<ChatMessage>(`v1/mercurio/messages/${editing.id}`, { body }));
      if (res) {
        setMessages((prev) => prev.map((x) => (x.id === res.data.id ? res.data : x)));
        setEditing(null);
        setDraft("");
      }
      return;
    }
    const res = await run(() => apiClient.post<ChatMessage>(`v1/mercurio/rooms/${room.id}/messages`, { body }));
    if (res) {
      setMessages((prev) => (prev.some((x) => x.id === res.data.id) ? prev : [...prev, res.data]));
      setDraft("");
    }
  }

  function onKey(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      void submit();
    } else if (e.key === "Escape" && editing) {
      setEditing(null);
      setDraft("");
    } else if (Date.now() - lastTypingSent.current > 2500) {
      lastTypingSent.current = Date.now();
      send({ type: "mercurio.typing", data: { room_id: room.id } });
    }
  }

  const typingNames = Object.values(typing).map((t) => t.name);
  const Icon = KIND_ICON[room.kind];

  return (
    <section aria-label={`Sala ${room.name}`} className="flex h-full min-h-0 flex-col">
      <header className="flex items-center gap-2 border-b border-surface-border px-4 py-3">
        <Icon size={18} aria-hidden="true" className="text-muted" />
        <h2 className="font-semibold">{room.name}</h2>
        {room.description && <p className="hidden truncate text-xs text-muted md:block">· {room.description}</p>}
        {connection !== "open" && <Badge tone="warning" className="ml-auto">Tempo real {connection === "connecting" ? "conectando…" : "offline"}</Badge>}
      </header>
      {dropped && (
        <p role="status" className="border-b border-surface-border bg-warning/10 px-4 py-2 text-sm">
          Esta sala não recebe mais mensagens em tempo real para você: ela foi arquivada ou seu acesso foi removido.
        </p>
      )}

      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-3" aria-live="polite" aria-relevant="additions">
        {hasMore && (
          <div className="mb-3 text-center">
            <Button variant="ghost" size="sm" onClick={() => void loadOlder()}>
              Carregar mensagens anteriores
            </Button>
          </div>
        )}
        <DataState loading={loading} error={null} empty={messages.length === 0} emptyTitle="Nenhuma mensagem ainda" emptyDescription="Seja o primeiro a escrever nesta sala.">
          <ol className="flex flex-col gap-3">
            {messages.map((m) => {
              const mine = m.author_id === me?.id;
              return (
                <li key={m.id} className={`group flex flex-col ${mine ? "items-end" : "items-start"}`}>
                  <span className="text-[11px] text-muted">
                    {mine ? "Você" : m.author_name} · {new Date(m.created_at).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" })}
                    {m.edited_at && !m.deleted && " · editada"}
                  </span>
                  <div className={`max-w-[80%] whitespace-pre-wrap break-words rounded-2xl px-3 py-2 text-sm ${m.deleted ? "italic text-muted" : mine ? "bg-primary text-primary-foreground" : "bg-surface-hover"}`}>
                    {m.deleted ? "mensagem removida" : m.body}
                  </div>
                  {!m.deleted && (mine || can("mercurio:manage")) && (
                    <span className="mt-0.5 flex gap-1 opacity-0 transition-opacity focus-within:opacity-100 group-hover:opacity-100">
                      {mine && (
                        <button
                          type="button"
                          className="rounded p-1 text-muted hover:text-foreground"
                          aria-label="Editar mensagem"
                          onClick={() => {
                            setEditing(m);
                            setDraft(m.body);
                          }}
                        >
                          <Pencil size={12} aria-hidden="true" />
                        </button>
                      )}
                      <button
                        type="button"
                        className="rounded p-1 text-muted hover:text-danger"
                        aria-label="Remover mensagem"
                        onClick={() => void run(() => apiClient.delete(`v1/mercurio/messages/${m.id}`))}
                      >
                        <Trash2 size={12} aria-hidden="true" />
                      </button>
                    </span>
                  )}
                </li>
              );
            })}
          </ol>
        </DataState>
        <div ref={bottom} />
      </div>

      <p className="h-5 px-4 text-xs text-muted" aria-live="polite">
        {typingNames.length > 0 && `${typingNames.join(", ")} ${typingNames.length > 1 ? "estão" : "está"} digitando…`}
      </p>
      <form onSubmit={submit} className="flex items-end gap-2 border-t border-surface-border p-3">
        <label htmlFor="mercurio-draft" className="sr-only">
          {editing ? "Editar mensagem" : "Mensagem"}
        </label>
        <textarea
          id="mercurio-draft"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={onKey}
          rows={2}
          maxLength={4000}
          placeholder={editing ? "Editando (Esc cancela)" : "Escreva uma mensagem (Enter envia, Shift+Enter quebra linha)"}
          className="min-h-11 flex-1 resize-none rounded-lg border border-surface-border bg-background px-3 py-2 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-primary"
        />
        <Button type="submit" aria-label={editing ? "Salvar edição" : "Enviar"} disabled={!draft.trim()}>
          <Send size={16} aria-hidden="true" />
        </Button>
      </form>
    </section>
  );
}

function Mercurio() {
  const router = useRouter();
  const { can } = useNexus();
  const [admin, setAdmin] = useState(false);
  const params = useSearchParams();
  const rooms = useApiQuery<ChatRoom[]>("v1/mercurio/rooms");
  const selectedId = params.get("sala") ?? rooms.data?.[0]?.id;
  const selected = rooms.data?.find((r) => r.id === selectedId);
  const { mutate } = rooms;
  const refresh = useCallback(() => void mutate(), [mutate]);

  // Notificação de não lida em outra sala (tópico user:<id>).
  useFrames((f) => {
    if (f.type === "mercurio.unread") refresh();
  });

  const groups: [string, ChatRoom[]][] = [
    ["Canais", (rooms.data ?? []).filter((r) => r.kind === "global")],
    ["Departamentos", (rooms.data ?? []).filter((r) => r.kind === "department")],
    ["Conversas diretas", (rooms.data ?? []).filter((r) => r.kind === "direct")],
  ];

  return (
    <div className="-mx-4 -mb-10 -mt-6 grid h-[calc(100dvh-var(--topbar-h))] grid-cols-1 sm:-mx-8 md:grid-cols-[16rem_1fr]">
      <nav aria-label="Salas do Mercúrio" className="hidden min-h-0 overflow-y-auto border-r border-surface-border p-3 md:block">
        <div className="mb-3 flex items-center justify-between px-2">
          <h1 className="text-lg font-semibold">Mercúrio</h1>
          {can("mercurio:manage") && (
            <Button variant="ghost" size="sm" aria-label="Gerenciar salas" onClick={() => setAdmin(true)}>
              <Settings2 size={16} aria-hidden="true" />
            </Button>
          )}
        </div>
        <DataState loading={rooms.isLoading} error={rooms.error} empty={(rooms.data ?? []).length === 0} emptyTitle="Nenhuma sala">
          {groups.map(([label, list]) =>
            list.length === 0 ? null : (
              <div key={label} className="mb-4">
                <p className="px-2 pb-1 text-[11px] font-semibold uppercase tracking-wider text-muted">{label}</p>
                <ul className="flex flex-col gap-0.5">
                  {list.map((r) => {
                    const Icon = KIND_ICON[r.kind];
                    const active = r.id === selectedId;
                    return (
                      <li key={r.id}>
                        <button
                          type="button"
                          onClick={() => router.replace(`/mercurio?sala=${r.id}`)}
                          aria-current={active ? "page" : undefined}
                          className={`flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm ${active ? "bg-primary/10 font-medium text-primary" : "hover:bg-surface-hover"}`}
                        >
                          <Icon size={14} aria-hidden="true" className="shrink-0" />
                          <span className="flex-1 truncate">{r.name}</span>
                          {r.unread > 0 && !active && (
                            <span className="rounded-full bg-primary px-1.5 text-[10px] font-bold text-primary-foreground">
                              {r.unread}
                              <span className="sr-only"> não lidas</span>
                            </span>
                          )}
                        </button>
                      </li>
                    );
                  })}
                </ul>
              </div>
            ),
          )}
        </DataState>
      </nav>
      <div className="flex min-h-0 flex-col">
        <div className="border-b border-surface-border p-2 md:hidden">
          <label htmlFor="mercurio-room" className="sr-only">
            Sala
          </label>
          <select
            id="mercurio-room"
            value={selectedId ?? ""}
            onChange={(e) => router.replace(`/mercurio?sala=${e.target.value}`)}
            className="h-10 w-full rounded-lg border border-surface-border bg-surface px-3 text-sm"
          >
            {(rooms.data ?? []).map((r) => (
              <option key={r.id} value={r.id}>
                {r.name}
                {r.unread ? ` (${r.unread})` : ""}
              </option>
            ))}
          </select>
        </div>
        {selected ? <RoomView key={selected.id} room={selected} onRead={refresh} /> : <p className="p-6 text-sm text-muted">Selecione uma sala.</p>}
      </div>
      <Dialog open={admin} onClose={() => setAdmin(false)} title="Salas do Mercúrio" size="xl">
        {admin && <RoomsAdmin rooms={rooms.data ?? []} onChanged={refresh} />}
      </Dialog>
    </div>
  );
}

export default function MercurioPage() {
  return (
    <Suspense fallback={null}>
      <Mercurio />
    </Suspense>
  );
}
