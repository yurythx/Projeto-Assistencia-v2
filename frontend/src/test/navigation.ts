// Mock de next/navigation compartilhado pelos testes de tela:
//   vi.mock("next/navigation", async () => (await import("@/test/navigation")).navigationModule);
import { vi } from "vitest";

export const router = {
  push: vi.fn(),
  replace: vi.fn(),
  refresh: vi.fn(),
  back: vi.fn(),
  prefetch: vi.fn(),
};

export const nav = {
  params: {} as Record<string, string>,
  pathname: "/",
  search: new URLSearchParams(),
};

/** Zera o estado entre testes. */
export function resetNavigation(params: Record<string, string> = {}, pathname = "/", search = "") {
  Object.values(router).forEach((f) => f.mockReset());
  nav.params = params;
  nav.pathname = pathname;
  nav.search = new URLSearchParams(search);
}

export const navigationModule = {
  useRouter: () => router,
  useParams: () => nav.params,
  usePathname: () => nav.pathname,
  useSearchParams: () => nav.search,
  redirect: vi.fn(),
  notFound: vi.fn(),
};
