import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { ErrorState } from "@/components/ui/ErrorState";
import { KeycloakSettingsForm } from "@/components/settings/KeycloakSettingsForm";
import { ApiError } from "@/lib/api/client";
import { serverApiGet } from "@/lib/api/server";
import { keycloakSettingsStatusSchema } from "@/lib/validation/api-schemas";
import type { KeycloakSettingsStatus } from "@/types/api";

export default async function KeycloakConfigPage() {
  let keycloakStatus: KeycloakSettingsStatus | null = null;
  let keycloakForbidden = false;
  let keycloakErrorMessage: string | null = null;

  try {
    const { data } = await serverApiGet<KeycloakSettingsStatus>(
      "v1/admin/keycloak",
      keycloakSettingsStatusSchema,
    );
    keycloakStatus = data;
  } catch (err) {
    if (err instanceof ApiError && err.status === 403) {
      keycloakForbidden = true;
    } else {
      keycloakErrorMessage = err instanceof ApiError ? err.message : "Falha ao carregar a configuração do Keycloak";
    }
  }

  return (
    <div className="flex flex-col gap-6">
      {keycloakForbidden && (
        <Card>
          <CardHeader>
            <CardTitle as="h2">Integração com o Keycloak (IAM)</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted">
              Restrito a administradores — sua conta não tem permissão para ver ou alterar a configuração
              do Keycloak.
            </p>
          </CardContent>
        </Card>
      )}

      {keycloakErrorMessage && <ErrorState message={keycloakErrorMessage} />}

      {keycloakStatus && <KeycloakSettingsForm initialStatus={keycloakStatus} />}
    </div>
  );
}
