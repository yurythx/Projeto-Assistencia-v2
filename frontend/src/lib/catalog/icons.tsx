import {
  Baby,
  BookOpen,
  Briefcase,
  Building2,
  FileText,
  HandHeart,
  HeartPulse,
  Home,
  Info,
  LayoutGrid,
  Scale,
  Users,
  type LucideProps,
} from "lucide-react";
import type { ComponentType } from "react";

// Ícone de um serviço do catálogo (campo "icon", nome lucide livre
// escolhido pelo gestor). Nome desconhecido cai no ícone genérico.
const ICONS: Record<string, ComponentType<LucideProps>> = {
  baby: Baby,
  "book-open": BookOpen,
  briefcase: Briefcase,
  building: Building2,
  "file-text": FileText,
  "hand-heart": HandHeart,
  "heart-pulse": HeartPulse,
  home: Home,
  info: Info,
  scale: Scale,
  users: Users,
};

export const SERVICE_ICON_NAMES = Object.keys(ICONS);

export function ServiceIcon({ name, ...props }: { name?: string } & LucideProps) {
  const Icon = (name && ICONS[name]) || LayoutGrid;
  return <Icon {...props} />;
}
