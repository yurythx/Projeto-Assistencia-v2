"use client";

import { useState } from "react";
import { KeyRound, Save, PlugZap, CheckCircle2, AlertTriangle, XCircle, Info } from "lucide-react";

import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { useToast } from "@/components/notifications/ToastProvider";
import { apiClient, ApiError } from "@/lib/api/client";
import type { KeycloakSettingsStatus, KeycloakTestResult } from "@/types/api";
import { keycloakSaveResponseSchema, keycloakTestResultSchema } from "@/lib/validation/api-schemas";

// Menu Configurações > Keycloak / IAM — cada campo tem uma descrição de
// como funciona e pra que serve (pedido explícito de auditoria/usuário),
// e um botão "Testar conexão" que valida os valores contra o Keycloak de
// verdade (discovery OIDC + client_credentials best-effort) ANTES de
// salvar. Salvar persiste no Postgres, cifrado (ver
// internal/platform/secretcrypto), e recarrega o verificador de tokens do
// backend em tempo real — sem reiniciar nenhum container (ver
// internal/platform/keycloakconfig).

interface FormState {
  issuerUrl: string;
  realm: string;
  clientId: string;
  clientSecret: string;
  audience: string;
  frontendClientId: string;
  frontendClientSecret: string;
}

function toFormState(s: KeycloakSettingsStatus): FormState {
  return {
    issuerUrl: s.issuer_url,
    realm: s.realm,
    clientId: s.client_id,
    clientSecret: "",
    audience: s.audience,
    frontendClientId: s.frontend_client_id,
    frontendClientSecret: "",
  };
}

const sourceBadge: Record<KeycloakSettingsStatus["source"], { label: string; tone: "success" | "info" | "neutral" }> = {
  database: { label: "Salvo no banco — em uso agora", tone: "success" },
  environment: { label: "Vindo de variável de ambiente", tone: "info" },
  unset: { label: "Nada configurado ainda", tone: "neutral" },
};

function TestResultPanel({ result }: { result: KeycloakTestResult }) {
  const icon =
    result.status === "ok" ? (
      <CheckCircle2 className="h-4 w-4 text-success" aria-hidden="true" />
    ) : result.status === "warning" ? (
      <AlertTriangle className="h-4 w-4 text-warning" aria-hidden="true" />
    ) : (
      <XCircle className="h-4 w-4 text-danger" aria-hidden="true" />
    );
  return (
    <div
      role="status"
      className="flex flex-col gap-2 rounded-lg border border-surface-border bg-surface-hover/30 p-3 text-xs"
    >
      <p className="flex items-center gap-2 font-semibold text-foreground">
        {icon}
        Discovery OIDC: {result.discovery_message}
      </p>
      {result.credentials_checked && (
        <p className="flex items-center gap-2 text-muted">
          {result.credentials_ok ? (
            <CheckCircle2 className="h-3.5 w-3.5 text-success" aria-hidden="true" />
          ) : (
            <AlertTriangle className="h-3.5 w-3.5 text-warning" aria-hidden="true" />
          )}
          {result.credentials_message}
        </p>
      )}
    </div>
  );
}

