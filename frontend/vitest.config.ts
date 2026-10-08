import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "node:path";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    setupFiles: ["./vitest.setup.ts"],
    globals: true,
    // e2e/ é Playwright, não Vitest (fixtures/APIs incompatíveis — page,
    // expect, etc. vêm de @playwright/test, não daqui) — sem o exclude,
    // o Vitest tenta rodar esses arquivos como se fossem seus e falha
    // com um erro de import, achado real adicionando o Playwright.
    exclude: ["node_modules", ".next", "e2e"],
    // Meta de cobertura do frontend (ADR 010): o CI roda com --coverage e
    // falha se cair abaixo destes pisos. Suba-os junto com novos testes.
    coverage: {
      provider: "v8",
      include: ["src/**/*.{ts,tsx}"],
      exclude: ["src/test/**", "src/**/*.test.{ts,tsx}", "src/**/*.d.ts", "src/types/**"],
      reporter: ["text-summary", "json-summary"],
      thresholds: { statements: 95, branches: 88, functions: 94, lines: 96 },
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
});
