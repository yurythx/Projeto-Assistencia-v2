import { FileText, Lock, Mail, ShieldCheck, UserCheck } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { connection } from "next/server";

import { PublicShell } from "@/components/layout/PublicShell";
import { getServerBranding } from "@/lib/branding/server";

// Casada com lgpd.CurrentTermVersion no backend
// (backend/internal/platform/lgpd/lgpd.go). Trocar a versão aqui e lá
// reobriga todos os usuários a aceitar de novo.
const TERM_VERSION = "v1.0.0-2026";

const description =
  "Política de Privacidade e Proteção de Dados Pessoais da plataforma, em conformidade com a LGPD (Lei 13.709/2018).";

export const metadata: Metadata = {
  title: "Política de Privacidade",
  description,
  openGraph: { title: "Política de Privacidade", description, type: "website" },
};

const dataCategories = [
  {
    categoria: "Identificação e credenciais",
    itens: "Nome de usuário, e-mail, nome de exibição, hash da senha (login local), identificador no Keycloak / Active Directory, grupos, perfis e lotações.",
    base: "Execução de políticas públicas e prestação de serviço público (art. 7º, III) e, no login local, consentimento (art. 7º, I).",
  },
  {
    categoria: "Registros de acesso e uso",
    itens: "Data e hora de login/logout, endereço IP de origem, identificador de correlação da requisição, ações realizadas no sistema (trilha de auditoria).",
    base: "Cumprimento de obrigação legal e regulatória (art. 7º, II) — Marco Civil da Internet e normas de controle interno (CGU/TCE).",
  },
  {
    categoria: "Registros de consentimento",
    itens: "Versão dos termos aceita, data/hora, IP e user-agent do aceite.",
    base: "Cumprimento de obrigação legal (art. 7º, II) e comprovação do consentimento (art. 8º, §1º).",
  },
  {
    categoria: "Preferências de acessibilidade e interface",
    itens: "Alto contraste, escala de fonte, tema — guardados em cookie/armazenamento local do navegador.",
    base: "Legítimo interesse em prover uma experiência acessível (art. 7º, IX); não saem do seu navegador.",
  },
];

const rights = [
  ["Confirmação e acesso", "Baixe tudo que a plataforma guarda sobre você em GET /api/v1/lgpd/meus-dados (JSON estruturado) — ou peça pelo canal do Encarregado."],
  ["Portabilidade", "O mesmo pacote JSON acima é interoperável e serve para portabilidade a outro fornecedor."],
  ["Correção", "Dados de identificação vêm do Active Directory / cadastro do órgão — a correção é feita na fonte."],
  ["Eliminação / anonimização", "POST /api/v1/lgpd/solicitar-exclusao registra o pedido; a conta é anonimizada automaticamente. A trilha de auditoria e os registros de consentimento são mantidos como registro legal (art. 16, I e III)."],
  ["Informação sobre compartilhamento", "Ver a seção “Com quem compartilhamos” abaixo."],
  ["Revogação do consentimento", "Aplicável ao login local; revogar implica não conseguir mais autenticar por esse meio."],
];

