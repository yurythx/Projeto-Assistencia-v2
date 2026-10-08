"use client";

import React, { createContext, useContext, useEffect, useSyncExternalStore } from "react";

import { DEFAULT_BRANDING, type SystemBrandingConfig } from "./brandingConfig";
import {
  getBrandingSnapshot,
  primeBrandingStore,
  resetBrandingStore,
  subscribeBranding,
  updateBrandingStore,
} from "./brandingStore";
import { safeResourceUrl } from "@/lib/security/safe-url";

// Re-export para não quebrar imports antigos (BrandingSettingsForm etc.).
export { DEFAULT_BRANDING };
export type { SystemBrandingConfig };

interface BrandingContextType {
  branding: SystemBrandingConfig;
  updateBranding: (newConfig: Partial<SystemBrandingConfig>) => void;
  resetBranding: () => void;
  toggleHighContrast: () => void;
  increaseFontSize: () => void;
  decreaseFontSize: () => void;
  resetFontSize: () => void;
}

const BrandingContext = createContext<BrandingContextType | undefined>(undefined);

export function BrandingProvider({
  children,
  initialBranding,
}: {
  children: React.ReactNode;
  /** Branding lido do cookie `nexus-branding` no layout do servidor. Vira o
   * server snapshot do useSyncExternalStore: como o cliente lê o MESMO
   * cookie, o snapshot de SSR e o do cliente coincidem e o nome não pisca
   * do valor antigo para o novo depois da hidratação. Ausente (fora do
   * layout, ex. testes) → DEFAULT_BRANDING. */
  initialBranding?: SystemBrandingConfig;
}) {
  // Estado vive fora do React (cookie) — useSyncExternalStore em vez de
  // useState + useEffect de hidratação. Ver brandingStore.ts.
  const serverSnapshot = initialBranding ?? DEFAULT_BRANDING;
  // Semeia o store com a identidade renderizada pelo servidor ANTES da
  // primeira leitura do cliente (snapshots iguais = sem flash).
  primeBrandingStore(serverSnapshot);
  const branding = useSyncExternalStore(
    subscribeBranding,
    getBrandingSnapshot,
    () => serverSnapshot,
  );

  // Sincroniza os atributos e-MAG (Alto Contraste + escala tipográfica) no
  // <html> — uso legítimo de useEffect (sincronizar com um sistema externo,
  // o DOM), sem chamar setState.
  useEffect(() => {
    if (typeof document === "undefined") return;
    const root = document.documentElement;
    if (branding.highContrast) {
      root.setAttribute("data-high-contrast", "true");
    } else {
      root.removeAttribute("data-high-contrast");
    }
    root.setAttribute("data-font-scale", String(branding.fontSizeScale || 100));
  }, [branding.highContrast, branding.fontSizeScale]);

  // Favicon white-label: branding.faviconUrl era um campo declarado desde
  // sempre (tipo + default + persistido no cookie), mas nada o lia — o
  // <link rel="icon"> ficava sempre no favicon padrão do Projeto Nexus
  // (achado de auditoria original). safeResourceUrl (S-03) aplica a mesma
  // allowlist de esquema que a logo já usa: só https:// vira href de
  // verdade.
  //
  // A 1ª versão deste efeito criava um <link rel="icon"> NOVO, adicional
  // aos que app/icon.svg + app/favicon.ico do Next já renderizam no
  // servidor (achado de uma 2ª rodada de verificação: com 3 <link
  // rel="icon"> concorrentes na <head> — o .ico, o .svg e este novo —,
  // qual vence é uma heurística de cada navegador, não algo garantido
  // por ordem no DOM; o Chrome, por exemplo, tende a preferir o ícone SVG
  // já existente independentemente de quando o outro foi inserido, então
  // a troca podia silenciosamente não aparecer nenhuma aba real mesmo com
  // o teste automatizado (que só checa o elemento, não a aba) passando.
  //
  // Correção: em vez de competir com um <link> a mais, troca o href dos
  // <link rel="icon">/<link rel="apple-touch-icon"> que JÁ EXISTEM — os
  // mesmos que o navegador já escolhe corretamente por padrão. Cada nó
  // guarda seu href original num data-attribute na primeira vez (uma
  // única leitura, nunca sobrescrita depois), para conseguir restaurar o
  // ícone padrão assim que a URL personalizada for removida/ficar
  // inválida — sem isso, depois de trocar uma vez não haveria como saber
  // qual era o valor original pra voltar.
  useEffect(() => {
    if (typeof document === "undefined") return;
    const safeFavicon = safeResourceUrl(branding.faviconUrl);
    const iconLinks = document.querySelectorAll<HTMLLinkElement>(
      'link[rel="icon"], link[rel="apple-touch-icon"]',
    );

    iconLinks.forEach((link) => {
      if (!link.dataset.nexusOriginalHref) {
        link.dataset.nexusOriginalHref = link.href;
      }
      link.href = safeFavicon || link.dataset.nexusOriginalHref;
    });
  }, [branding.faviconUrl]);

  const increaseFontSize = () => {
    const current = branding.fontSizeScale || 100;
    if (current < 130) updateBrandingStore({ fontSizeScale: current + 10 });
  };

  const decreaseFontSize = () => {
    const current = branding.fontSizeScale || 100;
    if (current > 90) updateBrandingStore({ fontSizeScale: current - 10 });
  };

  const resetFontSize = () => updateBrandingStore({ fontSizeScale: 100 });

  const value: BrandingContextType = {
    branding,
    updateBranding: updateBrandingStore,
    resetBranding: resetBrandingStore,
    toggleHighContrast: () => updateBrandingStore({ highContrast: !branding.highContrast }),
    increaseFontSize,
    decreaseFontSize,
    resetFontSize,
  };

  return <BrandingContext.Provider value={value}>{children}</BrandingContext.Provider>;
}

export function useBranding() {
  const context = useContext(BrandingContext);
  if (!context) {
    throw new Error("useBranding deve ser utilizado dentro de um BrandingProvider");
  }
  return context;
}
