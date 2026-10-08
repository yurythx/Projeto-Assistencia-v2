import { getServerSession } from "next-auth/next";
import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import type { ReactNode } from "react";

import { DashboardShell } from "@/components/layout/DashboardShell";
import { SWRProvider } from "@/lib/api/SWRProvider";
import { authOptions } from "@/lib/auth/options";

// Grupo de rotas (protected) — sem segmento próprio na URL — compartilhado
// por /dashboard e /configuracao (§ Reestruturação de rotas): as duas
// seções são autenticadas do mesmo jeito e usam o mesmo shell visual, e um
// grupo de rotas é exatamente o mecanismo do Next.js pra isso — evita
// duplicar esta checagem de sessão em dois layout.tsx separados.
//
// Defesa em profundidade: o proxy.ts já redireciona visitantes não
// autenticados para fora de /dashboard/** e /configuracao/**, mas uma
// checagem também aqui garante que nenhuma rota deste grupo é renderizada
// sem uma sessão válida, mesmo que o matcher do proxy seja malconfigurado
// no futuro.
export default async function ProtectedLayout({ children }: { children: ReactNode }) {
  const session = await getServerSession(authOptions);
  if (!session || session.error) {
    redirect("/login");
  }

  const userLabel = session.user?.email ?? session.user?.name ?? "Autenticado";

  // Mesmo cookie "nexus-theme" que ThemeToggle.tsx escreve — lido aqui (não
  // só no layout raiz) porque ThemeToggle só existe dentro da área
  // autenticada; ver o comentário em ThemeToggle.tsx sobre por que isto
  // evita o flash de tema errado sem precisar de um <script> inline
  // (bloqueado pela CSP com nonce — src/proxy.ts).
  const cookieStore = await cookies();
  const themeCookie = cookieStore.get("nexus-theme")?.value;
  const initialTheme = themeCookie === "dark" || themeCookie === "light" ? themeCookie : undefined;

  // Mesmas preferências de dispositivo lidas server-side pelo mesmo motivo
  // do tema: entregar o shell já no estado certo no 1º paint, sem o flash
  // de "padrão → valor salvo" na hidratação. Escritas por
  // lib/layout/sidebarCollapsedStore.ts e components/branding/brandingStore.ts.
  const initialCollapsed = cookieStore.get("nexus-sidebar-collapsed")?.value === "true";

  // O branding (nome, Alto Contraste, escala de fonte) agora é provido na
  // raiz — app/providers.tsx — para valer também nas páginas públicas.

  return (
    <SWRProvider>
      <DashboardShell
        userLabel={userLabel}
        initialTheme={initialTheme}
        initialCollapsed={initialCollapsed}
      >
        {children}
      </DashboardShell>
    </SWRProvider>
  );
}
