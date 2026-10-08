import { Layers, Lock, Radar, RefreshCw, ShieldCheck } from "lucide-react";
import type { Metadata } from "next";
import { connection } from "next/server";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { PublicShell } from "@/components/layout/PublicShell";
import { getServerBranding } from "@/lib/branding/server";
import { Table, TableBody, TableCell, TableHead, TableHeaderCell, TableRow } from "@/components/ui/Table";

const description =
  "Visão geral da arquitetura da plataforma e sua conformidade com as normas do Governo Federal (e-MAG, DSGov, LGPD, e-PING e OWASP).";

export const metadata: Metadata = {
  title: "Sobre & Conformidade",
  description,
  openGraph: { title: "Sobre & Conformidade", description, type: "website" },
};

// Detalhamento dos 5 Módulos de Conformidade Governamental
const govConformityModules = [
  {
    module: "Módulo 1: Segurança HTTP & Headers (Go Backend)",
    norm: "Portarias SGD/MGI & OWASP Top 10",
    tech: "Middleware Go (httpserver/middleware.go), HSTS, CSP com Nonce, Rate Limiter Postgres",
    justification: "Injeta cabeçalhos defensivos estritos em 100% das respostas HTTP (HSTS max-age=63072000, X-Frame-Options DENY, X-Content-Type-Options nosniff) e limita requisições por IP/token para prevenir ataques DoS/Slowloris e injeção de scripts maliciosos.",
  },
  {
    module: "Módulo 2: Privacidade & LGPD (Go Backend)",
    norm: "Lei Geral de Proteção de Dados (Lei 13.709/2018)",
    tech: "log/slog nativo em JSON, slog.LogValuer (MaskCPF, MaskEmail, MaskPhone) e X-Request-ID",
    justification: "Garante que dados pessoais sensíveis (PII) de cidadãos e servidores nunca vazem em texto puro nos logs do sistema. Toda requisição recebe um X-Request-ID correlacionado para rastreabilidade auditável de ponta a ponta.",
  },
  {
    module: "Módulo 3: Autenticação Federada & Interoperabilidade",
    norm: "Portaria SGD/SEDGG Nº 2.154 & e-PING",
    tech: "go-oidc/v3, JWKS do Keycloak dedicado (cache + rotação), federação LDAP/LDAPS com o AD, fallback RS256 e OpenAPI 3.0",
    justification: "Verifica a assinatura JWT localmente via JWKS sem chamadas adicionais por requisição e mapeia grupos do Active Directory para Perfis e escopos organizacionais (RBAC multi-escopo). Expõe especificações RESTful e-PING.",
  },
  {
    module: "Módulo 4: Acessibilidade Digital e-MAG",
    norm: "e-MAG 2.0 / WCAG 2.1 Nível AA",
    tech: "EMagAccessibilityBar, SkipLinks (Alt+1..4), VLibras nativo, Alto Contraste e eslint-plugin-jsx-a11y",
    justification: "Assegura o direito de acesso à informação para pessoas com deficiência. Inclui barra e-MAG com atalhos de navegação por teclado, leitor visual de alto contraste (preto/amarelo), escala de fonte proporcional e tradutor de Libras em todas as páginas.",
  },
  {
    module: "Módulo 5: Identidade Visual Governamental DSGov",
    norm: "Padrão Digital de Governo (GovBR-DS)",
    tech: "BrandingProvider Server-Side, Tokens Tailwind CSS v4, GovHeader, GovFooter e Fontes Oficiais",
    justification: "Garante a padronização estática e dinâmica de portais públicos, com suporte White-Label dinâmico para prefeituras e órgãos, rodapé institucional completo com LGPD/LAI e eliminação total de flashes de layout na hidratação.",
  },
];

