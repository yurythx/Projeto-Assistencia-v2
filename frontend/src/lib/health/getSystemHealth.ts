import "server-only";

import { BACKEND_INTERNAL_URL } from "@/lib/env";

// Formato server-side de "saúde da plataforma" — a mesma checagem que
// GET /api/health (rota HTTP, usada pelo cliente no painel de
// Monitoramento) expõe, mas chamável DIRETO por um Server Component que
// já roda no mesmo processo (ex.: o card "Estado do Core" do dashboard),
// sem o hop de rede redundante de fazer esse Server Component chamar sua
// PRÓPRIA rota HTTP por fetch. Extraído daqui pra route.ts consumir
// também, em vez de duplicar a mesma lógica de fetch+reshape nos dois
// lugares.
export interface SystemHealth {
  status: "ok" | "degraded" | "unhealthy";
  httpStatus: number;
  timestamp: string;
  services: Record<string, { status: string }>;
}

export async function getSystemHealth(): Promise<SystemHealth> {
  let backendResponse: Response;
  try {
    backendResponse = await fetch(`${BACKEND_INTERNAL_URL}/ready`, {
      cache: "no-store",
      signal: AbortSignal.timeout(5000),
    });
  } catch {
    return {
      status: "unhealthy",
      httpStatus: 503,
      timestamp: new Date().toISOString(),
      services: {},
    };
  }

  let checks: Record<string, string> = {};
  try {
    const body: { data: Record<string, string> | null } = await backendResponse.json();
    checks = body.data ?? {};
  } catch {
    // Resposta ilegível — segue com checks vazio; o status geral abaixo
    // ainda reflete o backendResponse.ok/status recebido.
  }

  const services = Object.fromEntries(
    Object.entries(checks).map(([name, status]) => [name, { status }]),
  );
  const allOk = Object.values(checks).every((status) => status === "ok");

  return {
    status: backendResponse.ok && allOk ? "ok" : "degraded",
    httpStatus: backendResponse.status,
    timestamp: new Date().toISOString(),
    services,
  };
}
