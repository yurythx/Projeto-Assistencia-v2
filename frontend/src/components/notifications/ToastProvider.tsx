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

import { createPortal } from "react-dom";
import { Toast, type ToastData, type ToastTone } from "@/components/ui/Toast";

interface ToastContextValue {
  showToast: (toast: { title: string; description?: string; tone?: ToastTone }) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

// Toda notificação some sozinha depois de 6s, além de poder ser fechada
// manualmente pelo botão "X" do Toast (ver dismiss abaixo). A-16: o timer
// pausa enquanto o mouse/foco estiver sobre o toast (ver pause/resume) —
// WCAG 2.2.1 Timing Adjustable exige que conteúdo temporizado possa ser
// pausado/estendido, não só dispensado manualmente.
const AUTO_DISMISS_MS = 6000;

// Provedor da pilha de toasts, montado uma vez no DashboardShell. Guarda
// a lista de toasts ativos em memória (não em Zustand/Redux — o estado é
// puramente local a esta árvore de componentes) e expõe showToast() via
// contexto para qualquer componente descendente disparar uma notificação
// visual, seja a partir de uma ação do usuário (IntegrationCard) ou de um
// evento de WebSocket (NotificationCenter).
export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastData[]>([]);
  const idRef = useRef(0);
  // P-05: guarda os timers de auto-dismiss para poder cancelá-los — sem
  // isto um `setTimeout` órfão chamava setState após o unmount do provider.
  const timersRef = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());
  // A-16: quanto tempo falta (em ms) para cada toast se dispensar sozinho,
  // e quando o segmento de contagem atual começou — junto dão o "tempo
  // restante" certo depois de uma pausa, em vez de reiniciar do zero ou
  // usar sempre AUTO_DISMISS_MS de novo.
  const remainingRef = useRef<Map<string, number>>(new Map());
  const startedAtRef = useRef<Map<string, number>>(new Map());

  const clearTimer = useCallback((id: string) => {
    const timer = timersRef.current.get(id);
    if (timer) {
      clearTimeout(timer);
      timersRef.current.delete(id);
    }
  }, []);

  const dismiss = useCallback(
    (id: string) => {
      clearTimer(id);
      remainingRef.current.delete(id);
      startedAtRef.current.delete(id);
      setToasts((current) => current.filter((t) => t.id !== id));
    },
    [clearTimer],
  );

  const scheduleDismiss = useCallback(
    (id: string, ms: number) => {
      startedAtRef.current.set(id, Date.now());
      timersRef.current.set(
        id,
        setTimeout(() => dismiss(id), ms),
      );
    },
    [dismiss],
  );

  // A-16: pausa o auto-dismiss (hover ou foco de teclado no toast) —
  // guarda quanto tempo realmente restava, para o resume retomar dali em
  // vez de do início.
  const pause = useCallback(
    (id: string) => {
      const startedAt = startedAtRef.current.get(id);
      const remaining = remainingRef.current.get(id) ?? AUTO_DISMISS_MS;
      if (startedAt !== undefined) {
        const elapsed = Date.now() - startedAt;
        remainingRef.current.set(id, Math.max(0, remaining - elapsed));
      }
      clearTimer(id);
    },
    [clearTimer],
  );

  // A-16: retoma a contagem pelo tempo que faltava. Se já não sobrava
  // tempo nenhum (pausou bem no fim), dispensa na hora — não trava o
  // toast na tela pra sempre.
  const resume = useCallback(
    (id: string) => {
      const remaining = remainingRef.current.get(id) ?? AUTO_DISMISS_MS;
      if (remaining <= 0) {
        dismiss(id);
        return;
      }
      scheduleDismiss(id, remaining);
    },
    [dismiss, scheduleDismiss],
  );

  const showToast = useCallback<ToastContextValue["showToast"]>(
    ({ title, description, tone = "info" }) => {
      idRef.current += 1;
      const id = `toast-${idRef.current}`;
      setToasts((current) => [...current, { id, title, description, tone }]);
      remainingRef.current.set(id, AUTO_DISMISS_MS);
      scheduleDismiss(id, AUTO_DISMISS_MS);
    },
    [scheduleDismiss],
  );

  // Monitora se existe algum <dialog open> nativo na tela. Se houver,
  // faz portal do container de toasts para dentro do <dialog>, garantindo que
  // os toasts fiquem no top-layer do navegador e nunca fiquem atrás do modal.
  const [activeDialog, setActiveDialog] = useState<HTMLDialogElement | null>(null);

  useEffect(() => {
    const updateActiveDialog = () => {
      const dialogs = Array.from(document.querySelectorAll("dialog"));
      const openDialog = dialogs.find((d) => d.open) || null;
      setActiveDialog(openDialog);
    };

    updateActiveDialog();
    // childList também: um <dialog> pode sair da tela desmontado (sem passar
    // por open=false). Sem isso, o portal ficaria preso num nó fora do DOM e
    // o toast — p.ex. o erro de uma exclusão recusada logo após o modal de
    // confirmação — sumiria.
    const observer = new MutationObserver(updateActiveDialog);
    observer.observe(document.body, { attributes: true, subtree: true, attributeFilter: ["open"], childList: true });
    return () => observer.disconnect();
  }, []);

  // Limpa todos os timers pendentes no unmount.
  useEffect(() => {
    const timers = timersRef.current;
    return () => {
      timers.forEach(clearTimeout);
      timers.clear();
    };
  }, []);

  const toastContainer = (
    <div
      aria-live="polite"
      role="status"
      className="pointer-events-none fixed bottom-4 right-4 z-[9999] flex flex-col gap-2"
    >
      {toasts.map((toast) => (
        <div key={toast.id} className="pointer-events-auto">
          <Toast toast={toast} onDismiss={dismiss} onPause={pause} onResume={resume} />
        </div>
      ))}
    </div>
  );

  return (
    <ToastContext.Provider value={{ showToast }}>
      {children}
      {activeDialog ? createPortal(toastContainer, activeDialog) : toastContainer}
    </ToastContext.Provider>
  );
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) {
    throw new Error("useToast must be used within a ToastProvider");
  }
  return ctx;
}