// Resumo público da matriz OWASP
const owaspMapping = [
  { code: "A01", risk: "Broken Access Control", today: "RBAC por permissão em cada rota sensível — nunca só a presença de um token." },
  { code: "A02", risk: "Cryptographic Failures", today: "RS256 com chave própria para o login local, bcrypt, segredos via arquivo, nunca em texto puro." },
  { code: "A03", risk: "Injection", today: "100% consultas parametrizadas — conferido: zero concatenação de string em SQL em todo o backend." },
  { code: "A04", risk: "Insecure Design", today: "Arquitetura Microkernel com módulos de negócio isolados (plug-ins) e decisões de arquitetura documentadas em ADRs." },
  { code: "A05", risk: "Security Misconfiguration", today: "CSP com nonce por requisição, containers non-root, headers de segurança em toda resposta." },
  { code: "A06", risk: "Vulnerable Components", today: "govulncheck no backend a cada push/PR no CI; lint, typecheck e build verificados no frontend na mesma pipeline." },
  { code: "A07", risk: "Identification & Auth Failures", today: "Bloqueio de conta, rate limiting distribuído, erro sempre genérico (nunca revela se um usuário existe)." },
  { code: "A08", risk: "Software & Data Integrity Failures", today: "Idempotência e outbox transacional com entrega Exactly-Once para novos módulos." },
  { code: "A09", risk: "Logging & Monitoring Failures", today: "Auditoria imutável (a tabela recusa UPDATE/DELETE), logs correlacionados por request id, métricas e tracing." },
  { code: "A10", risk: "SSRF", today: "Conferido: nenhum endpoint aceita uma URL arbitrária vinda de quem chama." },
];

const principles = [
  {
    icon: Layers,
    title: "Arquitetura Microkernel (Plug-in System)",
    description:
      "Core System (Kernel) robusto em Go 1.25 que fornece infraestrutura de segurança e auth, pronto para acoplamento de novos plug-ins de negócio com desacoplamento total.",
  },
  {
    icon: RefreshCw,
    title: "Resiliência & Outbox Transacional",
    description:
      "Publicação de eventos no RabbitMQ via Transactional Outbox Pattern com suporte a filas de mensagens mortas (DLQ).",
  },
  {
    icon: Lock,
    title: "DevSecOps & Privacidade",
    description:
      "Autenticação Keycloak OIDC com federação AD, mascaramento de PII em logs, CSP estrita com nonce e trilhas de auditoria imutáveis no Postgres.",
  },
  {
    icon: Radar,
    title: "Acessibilidade & Observabilidade",
    description:
      "Conformidade e-MAG 2.0 / WCAG 2.1 AA com VLibras, métricas Prometheus e rastreamento correlacionado de requisições.",
  },
];

