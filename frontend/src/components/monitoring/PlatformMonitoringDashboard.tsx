"use client";

import { useState } from "react";
import useSWR from "swr";
import {
  Activity,
  CheckCircle2,
  AlertTriangle,
  RefreshCw,
  Server,
  Database,
  Radio,
  HardDrive,
  Zap,
  Clock,
  Layers,
  Download
} from "lucide-react";

import { Button } from "@/components/ui/Button";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/Card";
import { StatusIndicator } from "@/components/ui/StatusIndicator";
import { useConnectionState } from "@/components/layout/ConnectionStateContext";
import { CONNECTION_LABEL, CONNECTION_TONE } from "@/lib/websocket/connectionCopy";
import { apiClient } from "@/lib/api/client";
import { useNexus } from "@/lib/nexus/NexusProvider";

import { OutboxRequeueButton } from "./OutboxRequeueButton";
import type { IntegrationStatus } from "@/types/api";

// GET /api/v1/monitoring/outbox-stats (ver docs/openapi.yaml) — essa,
// diferente de /api/health, É uma chamada de negócio normal através do
// proxy BFF de sempre (exige o bearer que apiClient já injeta).
interface OutboxStatsResponse {
  pending: number;
  published: number;
  failed: number;
}

interface SystemHealthResponse {
  status: "ok" | "degraded" | "unhealthy";
  timestamp: string;
  services: Record<string, { status: string }>;
  /** Round-trip medido no próprio navegador (performance.now()), não um
   * valor do backend — a "Latência Média" do KPI abaixo era um número
   * fixo no código-fonte (1.8 ms sempre, não importa o que acontecesse);
   * isto é o mais próximo de uma medição real que dá pra ter sem uma
   * instrumentação de latência por serviço no backend (não existe hoje). */
  latencyMs?: number;
}

// /api/health é uma rota PRÓPRIA do frontend (app/api/health/route.ts),
// não uma chamada de apiClient/proxy BFF — ela busca o /ready real do
// backend Go (que não vive sob /api/v1 e nunca exigiu autenticação, o
// mesmo endpoint que o HEALTHCHECK do Docker chama) e reformata pro
// formato que este painel espera. Chamar apiClient.get("health") aqui
// dava sempre 404: o proxy genérico só sabe montar /api/v1/... (achado de
// auditoria — console cheio de "GET /api/backend/health 404").
const fetcher = async (url: string): Promise<SystemHealthResponse> => {
  const start = performance.now();
  const res = await fetch(url);
  const latencyMs = Math.round(performance.now() - start);
  const json: { data: SystemHealthResponse } = await res.json();
  return { ...json.data, latencyMs };
};