export default async function PrivacidadePage() {
  await connection();
  const brand = await getServerBranding();

  return (
    <PublicShell>
      <div className="mx-auto flex w-full max-w-3xl flex-1 flex-col gap-10 px-6 py-12">
        <section className="flex flex-col gap-4">
          <div className="flex items-center gap-2 text-xs font-bold font-mono text-primary uppercase tracking-wider">
            <ShieldCheck size={16} className="text-success" />
            LGPD · Lei 13.709/2018 · Versão {TERM_VERSION}
          </div>
          <h1 className="text-3xl font-bold text-foreground">Política de Privacidade e Proteção de Dados</h1>
          <p className="text-muted leading-relaxed">
            Esta política descreve como o <strong>{brand.appName}</strong>, operado por{" "}
            <strong>{brand.orgName}</strong>, trata dados pessoais de servidores e
            cidadãos que utilizam seus sistemas, em conformidade com a Lei Geral de Proteção de Dados.
          </p>
          <p className="rounded-lg border border-warning/40 bg-warning/5 px-4 py-3 text-xs text-muted">
            <strong className="text-foreground">Minuta.</strong> Este texto é um ponto de partida técnico e
            aguarda revisão e homologação do Encarregado de Dados (DPO) do órgão antes de valer como
            documento oficial.
          </p>
        </section>

        <section className="flex flex-col gap-3">
          <h2 className="flex items-center gap-2 text-xl font-semibold text-foreground">
            <UserCheck size={18} className="text-primary" aria-hidden="true" /> 1. Controlador
          </h2>
          <p className="text-sm text-muted leading-relaxed">
            O controlador dos dados é <strong>{brand.orgName}</strong>. Cada módulo ativo da plataforma
            trata apenas os dados necessários à sua finalidade, com trilha de auditoria imutável.
          </p>
        </section>

        <section className="flex flex-col gap-3">
          <h2 className="text-xl font-semibold text-foreground">2. Quais dados tratamos e com que base legal</h2>
          <div className="overflow-x-auto rounded-lg border border-surface-border">
            <table className="w-full text-left text-sm">
              <thead className="bg-black/5 dark:bg-white/5">
                <tr>
                  <th className="px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-muted">Categoria</th>
                  <th className="px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-muted">Dados</th>
                  <th className="px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-muted">Base legal (LGPD art. 7º/8º)</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-surface-border">
                {dataCategories.map((d) => (
                  <tr key={d.categoria}>
                    <td className="px-4 py-3 align-top text-xs font-semibold text-foreground">{d.categoria}</td>
                    <td className="px-4 py-3 align-top text-xs text-muted leading-relaxed">{d.itens}</td>
                    <td className="px-4 py-3 align-top text-xs text-muted leading-relaxed">{d.base}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>

        <section className="flex flex-col gap-3">
          <h2 className="text-xl font-semibold text-foreground">3. Finalidades</h2>
          <ul className="flex list-disc flex-col gap-1.5 pl-5 text-sm text-muted leading-relaxed">
            <li>Autenticar e autorizar o acesso aos sistemas do órgão (SSO Keycloak com federação AD e login local).</li>
            <li>Manter trilha de auditoria imutável das operações, para controle interno e transparência (LAI).</li>
            <li>Registrar o aceite dos Termos de Uso e desta Política.</li>
            <li>Aplicar preferências de acessibilidade e interface.</li>
            <li>Garantir a segurança da informação (rate limiting, detecção de força bruta).</li>
          </ul>
        </section>

        <section className="flex flex-col gap-3">
          <h2 className="text-xl font-semibold text-foreground">4. Com quem compartilhamos</h2>
          <p className="text-sm text-muted leading-relaxed">
            Não há venda ou compartilhamento de dados pessoais com terceiros para fins comerciais. Há
            compartilhamento operacional com:
          </p>
          <ul className="flex list-disc flex-col gap-1.5 pl-5 text-sm text-muted leading-relaxed">
            <li><strong>Keycloak / Active Directory</strong> — autenticação federada (identidade e grupos corporativos).</li>
            <li><strong>Órgãos de controle</strong> (CGU, TCE, Ministério Público) — mediante requisição legal, a trilha de auditoria higienizada.</li>
            <li><strong>Provedores de infraestrutura</strong> contratados pelo órgão para hospedar a aplicação, como operadores, sob contrato e instrução do controlador.</li>
          </ul>
        </section>

        <section className="flex flex-col gap-3">
          <h2 className="flex items-center gap-2 text-xl font-semibold text-foreground">
            <Lock size={18} className="text-primary" aria-hidden="true" /> 5. Segurança e retenção
          </h2>
          <p className="text-sm text-muted leading-relaxed">
            Senhas do login local são armazenadas apenas como hash (bcrypt). Segredos de configuração são
            cifrados (AES-256-GCM) antes de irem ao banco. O tráfego usa cabeçalhos de segurança estritos
            (CSP, HSTS) e a trilha de auditoria é append-only no nível do banco, com cópia externa
            encadeada por hash. Dados de identificação são mantidos enquanto a conta estiver ativa; a
            trilha de auditoria segue os prazos de guarda da legislação de controle interno. Detalhes
            técnicos em{" "}
            <Link href="/sobre" className="text-primary hover:underline">Sobre &amp; Conformidade</Link>.
          </p>
        </section>

        <section className="flex flex-col gap-3">
          <h2 className="flex items-center gap-2 text-xl font-semibold text-foreground">
            <FileText size={18} className="text-primary" aria-hidden="true" /> 6. Seus direitos e como exercê-los
          </h2>
          <div className="overflow-x-auto rounded-lg border border-surface-border">
            <table className="w-full text-left text-sm">
              <thead className="bg-black/5 dark:bg-white/5">
                <tr>
                  <th className="px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-muted">Direito (LGPD art. 18)</th>
                  <th className="px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-muted">Como exercer</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-surface-border">
                {rights.map(([r, how]) => (
                  <tr key={r}>
                    <td className="px-4 py-3 align-top text-xs font-semibold text-foreground">{r}</td>
                    <td className="px-4 py-3 align-top text-xs text-muted leading-relaxed">{how}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>

        <section className="flex flex-col gap-3">
          <h2 className="text-xl font-semibold text-foreground">7. Cookies e armazenamento local</h2>
          <p className="text-sm text-muted leading-relaxed">
            A plataforma usa um cookie de sessão (criptografado, <code>HttpOnly</code>) para manter você
            autenticado e o armazenamento local do navegador para lembrar tema, contraste, escala de
            fonte e o aceite desta Política. Não há cookies de rastreamento publicitário nem analytics de
            terceiros.
          </p>
        </section>

        <section className="flex flex-col gap-3 rounded-xl border border-surface-border bg-surface p-6">
          <h2 className="flex items-center gap-2 text-lg font-semibold text-foreground">
            <Mail size={18} className="text-primary" aria-hidden="true" /> 8. Encarregado de Dados (DPO)
          </h2>
          <p className="text-sm text-muted leading-relaxed">
            Solicitações relativas a dados pessoais podem ser feitas pelos canais de atendimento de{" "}
            {brand.orgName} (ver rodapé) ou pelo canal específico do Encarregado de
            Proteção de Dados, <span className="italic">a ser designado e publicado pelo órgão</span>.
          </p>
        </section>

        <section className="flex flex-col gap-2 border-t border-surface-border pt-6 text-xs text-muted">
          <h2 className="text-sm font-semibold text-foreground">Histórico de versões</h2>
          <p><code className="font-mono">{TERM_VERSION}</code> — versão inicial (minuta técnica), 2026.</p>
        </section>
      </div>
    </PublicShell>
  );
}
