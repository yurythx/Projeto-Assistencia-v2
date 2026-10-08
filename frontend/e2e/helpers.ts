import { expect, type Page } from "@playwright/test";

// Conta criada por `make seed-admin` (senha aleatória a cada execução).
export const ADMIN_USERNAME = process.env.E2E_ADMIN_USERNAME ?? "admin";
export const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD;
export const NEEDS_ADMIN =
  "E2E_ADMIN_PASSWORD não definida — rode `make seed-admin` e exporte a senha impressa (ver docs/e2e.md).";

/** Aceita o termo LGPD sempre que o modal aparecer (ele abre com atraso e
 * cobriria o que o teste está clicando). */
export async function autoAcceptConsent(page: Page) {
  await page.addLocatorHandler(page.getByRole("button", { name: "Concordar e Continuar" }), async (button) => {
    await button.click();
  });
}

/** Login local pela tela real; termina no dashboard. */
export async function login(page: Page) {
  await autoAcceptConsent(page);
  await page.goto("/login");
  await page.getByLabel("Usuário").fill(ADMIN_USERNAME);
  await page.getByLabel("Senha", { exact: true }).fill(ADMIN_PASSWORD!);
  await page.getByRole("button", { name: "Entrar", exact: true }).click();
  await expect(page).toHaveURL(/\/dashboard(\?.*)?$/);
}

/** Sessão do administrador gravada pelo projeto "setup" (auth.setup.ts). */
export const STORAGE_STATE = "e2e/.auth/admin.json";

/** Já autenticado (storageState): abre uma página da área logada. */
export async function asAdmin(page: Page, path = "/dashboard") {
  await autoAcceptConsent(page);
  await page.goto(path);
  await expect(page).not.toHaveURL(/\/login/);
}

/** Sufixo único por execução (os testes rodam contra o mesmo banco). */
export function uniq(prefix: string) {
  return `${prefix} ${Date.now().toString(36)}${Math.random().toString(36).slice(2, 5)}`;
}