export function PlatformMonitoringDashboard() {
  const [lastCheckTime, setLastCheckTime] = useState<string>(new Date().toLocaleTimeString("pt-BR"));
  // Estado REAL da conexão WebSocket (DashboardShell → ConnectionStateProvider
  // → aqui) — o card "Conexão WebSocket" mostrava "Ativa" fixo no
  // código-fonte antes disto, mesmo desconectado (achado de auditoria).
  const connectionState = useConnectionState();

  const { data: health, error, mutate, isValidating } = useSWR<SystemHealthResponse>(
    "/api/health",
    fetcher,
    {
      refreshInterval: 10000, // auto-refresh a cada 10s
      revalidateOnFocus: true,
      // "Verificado às" (mostrado em cada card de serviço) precisa
      // acompanhar TODA revalidação bem-sucedida, não só o clique manual
      // em "Atualizar" — senão o carimbo de hora fica parado enquanto o
      // auto-refresh de 10s continua atualizando os dados por trás.
      onSuccess: () => setLastCheckTime(new Date().toLocaleTimeString("pt-BR")),
    }
  );

  const { can } = useNexus();
  const { data: outboxStats, mutate: refreshOutbox } = useSWR<OutboxStatsResponse>(
    "v1/monitoring/outbox-stats",
    (path: string) => apiClient.get<OutboxStatsResponse>(path).then((res) => res.data),
    { refreshInterval: 10000, revalidateOnFocus: true },
  );

  const handleRefresh = () => {
    mutate();
  };

  const isHealthy = !error && health?.status !== "unhealthy";

  // Status DE VERDADE, derivado do /ready do backend (via /api/health) —
  // antes deste conserto, os quatro cartões abaixo mostravam "online" fixo
  // no código-fonte, sempre, mesmo que o serviço estivesse fora do ar
  // (achado de auditoria). backendCheckStatus() cobre só o que o backend
  // de fato verifica em /ready (postgres, rabbitmq); o próprio backend-api
  // é considerado "online" implicitamente sempre que ESTA chamada teve
  // resposta (se ele estivesse fora do ar, error estaria setado); MinIO
  // não tem checagem própria em /ready hoje — reportado como "unknown" em
  // vez de inventar um "online", até existir uma checagem real pra ele.
  function backendCheckStatus(name: string): IntegrationStatus {
    if (error) return "offline";
    if (!health) return "unknown";
    const check = health.services?.[name];
    if (!check) return "unknown";
    return check.status === "ok" ? "online" : "offline";
  }

  const infrastructureServices: Array<{
    id: string;
    name: string;
    description: string;
    port: string;
    icon: typeof Server;
    status: IntegrationStatus;
    type: string;
  }> = [
    {
      id: "backend-api",
      name: "API REST Core & Auth",
      description: "Servidor backend Go em arquitetura limpa com JWT local e rotas /api/v1",
      port: "8002",
      icon: Server,
      status: error ? "offline" : health ? "online" : "unknown",
      type: "Core Microservice",
    },
    {
      id: "postgres-db",
      name: "PostgreSQL 16 Engine",
      description: "Banco de dados relacional principal com suporte a transações ACID e Outbox",
      port: "5433",
      icon: Database,
      status: backendCheckStatus("postgres"),
      type: "Relational DB",
    },
    {
      id: "rabbitmq-broker",
      name: "RabbitMQ AMQP Broker",
      description: "Fila de mensagens orientada a eventos para desacoplamento de workers",
      port: "5673 / 15673",
      icon: Radio,
      status: backendCheckStatus("rabbitmq"),
      type: "Message Broker",
    },
    {
      id: "minio-storage",
      name: "MinIO Object Storage (S3)",
      description: "Armazenamento de arquivos e anexos compatível com Amazon S3 API.",
      port: "9002 / 9003",
      icon: HardDrive,
      status: backendCheckStatus("minio"),
      type: "S3 Storage",
    },
  ];

  const onlineCount = infrastructureServices.filter((s) => s.status === "online").length;

  return (
    <div className="flex flex-col gap-8 pb-10">
      {/* Header Principal */}
      <header className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between border-b border-surface-border pb-6">
        <div className="flex flex-col gap-1">
          <div className="flex items-center gap-2">
            <span className="dateline">Telemetria & Infraestrutura</span>
            <span
              className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold ${
                isHealthy
                  ? "bg-success/10 text-success"
                  : "bg-warning/10 text-warning"
              }`}
            >
              {isHealthy ? <CheckCircle2 size={12} /> : <AlertTriangle size={12} />}
              {isHealthy ? "SISTEMA OPERACIONAL" : "ATENÇÃO"}
            </span>
          </div>
          <h1 className="text-3xl font-bold tracking-tight">Monitoramento da Plataforma</h1>
          <p className="text-sm text-muted">
            Status dos microsserviços, barramento de eventos Outbox e saúdes dos componentes em tempo real.
          </p>
        </div>

        <div className="flex items-center gap-3">
          <span className="text-xs text-muted">Atualizado às {lastCheckTime}</span>
          <Button
            size="sm"
            variant="secondary"
            onClick={handleRefresh}
            disabled={isValidating}
            className="gap-2"
          >
            <RefreshCw size={14} className={isValidating ? "animate-spin" : ""} />
            Atualizar
          </Button>
          <Button
            size="sm"
            variant="primary"
            onClick={() => window.open("/api/backend/v1/audit/export", "_blank")}
            className="gap-2"
          >
            <Download size={14} />
            Exportar LAI (CSV)
          </Button>
        </div>
      </header>

      {/* KPI Cards de Performance */}
      <section className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Card className="bg-surface/50">
          <CardContent className="pt-4 flex items-center justify-between">
            <div className="flex flex-col gap-1">
              <span className="text-xs font-semibold uppercase tracking-wider text-muted">Status Geral</span>
              <span className="text-xl font-bold text-foreground">
                {Math.round((onlineCount / infrastructureServices.length) * 100)}% Online
              </span>
              <span className={`text-[11px] ${isHealthy ? "text-success" : "text-warning"}`}>
                {onlineCount} de {infrastructureServices.length} serviços verificados como ativos
              </span>
            </div>
            <div
              className={`flex h-10 w-10 items-center justify-center rounded-lg ${
                isHealthy ? "bg-success/10 text-success" : "bg-warning/10 text-warning"
              }`}
            >
              <Activity size={20} />
            </div>
          </CardContent>
        </Card>

        <Card className="bg-surface/50">
          <CardContent className="pt-4 flex items-center justify-between">
            <div className="flex flex-col gap-1">
              <span className="text-xs font-semibold uppercase tracking-wider text-muted">Outbox Event Queue</span>
              <span className="text-xl font-bold text-foreground">
                {outboxStats ? `${outboxStats.pending} Pendentes` : "—"}
              </span>
              <span className={`text-[11px] ${outboxStats && outboxStats.failed > 0 ? "text-danger" : "text-muted"}`}>
                {!outboxStats
                  ? "GET /api/v1/monitoring/outbox-stats"
                  : outboxStats.failed > 0
                    ? `${outboxStats.failed} com falha (Dead Letter)`
                    : "EventBus processado sem atraso"}
              </span>
              {outboxStats && outboxStats.failed > 0 && can("monitoring:manage") && (
                <OutboxRequeueButton failed={outboxStats.failed} onDone={() => void refreshOutbox()} />
              )}
            </div>
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <Zap size={20} />
            </div>
          </CardContent>
        </Card>

        <Card className="bg-surface/50">
          <CardContent className="pt-4 flex items-center justify-between">
            <div className="flex flex-col gap-1">
              <span className="text-xs font-semibold uppercase tracking-wider text-muted">Latência do Healthcheck</span>
              <span className="text-xl font-bold text-foreground">
                {health?.latencyMs !== undefined ? `${health.latencyMs} ms` : "—"}
              </span>
              <span className="text-[11px] text-muted">Round-trip de GET /api/health (medido agora)</span>
            </div>
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-accent/10 text-accent">
              <Clock size={20} />
            </div>
          </CardContent>
        </Card>

        <Card className="bg-surface/50">
          <CardContent className="pt-4 flex items-center justify-between">
            <div className="flex flex-col gap-1">
              <span className="text-xs font-semibold uppercase tracking-wider text-muted">Conexão WebSocket</span>
              <span className={`text-xl font-bold ${CONNECTION_TONE[connectionState].textClass}`}>
                {CONNECTION_LABEL[connectionState]}
              </span>
              <span className="text-[11px] text-muted">Notificações em tempo real</span>
            </div>
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-purple-500/10 text-purple-500">
              <Radio size={20} />
            </div>
          </CardContent>
        </Card>
      </section>

      {/* Grid de Serviços do Nexus */}
      <section className="flex flex-col gap-4">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-lg font-semibold">Infraestrutura & Dependências</h2>
            <p className="text-xs text-muted">Componentes essenciais que sustentam a plataforma.</p>
          </div>
        </div>

        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {infrastructureServices.map((service) => {
            const Icon = service.icon;

            return (
              <Card key={service.id} className="relative overflow-hidden transition-all hover:border-primary/50">
                <CardHeader className="pb-3">
                  <div className="flex items-start justify-between">
                    <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
                      <Icon size={20} />
                    </div>
                    <StatusIndicator status={service.status} />
                  </div>
                  <CardTitle className="text-base font-bold pt-2">{service.name}</CardTitle>
                  <CardDescription className="text-xs text-muted leading-relaxed">
                    {service.description}
                  </CardDescription>
                </CardHeader>
                <CardContent className="pt-0 flex flex-col gap-3 border-t border-surface-border/50 mt-2 py-3">
                  <div className="flex items-center justify-between text-xs font-mono">
                    <span className="text-muted">Porta Host:</span>
                    <span className="font-semibold text-foreground">{service.port}</span>
                  </div>
                  <div className="flex items-center justify-between pt-1">
                    <span className="rounded bg-surface-border/60 px-2 py-0.5 text-[10px] font-medium text-muted">
                      {service.type}
                    </span>
                    <span className="text-[11px] text-muted">Verificado às {lastCheckTime}</span>
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>
      </section>

      {/* Painel do Transactional Outbox Pattern */}
      <section className="flex flex-col gap-4">
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-warning/10 text-warning">
                  <Layers size={20} />
                </div>
                <div>
                  <CardTitle className="text-base">Barramento Transactional Outbox</CardTitle>
                  <CardDescription className="text-xs">
                    Garantia de entrega de eventos de negócio (Exactly-Once Semantics)
                  </CardDescription>
                </div>
              </div>
              {/* Badge derivado de outboxStats de verdade — achado de
                  revisão: dizia "Outbox Worker Ativo" fixo no código-fonte,
                  a mesma classe de problema já corrigida nos KPIs acima
                  (nenhuma leitura real de que o worker está processando,
                  só uma contagem de linhas com falha registrada). */}
              <span
                className={`rounded-full px-3 py-1 text-xs font-semibold ${
                  !outboxStats
                    ? "bg-black/5 text-muted dark:bg-white/10"
                    : outboxStats.failed > 0
                      ? "bg-danger/10 text-danger"
                      : "bg-success/10 text-success"
                }`}
              >
                {!outboxStats ? "Sem dados" : outboxStats.failed > 0 ? "Falhas na fila" : "Outbox Ativo"}
              </span>
            </div>
          </CardHeader>
          <CardContent className="flex flex-col gap-4 text-xs">
            <div className="grid gap-4 sm:grid-cols-3 rounded-lg border border-surface-border p-4 bg-surface/30">
              <div className="flex flex-col gap-0.5">
                <span className="text-muted">Estratégia de Persistência:</span>
                <span className="font-semibold text-foreground">PostgreSQL `outbox_events`</span>
              </div>
              <div className="flex flex-col gap-0.5">
                <span className="text-muted">Transporte Assíncrono:</span>
                <span className="font-semibold text-foreground">RabbitMQ Exchange (`events.direct`)</span>
              </div>
              <div className="flex flex-col gap-0.5">
                <span className="text-muted">Politica de Re-tentativas:</span>
                <span className="font-semibold text-foreground">Exponential Backoff + Dead Letter Queue</span>
              </div>
            </div>

            <p className="text-muted text-xs leading-relaxed">
              O padrão Transactional Outbox grava as mutações de dados e a emissão de eventos em uma única transação SQL.
              O worker de fundo varre os eventos não publicados e distribui ao RabbitMQ, garantindo resiliência total a falhas de rede.
            </p>
          </CardContent>
        </Card>
      </section>
    </div>
  );
}
