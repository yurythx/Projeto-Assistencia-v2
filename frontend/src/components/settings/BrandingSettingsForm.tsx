"use client";

import { useState } from "react";
import { Building2, Save, RotateCcw, ShieldCheck } from "lucide-react";

import { useBranding, DEFAULT_BRANDING, type SystemBrandingConfig } from "@/components/branding/BrandingContext";
import { mergeServerBranding } from "@/components/branding/brandingConfig";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Textarea } from "@/components/ui/Textarea";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Dialog } from "@/components/ui/Dialog";
import { useToast } from "@/components/notifications/ToastProvider";
import { apiClient, ApiError } from "@/lib/api/client";
import { useNexus } from "@/lib/nexus/NexusProvider";
import type { Branding } from "@/lib/nexus/types";
import { safeResourceUrl } from "@/lib/security/safe-url";

import { refreshBranding } from "./brandingActions";

/** Formato do PUT /admin/branding. Os design tokens de cor não são
 * editados aqui e seguem como estão (o reset os limpa). */
function toApi(c: SystemBrandingConfig): Omit<Branding, "updated_at"> {
  return {
    app_name: c.appName.trim(),
    app_description: c.appDescription.trim(),
    org_name: c.orgName.trim(),
    logo_url: c.logoUrl.trim(),
    favicon_url: c.faviconUrl.trim(),
    support_email: c.supportEmail.trim(),
    support_phone: c.supportPhone.trim(),
    support_hours: c.supportHours.trim(),
    tokens: c.tokens ?? {},
  };
}

