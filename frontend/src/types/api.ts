// Tipos compartilhados do núcleo que não pertencem a um plug-in (os
// contratos dos módulos vivem em lib/nexus/types.ts, espelho dos DTOs Go).

export type IntegrationStatus = "unknown" | "online" | "offline" | "degraded" | "disabled";

// GET/PUT /api/v1/admin/keycloak (restrito a nexus-admin) — ver docs/openapi.yaml.
// "source" indica de onde vieram os valores: "database" (já salvo pelo
// menu Configurações > Keycloak — o que está em uso agora), "environment"
// (nunca foi salvo por esta tela; vem de variável de ambiente do
// processo) ou "unset". Nunca inclui um client secret em texto plano.
export interface KeycloakSettingsStatus {
  source: "database" | "environment" | "unset";
  issuer_url: string;
  realm: string;
  client_id: string;
  client_secret_set: boolean;
  audience: string;
  frontend_client_id: string;
  frontend_client_secret_set: boolean;
  updated_at?: string;
  updated_by?: string;
}

// POST /api/v1/admin/keycloak/test — resultado de um teste de conexão,
// nunca persiste nada.
export interface KeycloakTestResult {
  status: "ok" | "warning" | "failed";
  discovery_ok: boolean;
  discovery_message: string;
  credentials_checked: boolean;
  credentials_ok: boolean;
  credentials_message: string;
}
