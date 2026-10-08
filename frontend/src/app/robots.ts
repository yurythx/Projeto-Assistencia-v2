import type { MetadataRoute } from "next";

import { APP_URL } from "@/lib/env";

// Só o site institucional é público; a área autenticada (app/(protected))
// fica fora do rastreio. A verificação de assinatura tem noindex na página.
export default function robots(): MetadataRoute.Robots {
  return {
    rules: {
      userAgent: "*",
      allow: ["/", "/sobre", "/servicos", "/setores", "/eventos", "/contato", "/transparencia", "/acessibilidade", "/privacidade"],
      disallow: [
        "/login", "/dashboard", "/configuracao", "/monitoramento", "/auditoria", "/perfil", "/busca", "/mercurio", "/blog",
        "/wiki", "/agenda", "/diretorio", "/arquivos", "/signum", "/tramite", "/atlas", "/gestao", "/exemplos", "/verificar/",
      ],
    },
    sitemap: `${APP_URL}/sitemap.xml`,
  };
}
