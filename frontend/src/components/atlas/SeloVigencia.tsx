import { BadgeCheck } from "lucide-react";

import type { OrgaoTTDD } from "@/lib/nexus/types";

import { fonteTTDD } from "./labels";

/** Selo de vigência: de qual publicação oficial vêm os prazos. */
export function SeloVigencia({ orgao }: { orgao?: OrgaoTTDD }) {
  if (!orgao) return null;
  return (
    <p className="inline-flex items-start gap-1.5 rounded-md border border-success/30 bg-success/10 px-2.5 py-1.5 text-xs text-foreground">
      <BadgeCheck size={14} aria-hidden="true" className="mt-0.5 shrink-0 text-success" />
      <span>
        <strong>Vigente</strong> — {fonteTTDD(orgao)}
      </span>
    </p>
  );
}
