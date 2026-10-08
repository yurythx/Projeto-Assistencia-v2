import { expect, test } from "@playwright/test";

// Barra de acessibilidade DSGov / e-MAG (skill §3): atalhos Alt+1..4 para
// #main-content, #main-menu, #global-search e #gov-footer; Alto Contraste
// e escala de fonte persistidos.
test.describe("Acessibilidade e-MAG / WCAG 2.1 AA", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/");
  });

  test("links de salto existem e aparecem ao receber foco", async ({ page }) => {
    const skip = page.getByRole("link", { name: /Ir para o conteúdo/ });
    await expect(skip).toHaveAttribute("href", "#main-content");
    await expect(skip).toHaveAttribute("accesskey", "1");
    await page.keyboard.press("Tab");
    await expect(skip).toBeFocused();
    await expect(skip).toBeVisible();
  });

  test("Alt+1, Alt+2 e Alt+4 movem o foco para as âncoras e-MAG", async ({ page }) => {
    await page.keyboard.press("Alt+1");
    await expect(page.locator("#main-content")).toBeFocused();

    await page.keyboard.press("Alt+2");
    await expect(page.locator("#main-menu")).toBeFocused();

    await page.keyboard.press("Alt+4");
    await expect(page.locator("#gov-footer")).toBeFocused();
  });

  test("Alto Contraste marca o documento e o botão fica pressionado", async ({ page }) => {
    const contrast = page.getByRole("button", { name: /Alto contraste/i });
    await contrast.click();
    await expect(contrast).toHaveAttribute("aria-pressed", "true");
    await expect(page.locator("html")).toHaveAttribute("data-high-contrast", "true");
  });

  test("Controle de fonte A+ aumenta a escala", async ({ page }) => {
    await page.getByRole("button", { name: "Aumentar fonte" }).click();
    await expect(page.locator("html")).not.toHaveAttribute("data-font-scale", "100");
  });

  test("Modal de Consentimento LGPD é um diálogo modal acessível", async ({ page }) => {
    await page.evaluate(() => localStorage.removeItem("nexus_lgpd_consent"));
    await page.reload();

    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText("Termos de Privacidade & Proteção de Dados (LGPD)")).toBeVisible();
  });
});
