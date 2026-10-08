import { ShieldCheck } from "lucide-react";
import { connection } from "next/server";
import { cookies } from "next/headers";
import Link from "next/link";
import { Suspense, type CSSProperties } from "react";

import { LoginCard } from "@/components/auth/LoginCard";
import { getServerBranding } from "@/lib/branding/server";
import { AccessibilityBar } from "@/components/layout/AccessibilityBar";
import { Logo } from "@/components/ui/Logo";
import { Seal } from "@/components/ui/Seal";
import { ThemeToggle } from "@/components/ui/ThemeToggle";

// O painel de marca descreve exatamente o que a plataforma faz para um
// fiscal de contrato — sem linguagem de vendedor ("enterprise", "a
// tecnologia mais avançada do mercado"), que a versão anterior tinha
// herdado de um template.
const capabilities = [
  "Autenticação unificada via SSO Keycloak e credenciais locais seguras.",
  "Mensageria resiliente com Transactional Outbox e integração com RabbitMQ.",
  "Painel de telemetria e monitoramento de microsserviços em tempo real.",
  "Arquitetura limpa e modular pronta para expansão de novos módulos.",
];

export default async function LoginPage() {
  await connection();
  const brand = await getServerBranding();

  const cookieStore = await cookies();
  const themeCookie = cookieStore.get("nexus-theme")?.value;
  const initialTheme = themeCookie === "dark" || themeCookie === "light" ? themeCookie : undefined;

  return (
    <div className="flex min-h-screen flex-col pt-10">
      <header className="fixed inset-x-0 top-0 z-50">
        <AccessibilityBar showSearchShortcut={false} />
      </header>

      <main id="main-content" tabIndex={-1} className="flex min-h-0 flex-1 outline-none">
      <div
        className="relative hidden overflow-hidden bg-brand-panel text-white lg:flex lg:w-[44%] lg:flex-col lg:justify-between lg:p-12"
        style={{ "--seal": "#d1524a" } as CSSProperties}
      >
        <Seal
          size={520}
          decorative
          className="pointer-events-none absolute -bottom-40 -right-40 text-white/[0.06]"
        />

        <div className="relative flex items-center gap-2.5 text-lg font-semibold">
          <Logo size={30} />
          {brand.appName}
        </div>

        <div className="relative flex flex-col gap-6">
          <p className="dateline text-white/60">{brand.orgName}</p>
          <h1 className="max-w-md text-3xl font-semibold leading-tight text-white">
            Acesso à plataforma institucional.
          </h1>
          <p className="max-w-sm text-sm text-white/75">
            {brand.appDescription || "Entre com sua conta corporativa (SSO) ou credencial local."}
          </p>
          <ul className="mt-1 flex flex-col gap-3 text-sm text-white/90">
            {capabilities.map((item) => (
              <li key={item} className="flex gap-3">
                <span aria-hidden="true" className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-seal" />
                {item}
              </li>
            ))}
          </ul>
        </div>

        <p className="relative flex items-center gap-2 text-xs text-white/55">
          <ShieldCheck size={14} aria-hidden="true" />
          Entrada por senha ou SSO corporativo · auditoria de toda ação sensível
        </p>
      </div>

      {/* Painel do formulário */}
      <div className="relative flex flex-1 flex-col items-center justify-center p-6">
        <div className="absolute right-4 top-4">
          <ThemeToggle initialTheme={initialTheme} />
        </div>

        <Link href="/" className="mb-10 flex items-center gap-2 text-lg font-semibold lg:hidden">
          <Logo size={30} />
          {brand.appName}
        </Link>

        <Suspense fallback={null}>
          <LoginCard />
        </Suspense>

        <Link href="/" className="mt-10 text-xs text-muted hover:text-foreground">
          ← Voltar para a página inicial
        </Link>
      </div>
      </main>

      <footer id="gov-footer" tabIndex={-1} className="sr-only">
        Rodapé da página de login
      </footer>
    </div>
  );
}
