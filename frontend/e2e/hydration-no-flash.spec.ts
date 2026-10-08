import { expect, test } from "@playwright/test";

import { ADMIN_PASSWORD, asAdmin, NEEDS_ADMIN, STORAGE_STATE, uniq } from "./helpers";

// Regressão de "flash na hidratação": a identidade da organização vem do
// backend (GET /branding) e as preferências do visitante (Sidebar
// recolhida, Alto Contraste) de cookies — o layout do servidor já
// renderiza tudo no estado certo, sem troca visível depois de hidratar.
// Salvar o branding expira o cache do servidor na hora (updateTag), então
// o HTML seguinte já traz o nome novo.
test.describe("Sem flash de hidratação (branding + Sidebar)", () => {
  test.skip(!ADMIN_PASSWORD, NEEDS_ADMIN);
  test.use({ storageState: STORAGE_STATE });

  test("o HTML do servidor já traz o nome novo, a Sidebar recolhida e o alto contraste", async ({ page, context }) => {
    await asAdmin(page);

    const newName = uniq("Portal E2E");
    await page.goto("/configuracao");
    const nameField = page.getByLabel("Nome da Aplicação / Sistema *");
    await expect(nameField).toBeEnabled();
    const previous = await nameField.inputValue();
    await nameField.fill(newName);
    await page.getByRole("button", { name: /Salvar alterações/ }).click();
    await expect(page.getByText("Configurações salvas")).toBeVisible();

    await page.goto("/dashboard");
    await page.getByRole("button", { name: "Alternar menu de navegação" }).click();
    await page.getByRole("button", { name: "Alto contraste" }).click();
    await expect
      .poll(async () => (await context.cookies()).map((c) => c.name))
      .toEqual(expect.arrayContaining(["nexus-branding", "nexus-sidebar-collapsed"]));

    // HTML CRU do servidor: o que chega ANTES do React hidratar.
    const html = await (await context.request.get("/dashboard")).text();
    expect(html, "SSR deve trazer o nome novo").toContain(newName);
    expect(html, "SSR deve trazer a Sidebar recolhida").toContain("md:w-[var(--sidebar-w-collapsed)]");
    expect(html).toContain("md:pl-[var(--sidebar-w-collapsed)]");
    expect(html, "SSR deve carimbar data-high-contrast no <html>").toMatch(/<html[^>]*data-high-contrast="true"/);

    // devolve o nome original (a suíte roda contra o mesmo banco)
    await page.goto("/configuracao");
    await page.getByLabel("Nome da Aplicação / Sistema *").fill(previous || "Projeto Nexus");
    await page.getByRole("button", { name: /Salvar alterações/ }).click();
    await expect(page.getByText("Configurações salvas")).toBeVisible();
  });
});
