import type { ReactNode } from "react";

import { AssistenteProvider } from "@/components/atlas/AssistenteGaveta";

/** Todas as páginas do Atlas compartilham o assistente em gaveta lateral. */
export default function AtlasLayout({ children }: { children: ReactNode }) {
  return <AssistenteProvider>{children}</AssistenteProvider>;
}
