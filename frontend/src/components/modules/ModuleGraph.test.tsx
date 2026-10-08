import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ModuleGraph } from "./ModuleGraph";
import type { ModuleStatus } from "@/lib/nexus/types";

function mod(key: string, name: string, extra: Partial<ModuleStatus> = {}): ModuleStatus {
  return {
    key, name, description: `Descrição ${name}`, core: false, default_enabled: true, depends_on: [], permissions: [],
    public: false, icon: "box", route: `/${key}`, enabled: true, configured: true, dependents: [], blocked_by: [], ...extra,
  };
}

const MODULES = [
  mod("iam", "IAM", { core: true, dependents: ["signum"] }),
  mod("signum", "Signum", { depends_on: ["iam"], dependents: ["tramite"] }),
  mod("tramite", "Trâmite", { depends_on: ["signum"] }),
  mod("blog", "Blog", { enabled: false, configured: false }),
  mod("calendario", "Agenda de salas e compromissos", { enabled: false, configured: true, blocked_by: ["blog"] }),
];

function setup(extra: Partial<Parameters<typeof ModuleGraph>[0]> = {}) {
  const onToggle = vi.fn();
  const utils = render(<ModuleGraph modules={MODULES} canManage busy={false} onToggle={onToggle} {...extra} />);
  const node = (name: RegExp) => screen.getByRole("button", { name });
  return { ...utils, onToggle, node };
}

describe("ModuleGraph", () => {
  it("desenha um nó acessível por módulo, com estado e ligações, e uma aresta por dependência", () => {
    const { container, node } = setup();
    expect(screen.getByRole("group", { name: /5 módulos, 2 dependências/ })).toBeInTheDocument();
    expect(node(/^Signum; Ativo; depende de IAM; necessário para Trâmite$/)).toBeInTheDocument();
    expect(node(/^IAM; Núcleo/)).toBeInTheDocument();
    expect(node(/^Blog; Inativo$/)).toBeInTheDocument();
    expect(node(/^Agenda de salas e compromissos; Aguardando dependência$/)).toHaveTextContent("Agenda de salas e c…");
    expect(container.querySelectorAll("[data-edge]")).toHaveLength(2);
    expect(screen.queryByRole("region")).not.toBeInTheDocument();
  });

  it("selecionar destaca a cadeia transitiva e esmaece o resto; clicar de novo limpa", async () => {
    const { container, node } = setup();
    await userEvent.click(node(/^Trâmite/));
    expect(node(/^Trâmite/)).toHaveAttribute("aria-pressed", "true");
    const opacity = (k: string) => container.querySelector(`[data-node="${k}"]`)!.getAttribute("class");
    expect(opacity("iam")).toContain("opacity-100"); // dependência transitiva
    expect(opacity("signum")).toContain("opacity-100");
    expect(opacity("blog")).toContain("opacity-30");
    expect(container.querySelector('[data-edge="iam->signum"]')!.getAttribute("class")).toContain("opacity-100");

    const panel = screen.getByRole("region", { name: "Detalhes de Trâmite" });
    expect(within(panel).getByText("Signum, IAM", { exact: false })).toBeInTheDocument();
    expect(within(panel).getByText("nenhum módulo")).toBeInTheDocument();

    await userEvent.click(node(/^Trâmite/));
    expect(screen.queryByRole("region")).not.toBeInTheDocument();
  });

  it("opera pelo teclado: Enter e espaço selecionam, Esc limpa, outras teclas não fazem nada", async () => {
    const { node } = setup();
    node(/^Signum/).focus();
    await userEvent.keyboard("{Enter}");
    expect(screen.getByRole("region", { name: "Detalhes de Signum" })).toBeInTheDocument();
    await userEvent.keyboard("a");
    expect(screen.getByRole("region", { name: "Detalhes de Signum" })).toBeInTheDocument();
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("region")).not.toBeInTheDocument();
    await userEvent.keyboard(" ");
    expect(screen.getByRole("region", { name: "Detalhes de Signum" })).toBeInTheDocument();
  });

  it("o painel liga e desliga pelo mesmo fluxo da tela; o núcleo não tem interruptor", async () => {
    const { node, onToggle } = setup();
    await userEvent.click(node(/^Blog/));
    await userEvent.click(screen.getByRole("switch", { name: "Ativar Blog" }));
    expect(onToggle).toHaveBeenCalledWith(MODULES[3], true);

    await userEvent.click(node(/^Signum/));
    await userEvent.click(screen.getByRole("switch", { name: "Desativar Signum" }));
    expect(onToggle).toHaveBeenLastCalledWith(MODULES[1], false);

    await userEvent.click(node(/^IAM/));
    const panel = screen.getByRole("region", { name: "Detalhes de IAM" });
    expect(within(panel).getByText("Sempre ativo")).toBeInTheDocument();
    expect(within(panel).queryByRole("switch")).not.toBeInTheDocument();
  });

  it("sem permissão (ou durante uma operação) o interruptor fica desabilitado", async () => {
    const { node } = setup({ canManage: false });
    await userEvent.click(node(/^Blog/));
    expect(screen.getByRole("switch", { name: "Ativar Blog" })).toBeDisabled();
  });
});
