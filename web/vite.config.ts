import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';
import { configDefaults } from 'vitest/config';

// En desarrollo, Vite reenvía /api al backend en Go: el navegador ve un solo origen,
// como en producción (ADR-18), y las cookies HttpOnly funcionan igual.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': { target: process.env.SWEVEH_API_URL ?? 'http://localhost:8080', changeOrigin: false },
    },
  },
  build: {
    sourcemap: false,
    target: 'es2022',
    // Sin recursos incrustados como data: (fuentes pequeñas): la CSP queda en font-src 'self'.
    assetsInlineLimit: 0,
  },
  test: {
    environment: 'jsdom',
    exclude: [...configDefaults.exclude, 'e2e/**'], // el recorrido en navegador lo corre Playwright (npm run e2e)
    setupFiles: ['./src/test/setup.ts'],
    css: false,
    coverage: { provider: 'v8', reporter: ['text', 'lcov'], include: ['src/**/*.{ts,tsx}'], exclude: ['src/shared/api/schema.d.ts', 'src/test/**'] },
  },
});
