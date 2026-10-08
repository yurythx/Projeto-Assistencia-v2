import { Accessibility, Eye, Type, ExternalLink, Keyboard, FileText } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { connection } from "next/server";

import { PublicShell } from "@/components/layout/PublicShell";
import { getServerBranding } from "@/lib/branding/server";

export const metadata: Metadata = {
  title: "Acessibilidade",
  description:
    "Declaração e instruções de acessibilidade e-MAG da plataforma.",
};

// § Padronização de páginas públicas: esta página tinha seu próprio
// layout (breadcrumb manual que nenhuma outra página pública usa,
// cabeçalho em caixa com ícone, largura/espaçamento diferentes) — achado
// de revisão. Agora segue exatamente o mesmo molde de app/page.tsx e
// app/sobre/page.tsx: async + `await connection()` (obrigatório pra
// renderização dinâmica, sem isso o nonce de CSP gerado em proxy.ts nunca
// bate com o embutido no HTML estático — ver a mesma nota nessas duas
// páginas), o wrapper `mx-auto flex w-full max-w-5xl flex-1 flex-col gap-*
// px-6 py-12`, e a mesma seção de abertura (selo/eyebrow + h1 + parágrafo
// de introdução, sem ícone em caixa nem breadcrumb).
export default async function AccessibilityPage() {
  await connection();
  const brand = await getServerBranding();

  return (
    <PublicShell>
      <div className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-10 px-6 py-12">
        <section className="flex flex-col gap-4">
          <div className="flex items-center gap-2 text-xs font-bold font-mono text-primary uppercase tracking-wider">
            <Accessibility size={16} className="text-success" />
            e-MAG 2.0 / WCAG 2.1 AA
          </div>
          <h1 className="text-3xl font-bold text-foreground">Acessibilidade</h1>
          <p className="text-muted leading-relaxed">
            {brand.appName} ({brand.orgName}) — em conformidade com as normas{" "}
            <strong>e-MAG (Modelo de Acessibilidade do Governo Federal)</strong>.
          </p>
        </section>

        {/* Conteúdo Principal de Acessibilidade */}
        <div className="grid gap-6 md:grid-cols-2">
          {/* Card 1: Atalhos de Teclado (e-MAG) */}
          <section className="flex flex-col gap-4 rounded-xl border border-surface-border bg-surface p-6 shadow-sm">
            <div className="flex items-center gap-2 text-primary font-semibold text-lg border-b border-surface-border pb-3">
              <Keyboard size={20} />
              <h2>Atalhos de Teclado (e-MAG)</h2>
            </div>
            <p className="text-xs text-muted leading-relaxed">
              Os padrões de atalhos válidos em qualquer página do site e sistema são:
            </p>
            <ul className="flex flex-col gap-2.5 text-xs text-foreground font-mono">
              <li className="flex items-center gap-2 rounded-lg bg-surface-hover p-2 border border-surface-border">
                <span className="rounded bg-primary px-2 py-0.5 text-white font-bold">Alt + 1</span>
                <span>Início do conteúdo principal da página (`#main-content`)</span>
              </li>
              <li className="flex items-center gap-2 rounded-lg bg-surface-hover p-2 border border-surface-border">
                <span className="rounded bg-primary px-2 py-0.5 text-white font-bold">Alt + 2</span>
                <span>Início do menu principal (`#main-menu`)</span>
              </li>
              <li className="flex items-center gap-2 rounded-lg bg-surface-hover p-2 border border-surface-border">
                <span className="rounded bg-primary px-2 py-0.5 text-white font-bold">Alt + 3</span>
                <span>Busca interna (`#global-search`)</span>
              </li>
              <li className="flex items-center gap-2 rounded-lg bg-surface-hover p-2 border border-surface-border">
                <span className="rounded bg-primary px-2 py-0.5 text-white font-bold">Alt + 4</span>
                <span>Início do rodapé (`#gov-footer`)</span>
              </li>
            </ul>

            <div className="mt-2 rounded-lg border border-warning/30 bg-warning/10 p-3 text-[11px] text-foreground">
              <strong>Particularidades por Navegador:</strong>
              <ul className="mt-1 list-disc list-inside flex flex-col gap-1 opacity-90 font-sans">
                <li>
                  No Firefox (Windows/Linux): use <code>Alt + Shift + Número</code>
                </li>
                <li>
                  No Firefox (macOS): use <code>Ctrl + Alt + Número</code>
                </li>
                <li>
                  No Opera: use <code>Shift + Escape + Número</code>
                </li>
              </ul>
            </div>
          </section>

          {/* Card 2: Ferramentas de Contraste e Redimensionamento de Fonte */}
          <section className="flex flex-col gap-4 rounded-xl border border-surface-border bg-surface p-6 shadow-sm">
            <div className="flex items-center gap-2 text-primary font-semibold text-lg border-b border-surface-border pb-3">
              <Type size={20} />
              <h2>Ferramentas de Contraste e Fonte</h2>
            </div>
            <div className="flex flex-col gap-4 text-xs text-muted leading-relaxed">
              <div className="flex items-start gap-3">
                <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-warning/20 text-warning">
                  <Eye size={18} />
                </div>
                <div>
                  <h3 className="font-semibold text-foreground text-sm">Alto Contraste</h3>
                  <p>
                    É possível alterar o contraste de qualquer página através da opção{" "}
                    <strong>&quot;Alto Contraste&quot;</strong> na barra superior. O layout mudará
                    para o padrão e-MAG com fundo preto e acentos amarelos (taxa de contraste 19:1
                    AAA).
                  </p>
                </div>
              </div>

              <div className="flex items-start gap-3">
                <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                  <Type size={18} />
                </div>
                <div>
                  <h3 className="font-semibold text-foreground text-sm">
                    Redimensionamento de Fonte (A+ / A-)
                  </h3>
                  <p>
                    Na barra superior de acessibilidade, utilize os botões <strong>A+</strong> para
                    aumentar o tamanho do texto do documento em até 130% e <strong>A-</strong> para
                    reduzir. O botão <strong>A</strong> restaura o tamanho original.
                  </p>
                </div>
              </div>
            </div>
          </section>

          {/* Card 3: VLibras */}
          <section className="flex flex-col gap-4 rounded-xl border border-surface-border bg-surface p-6 shadow-sm">
            <div className="flex items-center gap-2 text-primary font-semibold text-lg border-b border-surface-border pb-3">
              <Accessibility size={20} />
              <h2>VLibras (Língua Brasileira de Sinais)</h2>
            </div>
            <p className="text-xs text-muted leading-relaxed">
              Através do ícone na barra superior ou do botão abaixo, é possível utilizar a suíte do{" "}
              <strong>VLibras</strong> para traduzir todo o conteúdo do site
              para a Língua Brasileira de Sinais - LIBRAS.
            </p>
            <div className="mt-auto pt-2">
              <a
                href="http://www.vlibras.gov.br/"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-xs font-semibold text-white shadow hover:bg-primary-hover transition-colors"
              >
                <span>Acessar Portal do VLibras</span>
                <ExternalLink size={14} />
              </a>
            </div>
          </section>

          {/* Card 4: Leis e Decretos sobre Acessibilidade */}
          <section className="flex flex-col gap-4 rounded-xl border border-surface-border bg-surface p-6 shadow-sm">
            <div className="flex items-center gap-2 text-primary font-semibold text-lg border-b border-surface-border pb-3">
              <FileText size={20} />
              <h2>Legislação sobre Acessibilidade</h2>
            </div>
            <p className="text-xs text-muted leading-relaxed">
              O {brand.appName} cumpre
              rigorosamente os seguintes normativos legais:
            </p>
            <ul className="flex flex-col gap-3 text-xs">
              <li className="flex flex-col gap-1 border-l-2 border-primary pl-3">
                <a
                  href="http://www.planalto.gov.br/ccivil_03/_ato2011-2014/2012/Decreto/D7724.htm"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="font-semibold text-primary hover:underline inline-flex items-center gap-1"
                >
                  <span>Decreto nº 7.724, de 16 de Maio de 2012</span>
                  <ExternalLink size={12} />
                </a>
                <span className="text-muted text-[11px]">
                  Regulamenta a Lei Federal nº 12.527 (Lei de Acesso à Informação — LAI).
                </span>
              </li>
              <li className="flex flex-col gap-1 border-l-2 border-primary pl-3">
                <a
                  href="http://www.planalto.gov.br/ccivil_03/_Ato2004-2006/2004/Decreto/D5296.htm"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="font-semibold text-primary hover:underline inline-flex items-center gap-1"
                >
                  <span>Decreto nº 5.296, de 02 de Dezembro de 2004</span>
                  <ExternalLink size={12} />
                </a>
                <span className="text-muted text-[11px]">
                  Regulamenta as Leis nº 10.048/2000 e nº 10.098/2000 sobre Acessibilidade.
                </span>
              </li>
            </ul>
          </section>
        </div>

        {/* Declaração formal de acessibilidade (F4.5 / Decreto 9.094) */}
        <section className="flex flex-col gap-4 rounded-xl border border-surface-border bg-surface p-6 shadow-sm">
          <div className="flex items-center gap-2 border-b border-surface-border pb-3 text-lg font-semibold text-primary">
            <FileText size={20} />
            <h2>Declaração de Acessibilidade</h2>
          </div>
          <dl className="grid grid-cols-1 gap-x-6 gap-y-3 text-xs sm:grid-cols-[10rem_1fr]">
            <dt className="font-semibold text-foreground">Conformidade declarada</dt>
            <dd className="text-muted leading-relaxed">
              e-MAG 2.0 / WCAG 2.1 nível AA — <strong>parcialmente conforme</strong>. Os fluxos
              principais (login, painel, configuração, páginas públicas) foram construídos e
              revisados segundo o padrão; a avaliação externa completa (item abaixo) ainda não foi
              concluída.
            </dd>

            <dt className="font-semibold text-foreground">Data da avaliação</dt>
            <dd className="text-muted leading-relaxed">
              Revisão interna contínua; última varredura de referência em 09/09/2026.
            </dd>

            <dt className="font-semibold text-foreground">Responsável</dt>
            <dd className="text-muted leading-relaxed">
              Equipe de tecnologia responsável pela plataforma em {brand.orgName}.
            </dd>

            <dt className="font-semibold text-foreground">Método</dt>
            <dd className="text-muted leading-relaxed">
              (1) Análise automatizada a cada alteração de código no CI (<code>eslint-plugin-jsx-a11y</code>,
              regras e-MAG obrigatórias); (2) revisão manual de contraste, foco visível e semântica
              HTML; (3) navegação apenas por teclado nos fluxos principais; (4) atalhos e-MAG
              (Alt+1..4) com âncoras verificadas em <code>&lt;main&gt;</code>, <code>&lt;nav&gt;</code>{" "}
              e <code>&lt;footer&gt;</code> reais.
            </dd>

            <dt className="font-semibold text-foreground">Pendências conhecidas</dt>
            <dd className="text-muted leading-relaxed">
              Avaliação por especialista humano com tecnologia assistiva (leitores de tela NVDA/VoiceOver,
              leitor móvel) ainda não realizada; correções pontuais de componentes complexos podem
              surgir dessa avaliação. Um VPAT (Voluntary Product Accessibility Template) será anexado
              ao dossiê de conformidade após ela.
            </dd>

            <dt className="font-semibold text-foreground">Contato</dt>
            <dd className="text-muted leading-relaxed">
              Barreiras de acessibilidade podem ser relatadas pelos canais de atendimento da
              Prefeitura (ver rodapé). Detalhes técnicos dos parâmetros aplicados em{" "}
              <Link href="/sobre" className="text-primary hover:underline">Sobre &amp; Conformidade</Link>.
            </dd>
          </dl>
        </section>
      </div>
    </PublicShell>
  );
}