export function KeycloakSettingsForm({ initialStatus }: { initialStatus: KeycloakSettingsStatus }) {
  const { showToast } = useToast();
  const [status, setStatus] = useState(initialStatus);
  const [form, setForm] = useState<FormState>(() => toFormState(initialStatus));
  const [testResult, setTestResult] = useState<KeycloakTestResult | null>(null);
  const [testing, setTesting] = useState(false);
  const [saving, setSaving] = useState(false);

  const handleChange = (field: keyof FormState, value: string) => {
    setForm((prev) => ({ ...prev, [field]: value }));
    setTestResult(null);
  };

  async function handleTest() {
    setTesting(true);
    setTestResult(null);
    try {
      const { data } = await apiClient.post<KeycloakTestResult>("v1/admin/keycloak/test", {
        issuer_url: form.issuerUrl,
        client_id: form.clientId,
        client_secret: form.clientSecret,
        audience: form.audience,
      });
      const parsed = keycloakTestResultSchema.parse(data);
      setTestResult(parsed);
    } catch (err) {
      showToast({
        title: "Não foi possível testar a conexão",
        description: err instanceof ApiError ? err.message : "Erro inesperado",
        tone: "danger",
      });
    } finally {
      setTesting(false);
    }
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    try {
      const { data } = await apiClient.put<{ settings: KeycloakSettingsStatus; test: KeycloakTestResult }>(
        "v1/admin/keycloak",
        {
          issuer_url: form.issuerUrl,
          realm: form.realm,
          client_id: form.clientId,
          client_secret: form.clientSecret,
          audience: form.audience,
          frontend_client_id: form.frontendClientId,
          frontend_client_secret: form.frontendClientSecret,
        },
      );
      const parsed = keycloakSaveResponseSchema.parse(data);
      setStatus(parsed.settings);
      setForm(toFormState(parsed.settings));
      setTestResult(parsed.test);
      showToast({
        title: "Configuração do Keycloak salva",
        description: "O backend já recarregou a verificação de tokens com os novos valores — sem reiniciar.",
        tone: "success",
      });
    } catch (err) {
      showToast({
        title: "Não foi possível salvar",
        description: err instanceof ApiError ? err.message : "Erro inesperado",
        tone: "danger",
      });
    } finally {
      setSaving(false);
    }
  }

  const badge = sourceBadge[status.source];

  return (
    <Card className="border border-surface-border bg-surface">
      <CardHeader className="border-b border-surface-border">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <CardTitle as="h2" className="flex items-center gap-2 text-base font-bold text-foreground">
            <KeyRound className="h-5 w-5 text-primary" aria-hidden="true" />
            Integração com o Keycloak (IAM)
          </CardTitle>
          <Badge tone={badge.tone}>{badge.label}</Badge>
        </div>
        <p className="mt-1 text-xs text-muted">
          O Keycloak é o provedor de identidade (IAM) que emite e assina os tokens de acesso desta
          plataforma. Esta tela NUNCA cria ou administra o Keycloak em si — só aponta o backend para um
          realm/client já existentes lá. Salvar aqui grava a configuração no banco (o Client Secret fica
          cifrado) e aplica na hora, sem reiniciar nenhum container.
        </p>
        {status.updated_at && (
          <p className="mt-1 text-[11px] text-muted">
            Última alteração: {new Date(status.updated_at).toLocaleString("pt-BR")}
            {status.updated_by ? ` por ${status.updated_by}` : ""}
          </p>
        )}
      </CardHeader>

      <CardContent className="p-6">
        <form onSubmit={handleSave} className="flex flex-col gap-6 text-xs">
          {status.source === "environment" && (
            <div className="flex items-start gap-2 rounded-lg border border-accent/30 bg-accent/5 p-3 text-xs text-foreground">
              <Info className="mt-0.5 h-4 w-4 shrink-0 text-accent" aria-hidden="true" />
              <p>
                Esta configuração ainda roda a partir de variáveis de ambiente do container, nunca foi
                salva por esta tela. Você pode salvar sem preencher o Client Secret (o valor atual do
                ambiente será reaproveitado) — mas se quiser TROCAR o secret, precisa digitá-lo aqui.
              </p>
            </div>
          )}

          {/* Seção 1: Backend — verificação de token */}
          <div className="flex flex-col gap-4">
            <h3 className="border-b border-surface-border pb-1 text-xs font-bold uppercase tracking-wider text-primary">
              1. Backend — Verificação de Tokens (Resource Server)
            </h3>
            <p className="text-[11px] text-muted">
              Estes valores são usados pelo backend Go para BAIXAR as chaves públicas do realm (JWKS) e
              validar a assinatura, o emissor e a audiência de todo token recebido — o backend nunca
              autentica ninguém sozinho, só confere um token que o Keycloak já emitiu.
            </p>

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div className="flex flex-col gap-1 sm:col-span-2">
                <Input
                  label="Issuer URL *"
                  name="issuerUrl"
                  type="url"
                  autoComplete="off"
                  value={form.issuerUrl}
                  onChange={(e) => handleChange("issuerUrl", e.target.value)}
                  placeholder="https://sso.orgao.gov.br/realms/nexus"
                  required
                />
                <p className="text-[11px] text-muted">
                  URL do realm no Keycloak. É a partir dela que o backend descobre automaticamente todo o
                  resto (endpoint de token, chaves públicas) — o documento fica em{" "}
                  <code className="font-mono">{"{issuer_url}"}/.well-known/openid-configuration</code>.
                </p>
              </div>

              <div className="flex flex-col gap-1">
                <Input
                  label="Realm *"
                  name="realm"
                  autoComplete="off"
                  value={form.realm}
                  onChange={(e) => handleChange("realm", e.target.value)}
                  placeholder="nexus"
                  required
                />
                <p className="text-[11px] text-muted">
                  Nome do realm no Keycloak — só informativo/de conferência; a validação de fato usa a
                  Issuer URL acima.
                </p>
              </div>

              <div className="flex flex-col gap-1">
                <Input
                  label="Audiência (aud) *"
                  name="audience"
                  autoComplete="off"
                  value={form.audience}
                  onChange={(e) => handleChange("audience", e.target.value)}
                  placeholder="nexus-backend"
                  required
                />
                <p className="text-[11px] text-muted">
                  Todo token precisa ter esta audiência (claim <code className="font-mono">aud</code>)
                  para ser aceito — normalmente igual ao Client ID do backend, abaixo.
                </p>
              </div>

              <div className="flex flex-col gap-1">
                <Input
                  label="Client ID do Backend *"
                  name="clientId"
                  autoComplete="off"
                  value={form.clientId}
                  onChange={(e) => handleChange("clientId", e.target.value)}
                  placeholder="nexus-backend"
                  required
                />
                <p className="text-[11px] text-muted">
                  Client &ldquo;confidential&rdquo; do backend, cadastrado no Keycloak — identifica este
                  backend perante o realm.
                </p>
              </div>

              <div className="flex flex-col gap-1">
                <Input
                  label="Client Secret do Backend"
                  name="clientSecret"
                  type="password"
                  autoComplete="new-password"
                  value={form.clientSecret}
                  onChange={(e) => handleChange("clientSecret", e.target.value)}
                  placeholder={status.client_secret_set ? "•••••••••••••• (já definido — deixe em branco para manter)" : "Cole o Client Secret"}
                />
                <p className="text-[11px] text-muted">
                  Nunca é reexibido depois de salvo, por segurança — fica cifrado no banco. Deixe em
                  branco para manter o valor atual.
                </p>
              </div>
            </div>
          </div>

          {/* Seção 2: Frontend — login via navegador */}
          <div className="flex flex-col gap-4">
            <h3 className="border-b border-surface-border pb-1 text-xs font-bold uppercase tracking-wider text-primary">
              2. Frontend — Login via Navegador (NextAuth)
            </h3>
            <p className="text-[11px] text-muted">
              Client Keycloak DIFERENTE do backend, usado só pelo botão &ldquo;Entrar com
              Keycloak&rdquo; (fluxo Authorization Code + PKCE). Estes valores ficam registrados aqui
              para consulta e auditoria
              em um único lugar; como o processo do frontend lê suas próprias variáveis de ambiente uma
              única vez ao subir, uma troca aqui só entra em vigor no login depois que o container do
              frontend for reiniciado com as variáveis correspondentes atualizadas.
            </p>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div className="flex flex-col gap-1">
                <Input
                  label="Client ID do Frontend"
                  name="frontendClientId"
                  autoComplete="off"
                  value={form.frontendClientId}
                  onChange={(e) => handleChange("frontendClientId", e.target.value)}
                  placeholder="nexus-frontend"
                />
              </div>
              <div className="flex flex-col gap-1">
                <Input
                  label="Client Secret do Frontend"
                  name="frontendClientSecret"
                  type="password"
                  autoComplete="new-password"
                  value={form.frontendClientSecret}
                  onChange={(e) => handleChange("frontendClientSecret", e.target.value)}
                  placeholder={
                    status.frontend_client_secret_set
                      ? "•••••••••••••• (já definido — deixe em branco para manter)"
                      : "Cole o Client Secret"
                  }
                />
              </div>
            </div>
          </div>

          {testResult && <TestResultPanel result={testResult} />}

          <div className="flex flex-wrap items-center justify-between gap-3 border-t border-surface-border pt-4">
            <Button type="button" variant="secondary" onClick={handleTest} loading={testing} disabled={!form.issuerUrl}>
              <PlugZap className="mr-1.5 h-4 w-4" aria-hidden="true" />
              Testar conexão
            </Button>
            <Button type="submit" variant="primary" loading={saving}>
              <Save className="mr-1.5 h-4 w-4" aria-hidden="true" />
              Salvar alterações
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
