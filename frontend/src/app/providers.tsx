"use client";

import { SessionProvider } from "next-auth/react";
import type { ReactNode } from "react";

import { VLibrasWidget } from "@/components/accessibility/VLibrasWidget";
import { BrandingProvider } from "@/components/branding/BrandingContext";
import type { SystemBrandingConfig } from "@/components/branding/brandingConfig";
import { AccessibilityShortcuts } from "@/components/layout/AccessibilityShortcuts";

// Provedores globais (site público + área autenticada):
// - SessionProvider (NextAuth);
// - BrandingProvider (white-label + Alto Contraste e escala de fonte e-MAG);
// - AccessibilityShortcuts (Alt+1..4 em qualquer página);
// - VLibrasWidget (widget oficial de Libras em toda página).
export function Providers({ children, initialBranding }: { children: ReactNode; initialBranding?: SystemBrandingConfig }) {
  return (
    <SessionProvider>
      <BrandingProvider initialBranding={initialBranding}>
        <AccessibilityShortcuts />
        {children}
        <VLibrasWidget />
      </BrandingProvider>
    </SessionProvider>
  );
}
