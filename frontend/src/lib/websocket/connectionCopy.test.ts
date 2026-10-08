import { describe, expect, it } from "vitest";

import { CONNECTION_LABEL, CONNECTION_TONE } from "./connectionCopy";
import type { ConnectionState } from "./client";

const ALL_STATES: ConnectionState[] = ["idle", "connecting", "open", "closed", "unauthorized"];

describe("connectionCopy", () => {
  // Fonte única usada por Topbar e PlatformMonitoringDashboard — as duas
  // telas mostram o MESMO estado; garantir que nenhum ConnectionState
  // fica sem rótulo/cor evita a divergência que motivou esta extração
  // ("Ao vivo" vs "Ativa" pro mesmo estado `open`).
  it("tem rótulo e cor pra todo ConnectionState", () => {
    for (const state of ALL_STATES) {
      expect(CONNECTION_LABEL[state]).toBeTruthy();
      expect(CONNECTION_TONE[state].dotClass).toBeTruthy();
      expect(CONNECTION_TONE[state].textClass).toBeTruthy();
    }
  });
});
