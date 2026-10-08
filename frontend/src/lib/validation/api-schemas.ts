import { z } from "zod";

// Schemas Zod das RESPOSTAS REST consumidas pelas telas — a contraparte do
// que lib/validation/schemas.ts já faz para os eventos de WebSocket (S-07
// da auditoria: "validar schema nos dois lados"). Passados opcionalmente a
// serverApiGet/apiClient; quando presentes, a resposta é validada em
// runtime antes de chegar à UI, em vez de um `as T` cego.
//
// `.passthrough()` de propósito: um campo NOVO no backend Go não deve
// quebrar o front — só a AUSÊNCIA/tipo errado de um campo que a tela usa.

export const keycloakSettingsStatusSchema = z
  .object({
    source: z.enum(["database", "environment", "unset"]),
    issuer_url: z.string(),
    realm: z.string(),
    client_id: z.string(),
    client_secret_set: z.boolean(),
    audience: z.string(),
    frontend_client_id: z.string(),
    frontend_client_secret_set: z.boolean(),
    updated_at: z.string().optional(),
    updated_by: z.string().optional(),
  })
  .passthrough();

export const keycloakTestResultSchema = z
  .object({
    status: z.enum(["ok", "warning", "failed"]),
    discovery_ok: z.boolean(),
    discovery_message: z.string(),
    credentials_checked: z.boolean(),
    credentials_ok: z.boolean(),
    credentials_message: z.string(),
  })
  .passthrough();

export const keycloakSaveResponseSchema = z
  .object({
    settings: keycloakSettingsStatusSchema,
    test: keycloakTestResultSchema,
  })
  .passthrough();