export default async function AboutPage() {
  await connection();
  const brand = await getServerBranding();

  return (
    <PublicShell>
      <div className="mx-auto flex w-full max-w-4xl flex-1 flex-col gap-12 px-6 py-12">
        <section className="flex flex-col gap-4">
          <div className="flex items-center gap-2 text-xs font-bold font-mono text-primary uppercase tracking-wider">
            <ShieldCheck size={16} className="text-success" />
            Conformidade Federal SGD/MGI
          </div>
          <h1 className="text-3xl font-bold text-foreground">Sobre a plataforma & diretrizes governamentais</h1>
          <p className="text-muted leading-relaxed">
            O <strong>{brand.appName}</strong>, operado por <strong>{brand.orgName}</strong>, é uma base Microkernel que unifica os rígidos padrões de <strong>Segurança (OWASP), Acessibilidade (e-MAG / WCAG 2.1 AA), Identidade Visual (DSGov), Privacidade (LGPD) e Interoperabilidade (OIDC / e-PING)</strong>.
          </p>
        </section>

        {/* Tabela detalhada de Conformidade Governamental */}
        <section className="flex flex-col gap-4">
          <div>
            <h2 className="text-xl font-semibold text-foreground">Conformidade com Normas Governamentais (SGD/MGI)</h2>
            <p className="mt-1 text-sm text-muted">
              Mapeamento dos 5 módulos de conformidade implementados na plataforma e suas justificativas técnicas.
            </p>
          </div>
          <div className="overflow-hidden rounded-xl border border-surface-border bg-surface">
            <Table caption="Detalhamento das diretrizes federais e justificativas de implementação.">
              <TableHead>
                <TableRow>
                  <TableHeaderCell>Módulo & Norma</TableHeaderCell>
                  <TableHeaderCell>Tecnologias & Métodos</TableHeaderCell>
                  <TableHeaderCell>Justificativa Técnica de Uso</TableHeaderCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {govConformityModules.map((item) => (
                  <TableRow key={item.module}>
                    <TableCell className="align-top font-medium text-foreground">
                      <div className="flex flex-col gap-1">
                        <span className="font-bold text-xs text-primary">{item.module}</span>
                        <span className="text-[11px] font-mono text-seal">{item.norm}</span>
                      </div>
                    </TableCell>
                    <TableCell className="align-top font-mono text-xs text-muted">
                      {item.tech}
                    </TableCell>
                    <TableCell className="align-top text-xs text-muted leading-relaxed">
                      {item.justification}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </section>

        <section className="flex flex-col gap-4">
          <h2 className="text-xl font-semibold text-foreground">Princípios Arquiteturais</h2>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            {principles.map((principle) => {
              const Icon = principle.icon;
              return (
                <Card key={principle.title}>
                  <CardHeader className="flex flex-row items-center gap-3 space-y-0">
                    <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                      <Icon size={18} aria-hidden="true" />
                    </span>
                    <CardTitle className="text-base">{principle.title}</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <p className="text-sm text-muted">{principle.description}</p>
                  </CardContent>
                </Card>
              );
            })}
          </div>
        </section>

        <section className="flex flex-col gap-4">
          <div>
            <h2 className="text-xl font-semibold text-foreground">OWASP Top 10 Enterprise</h2>
            <p className="mt-1 text-sm text-muted">
              Tratado como checklist de engenharia desde o primeiro commit. A coluna da direita descreve a prática hoje na plataforma.
            </p>
          </div>
          <Table caption="Riscos do OWASP Top 10 e a prática correspondente adotada na plataforma.">
            <TableHead>
              <TableRow>
                <TableHeaderCell>Risco</TableHeaderCell>
                <TableHeaderCell>Prática na plataforma</TableHeaderCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {owaspMapping.map((item) => (
                <TableRow key={item.code}>
                  <TableCell className="whitespace-nowrap align-top font-medium text-foreground">
                    <span className="font-mono text-xs font-semibold text-seal">{item.code}</span>{" "}
                    {item.risk}
                  </TableCell>
                  <TableCell className="text-muted">{item.today}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </section>

        <section className="flex flex-col gap-3 rounded-xl border border-surface-border bg-surface p-6">
          <h2 className="text-xl font-semibold text-foreground">Stack Tecnológica & Padrões de Projeto</h2>
          <p className="text-sm text-muted leading-relaxed">
            <strong className="text-foreground">Backend (Go 1.25):</strong> Arquitetura Limpa (Clean Architecture), roteador Chi v5, banco de dados PostgreSQL 16 com `pgxpool`, mensageria RabbitMQ via AMQP, `log/slog` com mascaramento PII, e suporte a testes unitários com 100% de mocks zerados.
          </p>
          <p className="text-sm text-muted leading-relaxed">
            <strong className="text-foreground">Frontend (Next.js 16 App Router):</strong> React 19 em TypeScript estrito, Tailwind CSS v4 com temas DSGov/e-MAG, `NextAuth.js` com SSO Keycloak, widget VLibras desacoplado da hidratação, e auditoria de acessibilidade por `jsx-a11y`.
          </p>
        </section>
      </div>
    </PublicShell>
  );
}
