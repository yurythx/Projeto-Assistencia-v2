import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

let searchParams = new URLSearchParams();
vi.mock("next/navigation", () => ({
  usePathname: () => "/",
  useSearchParams: () => searchParams,
}));

import { AuthFlashToast } from "./AuthFlashToast";
import { ToastProvider } from "@/components/notifications/ToastProvider";

// Espiona a API do navegador de verdade (não um mock de router) —
// achado de auditoria: a 1ª versão deste teste mockava useRouter() e
// afirmava que router.replace() tinha sido chamado, o que passava mesmo
// na versão do componente que tinha o bug real (router.replace() no App
// Router refaz o fetch RSC da página, descartando o toast recém-criado
// ANTES do usuário conseguir vê-lo — algo que um mock de useRouter nunca
// conseguiria capturar, só um navegador de verdade). Corrigido para usar
// window.history.replaceState, e este teste passou a espionar a API
// real em vez de mockar o router.
function renderWithParams(query: string) {
  searchParams = new URLSearchParams(query);
  return render(
    <ToastProvider>
      <AuthFlashToast />
    </ToastProvider>,
  );
}

describe("AuthFlashToast", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("logout=success mostra o toast de saída e limpa o parâmetro da URL", () => {
    const replaceState = vi.spyOn(window.history, "replaceState");
    renderWithParams("logout=success");

    expect(screen.getByText("Você saiu com segurança")).toBeInTheDocument();
    expect(replaceState).toHaveBeenCalledWith(null, "", "/");
  });

  it("welcome=1 mostra o toast de boas-vindas e limpa o parâmetro da URL", () => {
    const replaceState = vi.spyOn(window.history, "replaceState");
    renderWithParams("welcome=1");

    expect(screen.getByText("Bem-vindo(a) de volta!")).toBeInTheDocument();
    expect(replaceState).toHaveBeenCalledWith(null, "", "/");
  });

  it("preserva outros parâmetros da URL ao limpar só o próprio", () => {
    const replaceState = vi.spyOn(window.history, "replaceState");
    renderWithParams("logout=success&foo=bar");

    expect(replaceState).toHaveBeenCalledWith(null, "", "/?foo=bar");
  });

  it("sem parâmetro reconhecido, não mostra toast nem mexe na URL", () => {
    const replaceState = vi.spyOn(window.history, "replaceState");
    renderWithParams("");

    expect(screen.queryByText("Você saiu com segurança")).not.toBeInTheDocument();
    expect(screen.queryByText("Bem-vindo(a) de volta!")).not.toBeInTheDocument();
    expect(replaceState).not.toHaveBeenCalled();
  });
});
