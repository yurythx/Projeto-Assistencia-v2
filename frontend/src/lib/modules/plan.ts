import type { ModuleStatus } from "@/lib/nexus/types";

export interface ToggleStep {
  key: string;
  name: string;
  enabled: boolean;
}

/**
 * Planeja a ativação/desativação respeitando o grafo de dependências do
 * Kernel (o backend recusa com 409 qualquer ordem inválida):
 *  - desativar X desativa antes, de cima para baixo, todo módulo ativo que
 *    depende (direta ou transitivamente) de X;
 *  - ativar X ativa antes, de baixo para cima, toda dependência inativa.
 * Devolve os passos na ordem de execução (o último é o próprio X) ou um
 * erro quando o plano esbarra no núcleo (nunca desativável).
 */
export function planToggle(modules: ModuleStatus[], key: string, enabled: boolean): { steps: ToggleStep[]; error?: string } {
  const byKey = new Map(modules.map((m) => [m.key, m]));
  const target = byKey.get(key);
  if (!target) return { steps: [], error: `módulo ${key} desconhecido` };
  if (!enabled && target.core) return { steps: [], error: `${target.name} é do núcleo e não pode ser desativado` };

  const steps: ToggleStep[] = [];
  const seen = new Set<string>();

  const visit = (k: string) => {
    if (seen.has(k)) return;
    seen.add(k);
    const m = byKey.get(k);
    if (!m) return;
    if (enabled) {
      for (const dep of m.depends_on) visit(dep);
      if (!m.configured || k === key) steps.push({ key: k, name: m.name, enabled: true });
    } else {
      for (const d of m.dependents) visit(d);
      if (m.configured || k === key) steps.push({ key: k, name: m.name, enabled: false });
    }
  };
  visit(key);

  // Idempotência: nada a fazer se o alvo já está no estado pedido.
  if (target.configured === enabled && target.enabled === enabled) return { steps: [] };
  return { steps };
}

/** Nomes legíveis de uma lista de chaves. */
export function moduleNames(modules: ModuleStatus[], keys: string[]): string {
  const byKey = new Map(modules.map((m) => [m.key, m.name]));
  return keys.map((k) => byKey.get(k) ?? k).join(", ");
}
