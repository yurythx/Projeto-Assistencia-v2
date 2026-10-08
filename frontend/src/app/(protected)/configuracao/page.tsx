import { BrandingSettingsForm } from "@/components/settings/BrandingSettingsForm";

export default function SistemaIdentidadePage() {
  return (
    <div className="flex flex-col gap-6">
      {/* Identidade visual white-label (GET/PUT /branding) */}
      <BrandingSettingsForm />
    </div>
  );
}
