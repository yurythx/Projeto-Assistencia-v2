# VPAT — Voluntary Product Accessibility Template (minuta)

> **Minuta / autoavaliação.** Este VPAT é preenchido pela engenharia com
> base em análise automatizada (CI) + revisão manual + navegação por
> teclado. Um VPAT **homologado** exige a avaliação por especialista
> humano com tecnologia assistiva (F5.2 do roadmap — pendente). Enquanto
> isso, "Suporta parcialmente" e "Não avaliado" refletem essa limitação.

- **Produto:** Projeto Nexus — base tecnológica dos sistemas municipais
- **Versão:** branch `feat/conformidade-roadmap` (2026-09-09)
- **Padrões avaliados:** WCAG 2.1 níveis A e AA; e-MAG 2.0 (Governo Federal / SGD-MGI)
- **Método:** `eslint-plugin-jsx-a11y` (regras e-MAG) no CI; revisão manual
  de contraste, foco e semântica; navegação só-teclado nos fluxos
  principais (login, painel, configuração, páginas públicas)
- **Legenda de conformidade:** Suporta · Suporta parcialmente · Não suporta · Não avaliado · Não se aplica

---

## Tabela 1 — WCAG 2.1 Nível A

| Critério | Conformidade | Observações |
|---|---|---|
| 1.1.1 Conteúdo não textual | Suporta | Ícones decorativos com `aria-hidden`; ícones informativos com `aria-label`; sem imagens de conteúdo sem `alt` no núcleo |
| 1.2.1–1.2.3 Mídia | Não se aplica | A plataforma-base não distribui áudio/vídeo pré-gravado |
| 1.3.1 Informação e relações | Suporta parcialmente | HTML semântico (`header/nav/main/footer`), `<label htmlFor>`, tabelas com `<caption>`; revisão de ARIA em componentes complexos pendente da avaliação externa |
| 1.3.2 Sequência com significado | Suporta | Ordem do DOM = ordem de leitura |
| 1.3.3 Características sensoriais | Suporta | Instruções não dependem só de forma/cor |
| 1.4.1 Uso de cor | Suporta | Estado também por texto/ícone (status das integrações, erros de formulário) |
| 1.4.2 Controle de áudio | Não se aplica | — |
| 2.1.1 Teclado | Suporta parcialmente | Fluxos principais operáveis só por teclado; varredura completa pendente |
| 2.1.2 Sem armadilha de teclado | Suporta | Modais (LGPD, diálogos) devolvem o foco e fecham com `Esc` |
| 2.1.4 Atalhos de caractere | Suporta | Teclas de acesso e-MAG (Alt+1..4) exigem modificador; sem atalho de tecla única |
| 2.2.1–2.2.2 Tempo / movimento | Suporta | `prefers-reduced-motion` respeitado; sem limite de tempo em formulários |
| 2.3.1 Três flashes | Suporta | Sem conteúdo piscante |
| 2.4.1 Ignorar blocos | Suporta | Skip links (Alt+1 conteúdo, Alt+2 menu, Alt+4 rodapé) com âncoras focáveis nos dois shells |
| 2.4.2 Página com título | Suporta | `<title>` distinto por rota (`Metadata` do Next) |
| 2.4.3 Ordem de foco | Suporta parcialmente | Ordem lógica; revisão em telas densas pendente |
| 2.4.4 Finalidade do link | Suporta | Texto de link descritivo; ícones externos com `ExternalLink` |
| 2.5.1–2.5.4 Gestos / entrada por ponteiro | Suporta | Sem gesto multitoque ou baseado em trajetória obrigatório |
| 3.1.1 Idioma da página | Suporta | `<html lang="pt-BR">` |
| 3.2.1 Em foco / 3.2.2 Em entrada | Suporta | Sem mudança de contexto ao focar/preencher |
| 3.3.1 Identificação de erro | Suporta | Erros de formulário com `role="alert"` + `aria-describedby` |
| 3.3.2 Rótulos ou instruções | Suporta | Todo campo com `<label>` |
| 4.1.1 Análise | Suporta | HTML válido (build do Next falha em erro de marcação) |
| 4.1.2 Nome, função, valor | Suporta parcialmente | Componentes nativos + ARIA nos customizados; auditoria completa de ARIA pendente |

## Tabela 2 — WCAG 2.1 Nível AA

| Critério | Conformidade | Observações |
|---|---|---|
| 1.2.4–1.2.5 Legendas/áudio-descrição ao vivo | Não se aplica | — |
| 1.3.4 Orientação | Suporta | Layout responsivo, sem travar orientação |
| 1.3.5 Identificar finalidade da entrada | Suporta parcialmente | `type`/`name` adequados; `autocomplete` a revisar em formulários de perfil |
| 1.4.3 Contraste (mínimo) | Suporta | Tokens documentados no `globals.css` com a razão de contraste (ex.: `--muted` ≈ 5.4:1); modo Alto Contraste e-MAG ≈ 19:1 |
| 1.4.4 Redimensionar texto | Suporta | Escala de fonte 90–130% (barra e-MAG), tipografia em `rem` |
| 1.4.5 Imagens de texto | Suporta | Sem imagem de texto; logotipo é SVG/ícone |
| 1.4.10 Refluxo | Suporta parcialmente | Sem rolagem horizontal em 320px nas páginas públicas; telas densas do painel a revisar |
| 1.4.11 Contraste de não texto | Suporta parcialmente | Bordas/ícones de UI revisados no tema claro; varredura do tema escuro pendente |
| 1.4.12 Espaçamento de texto | Suporta | Layout tolera ajuste de espaçamento do usuário |
| 1.4.13 Conteúdo ao passar/focar | Suporta | Tooltips/menus dispensáveis e persistentes |
| 2.4.5 Várias formas | Suporta | Navegação por menu + skip links + `sitemap.xml` |
| 2.4.6 Cabeçalhos e rótulos | Suporta | Hierarquia de `h1..h3` sem pular níveis (`CardTitle as=` existe para isso) |
| 2.4.7 Foco visível | Suporta | `focus-visible` com anel em todos os alvos interativos |
| 3.1.2 Idioma de partes | Suporta | Conteúdo em pt-BR; termos estrangeiros pontuais sem `lang` — impacto baixo |
| 3.2.3 Navegação consistente | Suporta | Shell/rodapé consistentes entre páginas |
| 3.2.4 Identificação consistente | Suporta | Componentes do design system reutilizados |
| 3.3.3 Sugestão de erro | Suporta | Mensagens de validação orientam a correção |
| 3.3.4 Prevenção de erro (legal/financeiro) | Suporta parcialmente | Ações sensíveis (ex.: solicitar exclusão LGPD) são idempotentes e confirmáveis; revisão de confirmações em ações administrativas pendente |
| 4.1.3 Mensagens de status | Suporta | Toasts e estados com `role="alert"`/`aria-live` |

## Pendências para o VPAT homologado

- Avaliação por especialista com **NVDA / VoiceOver / leitor móvel** (F5.2).
- Varredura completa de **foco/ordem** e **ARIA** nas telas densas do painel.
- Revisão de **contraste de não texto no tema escuro**.
- Teste de **refluxo a 320px** nas telas autenticadas.
