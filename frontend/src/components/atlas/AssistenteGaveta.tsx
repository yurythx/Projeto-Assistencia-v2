"use client";

import { Bot, X } from "lucide-react";
import {
  createContext,
  useCallback,
  useContext,
  useId,
  useMemo,
  useState,
  type ReactNode,
} from "react";

import { ModalShell } from "@/components/ui/ModalShell";
import { useNexus } from "@/lib/nexus/NexusProvider";

import { AssistentePanel } from "./AssistentePanel";

interface AssistenteCtx {
  /** Quem tem atlas:read vê o assistente. */
  disponivel: boolean;
  /** Abre a gaveta; `pergunta` vem preenchida (contexto da página). */
  abrir: (pergunta?: string) => void;
}

const Ctx = createContext<AssistenteCtx>({ disponivel: false, abrir: () => {} });

export const useAssistente = () => useContext(Ctx);

/** Assistente do Atlas em gaveta lateral, disponível em todas as páginas do
 * módulo (botão flutuante + "Perguntar sobre…" de cada página). */
export function AssistenteProvider({ children }: { children: ReactNode }) {
  const { can } = useNexus();
  const disponivel = can("atlas:read");
  const [aberto, setAberto] = useState(false);
  const [pergunta, setPergunta] = useState("");
  // Nova conversa a cada abertura com contexto diferente.
  const [sessao, setSessao] = useState(0);
  const tituloId = useId();

  const abrir = useCallback((p = "") => {
    setPergunta(p);
    setSessao((s) => s + 1);
    setAberto(true);
  }, []);
  const fechar = () => setAberto(false);
  const valor = useMemo(() => ({ disponivel, abrir }), [disponivel, abrir]);

  return (
    <Ctx.Provider value={valor}>
      {children}
      {disponivel && (
        <>
          <button
            type="button"
            onClick={() => abrir()}
            className="fixed bottom-6 right-6 z-30 flex items-center gap-2 rounded-full bg-primary px-4 py-3 text-sm font-semibold text-primary-foreground shadow-lg transition hover:bg-primary-hover focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary print:hidden"
          >
            <Bot size={18} aria-hidden="true" /> Perguntar ao assistente
          </button>
          <ModalShell open={aberto} onClose={fechar} variant="drawer" labelledBy={tituloId}>
            <header className="flex shrink-0 items-center justify-between border-b border-surface-border px-4 py-3">
              <h2 id={tituloId} className="flex items-center gap-2 font-bold text-foreground">
                <Bot size={18} aria-hidden="true" className="text-primary" /> Assistente do Atlas
              </h2>
              <button
                type="button"
                onClick={fechar}
                aria-label="Fechar o assistente"
                className="rounded p-1 text-muted hover:bg-surface-hover hover:text-foreground"
              >
                <X size={18} aria-hidden="true" />
              </button>
            </header>
            {aberto && (
              <AssistentePanel key={sessao} perguntaInicial={pergunta} onNavegar={fechar} />
            )}
          </ModalShell>
        </>
      )}
    </Ctx.Provider>
  );
}
