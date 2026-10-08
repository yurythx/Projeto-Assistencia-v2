import { getServerSession } from "next-auth/next";
import { Cpu, Layers, Shield, Zap } from "lucide-react";

import { ModulesGrid } from "@/components/dashboard/ModulesGrid";
import { authOptions } from "@/lib/auth/options";
import { getSystemHealth } from "@/lib/health/getSystemHealth";

const statCellToneClass = {
  primary: "bg-primary/10 text-primary",
  success: "bg-success/10 text-success",
  warning: "bg-warning/10 text-warning",
  danger: "bg-danger/10 text-danger",
} as const;

function StatCell({
  label,
  value,
  hint,
  icon: Icon,
  tone = "primary",
}: {
  label: string;
  value: string;
  hint?: string;
  icon: typeof Shield;
  tone?: keyof typeof statCellToneClass;
}) {
  return (
    <div className="flex items-center gap-4 rounded-xl border border-surface-border bg-surface p-4 shadow-sm">
      <div className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-lg ${statCellToneClass[tone]}`}>
        <Icon size={20} aria-hidden="true" />
      </div>
      <div className="flex min-w-0 flex-col gap-0.5">
        <span className="text-xs font-semibold uppercase tracking-wider text-muted">{label}</span>
        <span className="text-lg font-bold text-foreground">{value}</span>
        {hint && <span className="truncate text-[11px] text-muted">{hint}</span>}
      </div>
    </div>
  );
}

export default async function DashboardOverviewPage() {
  const session = await getServerSession(authOptions);
  const firstName = (session?.user?.name ?? session?.user?.email ?? "").split(/[@\s]/)[0];
  const today = new Date().toLocaleDateString("pt-BR", { weekday: "long", day: "numeric", month: "long", year: "numeric" });

  // Checagem real (/ready do backend: postgres, rabbitmq, minio, redis).
  const health = await getSystemHealth();
  const degraded = Object.entries(health.services)
    .filter(([, s]) => s.status !== "ok")
    .map(([name]) => name);

  return (
    <div className="flex flex-col gap-8">
      <header className="flex flex-col gap-1 border-b border-surface-border pb-6">
        <p className="dateline">{today}</p>
        <h1 className="text-3xl font-bold tracking-tight">{firstName ? `Olá, ${firstName}` : "Visão geral"}</h1>
        <p className="text-sm text-muted">Módulos disponíveis para você e o estado da plataforma.</p>
      </header>

      <section aria-labelledby="indicadores" className="flex flex-col gap-3">
        <h2 id="indicadores" className="text-xs font-semibold uppercase tracking-widest text-muted">
          Plataforma
        </h2>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <StatCell label="Arquitetura" value="Microkernel" hint="Plug-ins ativados em runtime" icon={Layers} />
          <StatCell label="Identidade" value="Keycloak + RS256" hint="Federação AD · fallback local" icon={Shield} />
          <StatCell label="Mensageria" value="Outbox + RabbitMQ" hint="Entrega garantida com DLQ" icon={Zap} />
          <StatCell
            label="Estado do núcleo"
            value={health.status === "ok" ? "Saudável" : health.status === "degraded" ? "Degradado" : "Indisponível"}
            hint={health.status === "ok" ? "Dependências verificadas" : degraded.join(", ") || "Backend não respondeu"}
            tone={health.status === "ok" ? "success" : health.status === "degraded" ? "warning" : "danger"}
            icon={Cpu}
          />
        </div>
      </section>

      <section aria-labelledby="modulos" className="flex flex-col gap-4">
        <h2 id="modulos" className="text-lg font-semibold">
          Módulos
        </h2>
        <ModulesGrid />
      </section>
    </div>
  );
}
