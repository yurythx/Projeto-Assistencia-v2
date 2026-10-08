"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";

import type { ToastTone } from "@/components/ui/Toast";
import { apiClient } from "@/lib/api/client";

export interface NotificationHistoryItem {
  id: string;
  title: string;
  description?: string;
  tone: ToastTone;
  createdAt: number;
  read: boolean;
  /** Caminho interno para onde o aviso leva (notificação persistida). */
  link?: string;
  /** Id da notificação gravada no servidor (ADR 027); sem ele, o item é só
   * da sessão (toasts de difusão geral). */
  serverId?: string;
}

/** Notificação gravada no servidor (GET v1/notificacoes, frame notificacao.nova). */
export interface NotificacaoServidor {
  id: string;
  modulo: string;
  titulo: string;
  mensagem: string;
  link: string;
  lida: boolean;
  created_at: string;
}

// Quantas notificações a bandeja do sino guarda — a caixa completa fica em
// /notificacoes (persistida no servidor).
const MAX_ITEMS = 20;

interface NotificationHistoryContextValue {
  items: NotificationHistoryItem[];
  unreadCount: number;
  push: (item: { title: string; description?: string; tone?: ToastTone }) => void;
  /** Aviso persistido que chegou em tempo real (sem duplicar o que já está). */
  pushServer: (n: NotificacaoServidor) => void;
  markAllRead: () => void;
}

const NotificationHistoryContext = createContext<NotificationHistoryContextValue | null>(null);

const doServidor = (n: NotificacaoServidor): NotificationHistoryItem => ({
  id: `srv-${n.id}`,
  serverId: n.id,
  title: n.titulo,
  description: n.mensagem || undefined,
  link: n.link || undefined,
  tone: "info",
  createdAt: new Date(n.created_at).getTime(),
  read: n.lida,
});

// Provedor da bandeja do sino, montado uma vez no DashboardShell ao lado de
// ToastProvider. Carrega a caixa persistida do servidor (ADR 027) e recebe,
// pelo NotificationCenter, os avisos novos e os toasts de difusão geral;
// marcar como lido vale também no servidor.
export function NotificationHistoryProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<NotificationHistoryItem[]>([]);
  const idRef = useRef(0);

  const juntar = useCallback((novos: NotificationHistoryItem[]) => {
    setItems((atuais) => {
      const vistos = new Set(atuais.map((i) => i.serverId).filter(Boolean));
      const frescos = novos.filter((n) => !n.serverId || !vistos.has(n.serverId));
      return [...frescos, ...atuais].sort((a, b) => b.createdAt - a.createdAt).slice(0, MAX_ITEMS);
    });
  }, []);

  useEffect(() => {
    let ativo = true;
    apiClient
      .get<{ itens: NotificacaoServidor[]; nao_lidas: number }>(
        `v1/notificacoes?limit=${MAX_ITEMS}`,
      )
      .then((res) => {
        if (ativo) juntar((res.data?.itens ?? []).map(doServidor));
      })
      .catch(() => {
        // sem caixa (API fora ou sessão sem usuário local): só a sessão
      });
    return () => {
      ativo = false;
    };
  }, [juntar]);

  const push = useCallback<NotificationHistoryContextValue["push"]>(
    ({ title, description, tone = "info" }) => {
      idRef.current += 1;
      juntar([
        {
          id: `notif-${idRef.current}`,
          title,
          description,
          tone,
          createdAt: Date.now(),
          read: false,
        },
      ]);
    },
    [juntar],
  );

  const pushServer = useCallback((n: NotificacaoServidor) => juntar([doServidor(n)]), [juntar]);

  const markAllRead = useCallback(() => {
    if (items.some((i) => i.serverId && !i.read)) {
      void apiClient.post("v1/notificacoes/lidas").catch(() => {});
    }
    setItems((current) => current.map((item) => ({ ...item, read: true })));
  }, [items]);

  const unreadCount = items.filter((item) => !item.read).length;

  return (
    <NotificationHistoryContext.Provider
      value={{ items, unreadCount, push, pushServer, markAllRead }}
    >
      {children}
    </NotificationHistoryContext.Provider>
  );
}

export function useNotificationHistory(): NotificationHistoryContextValue {
  const ctx = useContext(NotificationHistoryContext);
  if (!ctx) {
    throw new Error("useNotificationHistory must be used within a NotificationHistoryProvider");
  }
  return ctx;
}
