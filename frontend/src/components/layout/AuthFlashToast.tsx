"use client";

import { useEffect } from "react";
import { usePathname, useSearchParams } from "next/navigation";

import { useToast } from "@/components/notifications/ToastProvider";

// Mensagens amigáveis de autenticação (achado de auditoria: logout e
// login terminavam em silêncio total — nenhuma confirmação visual de que
// a ação funcionou). Cada fluxo carimba um parâmetro de busca no
// redirecionamento final; este componente só existe pra ler esse
// parâmetro, mostrar o toast certo, e então LIMPAR a URL — sem isso, um
// F5 ou "voltar" do navegador repetiria o mesmo toast indefinidamente.
//
// A limpeza usa window.history.replaceState DIRETO, NUNCA router.replace()
// do next/navigation — achado da 2ª rodada (o toast simplesmente não
// aparecia, apesar do componente/efeito estarem corretos isoladamente,
// como os testes unitários já confirmavam): páginas como app/page.tsx são
// Server Components renderizados dinamicamente (await connection()), e
// router.replace() no App Router refaz o fetch RSC do segmento mesmo só
// trocando a query string — o que reconcilia toda a subárvore que inclui
// o próprio ToastProvider, descartando o toast que acabou de ser
// adicionado ao estado dele antes de a pessoa sequer conseguir vê-lo.
// history.replaceState só troca a URL na barra de endereço, sem passar
// pelo router do Next e sem re-render nenhum — a forma correta de "limpar
// um parâmetro de flash" nesta arquitetura.
//
// Montado uma vez em cada shell que pode ser o destino de um desses
// redirecionamentos: PublicShell (?logout=success, na home) e
// DashboardShell (?welcome=1, depois do login). Não renderiza nada.
export function AuthFlashToast() {
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const { showToast } = useToast();

  // searchParams.toString() como dependência (não o objeto em si, que é
  // uma instância nova a cada render) — dispara de novo só quando a
  // query string muda de verdade.
  const query = searchParams.toString();

  useEffect(() => {
    const params = new URLSearchParams(query);
    const logout = params.get("logout");
    const welcome = params.get("welcome");

    if (logout === "success") {
      showToast({
        title: "Você saiu com segurança",
        description: "Sua sessão foi encerrada.",
        tone: "success",
      });
    } else if (welcome === "1") {
      showToast({
        title: "Bem-vindo(a) de volta!",
        description: "Login realizado com sucesso.",
        tone: "success",
      });
    } else {
      return;
    }

    params.delete("logout");
    params.delete("welcome");
    const rest = params.toString();
    const newUrl = rest ? `${pathname}?${rest}` : pathname;
    window.history.replaceState(null, "", newUrl);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- só reage à query; pathname/showToast são estáveis o bastante aqui
  }, [query]);

  return null;
}