export function BrandingSettingsForm() {
  const { branding, updateBranding } = useBranding();
  const { showToast } = useToast();
  const { can } = useNexus();
  const canManage = can("branding:manage");
  const [saving, setSaving] = useState(false);

  const [form, setForm] = useState<SystemBrandingConfig>(branding);
  const [logoPreviewError, setLogoPreviewError] = useState(false);
  const [faviconPreviewError, setFaviconPreviewError] = useState(false);
  const [confirmResetOpen, setConfirmResetOpen] = useState(false);

  // Ressincroniza o formulário quando o branding do contexto muda (após
  // salvar/restaurar). Ajuste de estado durante o render — padrão do React.
  const [syncedBranding, setSyncedBranding] = useState(branding);
  if (branding !== syncedBranding) {
    setSyncedBranding(branding);
    setForm(branding);
  }

  const handleChange = (field: keyof SystemBrandingConfig, value: string | boolean) => {
    setForm((prev) => ({ ...prev, [field]: value }));
    if (field === "logoUrl") setLogoPreviewError(false);
    if (field === "faviconUrl") setFaviconPreviewError(false);
  };

  // A identidade é da ORGANIZAÇÃO: grava no backend (validação de URL e
  // de contraste WCAG, auditoria) e só então atualiza a tela. Antes o
  // formulário mudava apenas o estado desta aba e anunciava "salvo" — um
  // recarregamento, ou qualquer outro usuário, via o branding antigo.
  async function persist(config: SystemBrandingConfig, success: { title: string; description: string; tone: "success" | "info" }) {
    setSaving(true);
    try {
      const { data } = await apiClient.put<Branding>("v1/admin/branding", toApi(config));
      const saved = mergeServerBranding(data, branding);
      // Expira o cache do servidor; se falhar, o valor novo aparece quando
      // o cache de 60s vencer (a gravação em si já deu certo).
      await refreshBranding().catch(() => undefined);
      updateBranding(saved);
      setForm(saved);
      showToast(success);
      return true;
    } catch (err) {
      showToast({
        title: "Não foi possível salvar",
        description: err instanceof ApiError ? err.message : "Tente novamente.",
        tone: "danger",
      });
      return false;
    } finally {
      setSaving(false);
    }
  }

  const handleSave = (e: React.FormEvent) => {
    e.preventDefault();
    void persist(form, {
      title: "Configurações salvas",
      description: "A identidade visual foi atualizada para todos os usuários.",
      tone: "success",
    });
  };

  const doReset = async () => {
    const ok = await persist({ ...branding, ...DEFAULT_BRANDING, highContrast: branding.highContrast, fontSizeScale: branding.fontSizeScale }, {
      title: "Padrões restaurados",
      description: "A identidade visual padrão foi reestabelecida.",
      tone: "info",
    });
    if (ok) setConfirmResetOpen(false);
  };

  // S-03: só usa a URL da logo/favicon como <img src>/<link href> se for
  // https:// bem-formada.
  const safeLogo = logoPreviewError ? null : safeResourceUrl(form.logoUrl);
  const safeFavicon = faviconPreviewError ? null : safeResourceUrl(form.faviconUrl);

  return (
    <Card className="border border-surface-border bg-surface">
      <CardHeader className="border-b border-surface-border">
        <CardTitle as="h2" className="flex items-center gap-2 text-base font-bold text-foreground">
          <Building2 className="h-5 w-5 text-primary" aria-hidden="true" />
          Identidade Visual Institucional & Branding White-Label
        </CardTitle>
        <p className="mt-1 text-xs text-muted">
          Personalize o nome da aplicação, logomarca, descrição e dados de contato do órgão.
        </p>
      </CardHeader>

      <CardContent className="p-6">
        {!canManage && (
          <p role="note" className="mb-4 rounded-md bg-warning/10 p-3 text-xs">
            Somente leitura: alterar a identidade visual exige a permissão <code className="font-mono">branding:manage</code>.
          </p>
        )}
        <form onSubmit={handleSave} className="flex flex-col gap-6 text-xs">
          <fieldset disabled={!canManage || saving} className="contents">
          {/* Seção 1: Identificação */}
          <div className="flex flex-col gap-4">
            <h3 className="border-b border-surface-border pb-1 text-xs font-bold uppercase tracking-wider text-primary">
              1. Identificação & Metadados
            </h3>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <Input
                label="Nome da Aplicação / Sistema *"
                name="appName"
                autoComplete="off"
                value={form.appName}
                onChange={(e) => handleChange("appName", e.target.value)}
                placeholder="Ex: Portal da Prefeitura"
                required
              />
              <Input
                label="Órgão / Prefeitura Municipal *"
                name="orgName"
                autoComplete="organization"
                value={form.orgName}
                onChange={(e) => handleChange("orgName", e.target.value)}
                placeholder="Ex: Secretaria de Governo Digital"
                required
              />
            </div>
            <Textarea
              label="Descrição Institucional"
              name="appDescription"
              rows={2}
              value={form.appDescription}
              onChange={(e) => handleChange("appDescription", e.target.value)}
              placeholder="Descrição curta para a barra e-MAG e metadados…"
            />
          </div>

          {/* Seção 2: Logomarca */}
          <div className="flex flex-col gap-4">
            <h3 className="border-b border-surface-border pb-1 text-xs font-bold uppercase tracking-wider text-primary">
              2. Logomarca & Imagens Institucionais
            </h3>
            <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
              <div className="flex flex-col gap-2">
                <Input
                  label="URL da Logomarca (PNG/SVG/WebP)"
                  name="logoUrl"
                  type="url"
                  inputMode="url"
                  autoComplete="off"
                  value={form.logoUrl}
                  onChange={(e) => handleChange("logoUrl", e.target.value)}
                  placeholder="https://exemplo.gov.br/logo.png"
                />
                <p className="text-[11px] text-muted">
                  Apenas URLs <code className="font-mono">https://</code> ou caminhos locais (<code className="font-mono">/logo.svg</code>) são aceitos.
                </p>
              </div>

              {/* Preview — espaço reservado fixo para não gerar CLS (P-01) */}
              <div className="flex min-h-[100px] flex-col items-center justify-center rounded-lg border border-surface-border bg-surface-hover/30 p-4">
                <span className="mb-2 text-[10px] font-bold uppercase text-muted">
                  Pré-visualização da Logomarca
                </span>
                {safeLogo ? (
                  <span className="inline-flex h-10 w-[180px] items-center justify-center overflow-hidden">
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img
                      src={safeLogo}
                      alt={`Logomarca de ${form.appName || "aplicação"}`}
                      width={180}
                      height={40}
                      onError={() => setLogoPreviewError(true)}
                      className="h-10 w-auto max-w-full object-contain"
                    />
                  </span>
                ) : (
                  <div className="flex items-center gap-2 text-muted">
                    <ShieldCheck className="h-6 w-6 text-primary" aria-hidden="true" />
                    <span className="text-xs font-semibold">{form.appName || "Projeto Nexus"}</span>
                    <span className="text-[10px] text-muted">(fallback vetorial)</span>
                  </div>
                )}
              </div>

              <div className="flex flex-col gap-2">
                <Input
                  label="URL do Favicon (ICO/PNG/SVG)"
                  name="faviconUrl"
                  type="url"
                  inputMode="url"
                  autoComplete="off"
                  value={form.faviconUrl}
                  onChange={(e) => handleChange("faviconUrl", e.target.value)}
                  placeholder="https://exemplo.gov.br/favicon.png"
                />
                <p className="text-[11px] text-muted">
                  Apenas URLs <code className="font-mono">https://</code> são aceitas. Sem isto, a
                  aplicação usa o selo padrão da plataforma.
                </p>
              </div>

              {/* Preview — mesmo raciocínio de CLS do preview da logo acima. */}
              <div className="flex min-h-[100px] flex-col items-center justify-center rounded-lg border border-surface-border bg-surface-hover/30 p-4">
                <span className="mb-2 text-[10px] font-bold uppercase text-muted">
                  Pré-visualização do Favicon
                </span>
                {safeFavicon ? (
                  <span className="inline-flex h-8 w-8 items-center justify-center overflow-hidden">
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img
                      src={safeFavicon}
                      alt="Favicon personalizado"
                      width={32}
                      height={32}
                      onError={() => setFaviconPreviewError(true)}
                      className="h-8 w-8 object-contain"
                    />
                  </span>
                ) : (
                  <div className="flex items-center gap-2 text-muted">
                    <ShieldCheck className="h-6 w-6 text-primary" aria-hidden="true" />
                    <span className="text-[10px] text-muted">(favicon padrão)</span>
                  </div>
                )}
              </div>
            </div>
          </div>

          {/* Seção 3: Contatos */}
          <div className="flex flex-col gap-4">
            <h3 className="border-b border-surface-border pb-1 text-xs font-bold uppercase tracking-wider text-primary">
              3. Canais de Atendimento & Suporte
            </h3>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
              <Input
                label="E-mail de Suporte"
                name="supportEmail"
                type="email"
                inputMode="email"
                autoComplete="email"
                value={form.supportEmail}
                onChange={(e) => handleChange("supportEmail", e.target.value)}
                placeholder="suporte@municipio.gov.br"
              />
              <Input
                label="Telefone de Atendimento"
                name="supportPhone"
                type="tel"
                inputMode="tel"
                autoComplete="tel"
                value={form.supportPhone}
                onChange={(e) => handleChange("supportPhone", e.target.value)}
                placeholder="(66) 3411-5000"
              />
              <Input
                label="Horário de Atendimento"
                name="supportHours"
                autoComplete="off"
                value={form.supportHours}
                onChange={(e) => handleChange("supportHours", e.target.value)}
                placeholder="Segunda a Sexta, 08h às 17h"
              />
            </div>
          </div>

          </fieldset>
          <div className="flex items-center justify-between border-t border-surface-border pt-4">
            <Button
              type="button"
              variant="ghost"
              onClick={() => setConfirmResetOpen(true)}
              disabled={!canManage || saving}
              className="text-danger hover:bg-danger/10"
            >
              <RotateCcw className="mr-1.5 h-4 w-4" aria-hidden="true" />
              Restaurar padrões
            </Button>
            <Button type="submit" variant="primary" disabled={!canManage} loading={saving}>
              <Save className="mr-1.5 h-4 w-4" aria-hidden="true" />
              Salvar alterações
            </Button>
          </div>
        </form>
      </CardContent>

      {/* D-01: confirmação de ação destrutiva pelo Dialog do kit, não confirm() */}
      <Dialog
        open={confirmResetOpen}
        onClose={() => setConfirmResetOpen(false)}
        title="Restaurar configurações padrão?"
        description="A identidade visual (nome, logo, contatos) volta ao padrão da aplicação. Esta ação não pode ser desfeita."
        footer={
          <>
            <Button variant="ghost" onClick={() => setConfirmResetOpen(false)}>
              Cancelar
            </Button>
            <Button variant="danger" className="ml-auto" loading={saving} onClick={() => void doReset()}>
              Restaurar padrões
            </Button>
          </>
        }
      />
    </Card>
  );
}
