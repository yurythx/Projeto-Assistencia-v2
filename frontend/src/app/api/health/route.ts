import { NextResponse } from "next/server";

import { getSystemHealth } from "@/lib/health/getSystemHealth";

// GET /api/health — ponte server-to-server pro /ready do backend Go.
//
// Rota PRÓPRIA, fora do proxy BFF genérico (app/api/backend/[...path]/
// route.ts), de propósito: aquele proxy sempre monta o alvo como
// /api/{path} (rotas de negócio versionadas, ex. /api/v1/integrations) e
// sempre injeta um Authorization: Bearer — nenhuma das duas coisas se
// aplica aqui. /ready não vive sob /api/v1 (é o mesmo endpoint que o
// HEALTHCHECK do Docker Compose chama direto, sem token nenhum, e
// continua público de propósito: um orquestrador de containers nunca tem
// uma sessão de usuário pra apresentar). Tentar buscá-lo através do proxy
// genérico só resultava em 404 (bug real: GET /api/backend/health, que o
// proxy nunca conseguiria mapear pra nada que exista no backend).
//
// A lógica de fetch+reshape vive em lib/health/getSystemHealth.ts —
// reaproveitada também pelo card "Estado do Core" do dashboard (Server
// Component, chama a função direto, sem precisar bater nesta rota HTTP).
export async function GET() {
  const health = await getSystemHealth();
  return NextResponse.json(
    {
      data: { status: health.status, timestamp: health.timestamp, services: health.services },
      error:
        health.status === "unhealthy"
          ? { code: "DEPENDENCY_UNAVAILABLE", message: "A API não respondeu ao /ready." }
          : null,
    },
    { status: health.httpStatus },
  );
}
