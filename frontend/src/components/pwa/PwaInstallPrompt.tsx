"use client";

import { Download, X } from "lucide-react";
import { useEffect, useState } from "react";

import { useBranding } from "@/components/branding/BrandingContext";
import { Button } from "@/components/ui/Button";

const DISMISSED_KEY = "nexus-pwa-install-dismissed";

// BeforeInstallPromptEvent não faz parte do lib.dom.d.ts padrão do
// TypeScript (é uma extensão específica de navegadores baseados em
// Chromium) — tipado localmente com só o que este componente usa.
interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: "accepted" | "dismissed" }>;
}

function readDismissed(): boolean {
  try {
    return localStorage.getItem(DISMISSED_KEY) === "1";
  } catch {
    return false; // navegação privada ou storage bloqueado — nunca quebra o prompt
  }
}

// PwaInstallPrompt — App mobile / PWA (roadmap §6, Kernel): um banner
// discreto que aparece só quando o navegador sinaliza que o app É
// instalável (evento `beforeinstallprompt`, hoje só em Chromium —
// Firefox/Safari nunca disparam isto, então o banner nunca aparece lá,
// degradação esperada, não um bug). Lembrado por localStorage — quem
// dispensa uma vez não vê de novo neste navegador (mas volta a
// aparecer se o app for reinstalado ou o storage for limpo).
export function PwaInstallPrompt() {
  const { branding } = useBranding();
  const [deferredPrompt, setDeferredPrompt] = useState<BeforeInstallPromptEvent | null>(null);
  // Lazy initializer (não um efeito): readDismissed() é síncrono e seguro
  // de chamar durante o render (nunca lança, ver o try/catch dentro) —
  // não muda o resultado ler o storage aqui vs. depois de montar, e evita
  // o re-render em cascata de um setState dentro de useEffect. Nunca
  // aparece antes de deferredPrompt existir de qualquer forma (ver o
  // guard de render no fim), então não há "flash" a evitar.
  const [dismissed, setDismissed] = useState(() => readDismissed());

  useEffect(() => {
    function onBeforeInstallPrompt(event: Event) {
      event.preventDefault();
      setDeferredPrompt(event as BeforeInstallPromptEvent);
    }
    function onInstalled() {
      setDeferredPrompt(null);
    }
    window.addEventListener("beforeinstallprompt", onBeforeInstallPrompt);
    window.addEventListener("appinstalled", onInstalled);
    return () => {
      window.removeEventListener("beforeinstallprompt", onBeforeInstallPrompt);
      window.removeEventListener("appinstalled", onInstalled);
    };
  }, []);

  function dismiss() {
    setDismissed(true);
    try {
      localStorage.setItem(DISMISSED_KEY, "1");
    } catch {
      /* sem storage disponível — só não persiste entre sessões, sem quebrar nada */
    }
  }

  async function install() {
    if (!deferredPrompt) return;
    await deferredPrompt.prompt();
    await deferredPrompt.userChoice;
    setDeferredPrompt(null);
  }

  if (!deferredPrompt || dismissed) return null;

  return (
    <div
      role="status"
      className="fixed bottom-4 left-4 right-4 z-50 flex items-center gap-3 rounded-xl border border-surface-border bg-surface p-3 shadow-lg sm:left-auto sm:right-4 sm:w-80"
    >
      <Download size={20} aria-hidden="true" className="shrink-0 text-primary" />
      <div className="min-w-0 flex-1">
        <p className="text-sm font-medium text-foreground">Instalar o {branding.appName}</p>
        <p className="text-xs text-muted">Acesse direto da tela inicial, como um aplicativo.</p>
      </div>
      <Button size="sm" onClick={() => void install()}>
        Instalar
      </Button>
      <button
        type="button"
        onClick={dismiss}
        aria-label="Dispensar"
        className="shrink-0 rounded-md p-1 text-muted hover:bg-black/5 dark:hover:bg-white/5"
      >
        <X size={16} aria-hidden="true" />
      </button>
    </div>
  );
}
