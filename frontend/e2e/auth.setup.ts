import { test as setup } from "@playwright/test";

import { ADMIN_PASSWORD, login, STORAGE_STATE } from "./helpers";

// Login único da suíte; os testes reaproveitam a sessão (storageState).
setup("login do administrador", async ({ page }) => {
  setup.skip(!ADMIN_PASSWORD, "sem E2E_ADMIN_PASSWORD");
  await login(page);
  await page.context().storageState({ path: STORAGE_STATE });
});
