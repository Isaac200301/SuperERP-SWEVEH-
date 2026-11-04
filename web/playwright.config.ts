import { defineConfig, devices } from '@playwright/test';

/**
 * Recorrido en navegador real contra el entorno local (`make dev`: api en :8080 con OTP_SENDER=consola y Vite en
 * :5173). No corre en el CI todavía (docs/07). El código del registro se lee del log del api: SWEVEH_API_LOG.
 */
export default defineConfig({
  testDir: './e2e',
  timeout: 120_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  retries: 0,
  reporter: [['list']],
  use: {
    baseURL: process.env.SWEVEH_WEB_URL ?? 'http://localhost:5173',
    ...devices['Desktop Chrome'],
    locale: 'es-CO',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  outputDir: process.env.SWEVEH_E2E_OUT ?? 'test-results',
});
