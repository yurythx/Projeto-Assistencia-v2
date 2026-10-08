import type { Metadata } from "next";
import localFont from "next/font/local";
import { cookies } from "next/headers";
import type { ReactNode } from "react";
import "./globals.css";

import { BRANDING_COOKIE, mergeServerBranding, parseBrandingCookie, tokensToCss } from "@/components/branding/brandingConfig";
import { publicGetOr } from "@/lib/api/publicServer";
import { BRANDING_TAG } from "@/lib/branding/tag";
import type { Branding } from "@/lib/nexus/types";
import { Providers } from "./providers";

// Fontes servidas localmente (woff2 de @fontsource, licença OFL em
// ./fonts): o build não depende de baixar do Google Fonts — o download
// falhava de forma intermitente no CI/Docker e em redes restritas, e
// evita também chamadas a terceiros na navegação (LGPD).
const inter = localFont({
  src: "./fonts/inter-latin-wght-normal.woff2",
  variable: "--font-inter",
  weight: "100 900",
  display: "swap",
});
const newsreader = localFont({
  src: [
    { path: "./fonts/newsreader-latin-500-normal.woff2", weight: "500", style: "normal" },
    { path: "./fonts/newsreader-latin-600-normal.woff2", weight: "600", style: "normal" },
  ],
  variable: "--font-newsreader",
  display: "swap",
});
const geistMono = localFont({
  src: "./fonts/geist-mono-latin-wght-normal.woff2",
  variable: "--font-geist-mono",
  weight: "100 900",
  display: "swap",
});

export async function generateMetadata(): Promise<Metadata> {
  const b = await publicGetOr<Branding | null>("branding", null, 60, [BRANDING_TAG]);
  const name = b?.app_name || "Projeto Nexus";
  return {
    title: { default: name, template: `%s — ${name}` },
    description: b?.app_description || "Plataforma corporativa modular (Microkernel) em conformidade com e-MAG, DSGov e LGPD.",
  };
}

export default async function RootLayout({ children }: { children: ReactNode }) {
  const cookieStore = await cookies();
  const theme = cookieStore.get("nexus-theme")?.value;
  const dataTheme = theme === "dark" || theme === "light" ? theme : undefined;

  // Identidade da organização (API, cache de 60s) + preferências e-MAG do
  // visitante (cookie) — aplicadas em <html> ANTES do 1º paint.
  const api = await publicGetOr<Branding | null>("branding", null, 60, [BRANDING_TAG]);
  const branding = mergeServerBranding(api, parseBrandingCookie(cookieStore.get(BRANDING_COOKIE)?.value));
  const tokenCss = tokensToCss(branding.tokens);

  return (
    <html
      lang="pt-BR"
      data-theme={dataTheme}
      data-high-contrast={branding.highContrast ? "true" : undefined}
      data-font-scale={String(branding.fontSizeScale || 100)}
      className={`${inter.variable} ${newsreader.variable} ${geistMono.variable} h-full antialiased`}
    >
      {/* Ternário, não `&&`: com tokenCss === "" o `&&` rendia um nó de texto
          vazio dentro de <head>, a hidratação falhava (React #418) e a
          navegação seguinte quebrava (removeChild em null) — o login não
          chegava ao dashboard numa instalação sem tokens de cor. */}
      <head>{tokenCss ? <style id="nexus-white-label">{tokenCss}</style> : null}</head>
      <body className="flex min-h-full flex-col">
        <Providers initialBranding={branding}>{children}</Providers>
      </body>
    </html>
  );
}
