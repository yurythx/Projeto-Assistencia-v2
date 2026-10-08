import "server-only";

import { DEFAULT_BRANDING } from "@/components/branding/brandingConfig";
import { publicGetOr } from "@/lib/api/publicServer";
import { BRANDING_TAG } from "@/lib/branding/tag";
import type { Branding } from "@/lib/nexus/types";

export interface ServerBranding {
  appName: string;
  appDescription: string;
  orgName: string;
  supportEmail: string;
  supportPhone: string;
}

/** Identidade white-label (GET /branding, cache de 60s) para Server
 * Components — nenhum texto institucional fica fixo no código. */
export async function getServerBranding(): Promise<ServerBranding> {
  const b = await publicGetOr<Branding | null>("branding", null, 60, [BRANDING_TAG]);
  return {
    appName: b?.app_name || DEFAULT_BRANDING.appName,
    appDescription: b?.app_description || "",
    orgName: b?.org_name || DEFAULT_BRANDING.orgName,
    supportEmail: b?.support_email || "",
    supportPhone: b?.support_phone || "",
  };
}
