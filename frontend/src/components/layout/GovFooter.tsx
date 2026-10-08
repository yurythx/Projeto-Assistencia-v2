"use client";

import { Clock, Mail, Phone } from "lucide-react";
import Link from "next/link";

import { useBranding } from "@/components/branding/BrandingContext";
import { Logo } from "@/components/ui/Logo";

/** Rodapé institucional DSGov (alvo do atalho Alt+4: #gov-footer). */
export function GovFooter() {
  const { branding } = useBranding();
  const year = new Date().getFullYear();

  return (
    <footer id="gov-footer" tabIndex={-1} className="gov-footer mt-auto bg-footer-bg text-footer-foreground outline-none">
      <div className="mx-auto grid w-full max-w-7xl gap-8 px-4 py-10 text-sm sm:px-8 md:grid-cols-3">
        <div className="flex flex-col gap-3">
          <div className="flex items-center gap-2 text-base font-bold">
            <Logo size={22} />
            <span>{branding.appName}</span>
          </div>
          {branding.appDescription && <p className="opacity-80">{branding.appDescription}</p>}
          <p className="text-xs font-semibold uppercase tracking-wider opacity-90">{branding.orgName}</p>
        </div>

        <div className="flex flex-col gap-2">
          <h2 className="text-xs font-bold uppercase tracking-wider">Atendimento</h2>
          <ul className="flex flex-col gap-2 opacity-90">
            {branding.supportEmail && (
              <li className="flex items-center gap-2">
                <Mail size={16} aria-hidden="true" />
                <a href={`mailto:${branding.supportEmail}`} className="underline-offset-2 hover:underline">
                  {branding.supportEmail}
                </a>
              </li>
            )}
            {branding.supportPhone && (
              <li className="flex items-center gap-2">
                <Phone size={16} aria-hidden="true" />
                <span>{branding.supportPhone}</span>
              </li>
            )}
            {branding.supportHours && (
              <li className="flex items-center gap-2">
                <Clock size={16} aria-hidden="true" />
                <span>{branding.supportHours}</span>
              </li>
            )}
            <li>
              <Link href="/contato" className="underline-offset-2 hover:underline">
                Fale conosco
              </Link>
            </li>
          </ul>
        </div>

        <nav aria-label="Institucional" className="flex flex-col gap-2">
          <h2 className="text-xs font-bold uppercase tracking-wider">Transparência e conformidade</h2>
          <ul className="flex flex-col gap-1.5 opacity-90">
            <li>
              <Link href="/privacidade" className="underline-offset-2 hover:underline">
                Privacidade e proteção de dados (LGPD)
              </Link>
            </li>
            <li>
              <Link href="/acessibilidade" className="underline-offset-2 hover:underline">
                Acessibilidade (e-MAG / WCAG 2.1 AA)
              </Link>
            </li>
            <li>
              <Link href="/transparencia" className="underline-offset-2 hover:underline">
                Transparência ativa (LAI)
              </Link>
            </li>
            <li>
              <Link href="/verificar" className="underline-offset-2 hover:underline">
                Verificar assinatura eletrônica
              </Link>
            </li>
            <li>
              <Link href="/sobre" className="underline-offset-2 hover:underline">
                Sobre a plataforma
              </Link>
            </li>
          </ul>
        </nav>
      </div>
      <div className="border-t border-white/15">
        <div className="mx-auto flex w-full max-w-7xl flex-col items-center justify-between gap-2 px-4 py-4 text-xs opacity-80 sm:flex-row sm:px-8">
          <p>
            © {year} {branding.orgName}
          </p>
          <p>Plataforma construída sobre o Projeto Nexus · Padrão Digital de Governo (DSGov)</p>
        </div>
      </div>
    </footer>
  );
}
