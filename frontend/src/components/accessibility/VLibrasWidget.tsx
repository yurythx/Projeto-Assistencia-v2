"use client";

import { useEffect, type HTMLAttributes } from "react";
import Script from "next/script";

declare global {
  interface Window {
    VLibras?: {
      Widget: new (url: string) => void;
    };
    VLibrasWidget?: {
      open?: () => void;
    };
  }
}

// Estende os atributos HTML para aceitar os data-attributes do VLibras.
interface VLibrasDivProps extends HTMLAttributes<HTMLDivElement> {
  vw?: string;
  "vw-access-button"?: string;
  "vw-plugin-wrapper"?: string;
}

function initVLibras() {
  if (typeof window !== "undefined" && window.VLibras?.Widget) {
    try {
      new window.VLibras.Widget("https://vlibras.gov.br/app");
    } catch {
      // ignora se já montado ou inicializado
    }
  }
}

// Widget oficial VLibras (Governo Federal — https://vlibras.gov.br): tradução
// automática de todo o conteúdo do portal para a Língua Brasileira de Sinais.
// Montado uma vez, globalmente, em app/providers.tsx. O plugin é carregado do
// domínio oficial vlibras.gov.br — ver as liberações de CSP em src/proxy.ts.
export function VLibrasWidget() {
  useEffect(() => {
    initVLibras();
  }, []);

  return (
    <>
      <div {...({ vw: "true" } as VLibrasDivProps)} className="enabled">
        <div {...({ "vw-access-button": "true" } as VLibrasDivProps)} className="active" />
        <div {...({ "vw-plugin-wrapper": "true" } as VLibrasDivProps)}>
          <div className="vw-plugin-top-wrapper" />
        </div>
      </div>
      <Script
        src="https://vlibras.gov.br/app/vlibras-plugin.js"
        strategy="afterInteractive"
        onLoad={initVLibras}
      />
    </>
  );
}
