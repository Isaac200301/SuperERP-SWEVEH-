import { defineConfig, devices } from '@playwright/test';

/**
 * Recorrido en navegador real. `make e2e` (y el job e2e del CI) compila la web, levanta el api sirviendo la SPA con
 * OTP_SENDER=consola y define SWEVEH_WEB_URL y SWEVEH_API_LOG (scripts/e2e.sh). Contra `make dev` también sirve:
 * Vite en :5173 y SWEVEH_API_LOG apuntando al log de `go run ./cmd/api`.
 */
export default defineConfig({
  testDir: './e2e',
  timeout: 120_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  // Una a la vez: el código de entrada se toma del log del api por los últimos dígitos del celular.
  workers: 1,
  retries: 0,
  reporter: [['list']],
  use: {
    baseURL: process.env.SWEVEH_WEB_URL ?? 'http://localhost:5173',
    locale: 'es-CO',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  // El recorrido de escritorio y el del celular (mockups móviles v1, CA-11) van en proyectos separados.
  projects: [
    { name: 'escritorio', use: { ...devices['Desktop Chrome'] }, testIgnore: /movil\.spec\.ts/ },
    { name: 'movil', use: { ...devices['Pixel 7'] }, testMatch: /movil\.spec\.ts/ },
  ],
  outputDir: process.env.SWEVEH_E2E_OUT ?? 'test-results',
});
