// Chaves dos módulos no Kernel (internal/platform/kernel — Manifest.Key).
export const MODULE = {
  iam: "iam",
  audit: "audit",
  mercurio: "mercurio",
  egress: "egress",
  blog: "blog",
  catalog: "catalog",
  contact: "contact",
  directory: "directory",
  calendar: "calendar",
  files: "files",
  wiki: "wiki",
  search: "search",
  signum: "signum",
  tramite: "tramite",
  example: "example",
} as const;

export type ModuleKey = (typeof MODULE)[keyof typeof MODULE];
