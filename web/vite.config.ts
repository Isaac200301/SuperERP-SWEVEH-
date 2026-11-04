import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';
import { VitePWA } from 'vite-plugin-pwa';
import { configDefaults } from 'vitest/config';

// Piso instalable en el celular del mecánico (HU-23, docs/06 §790). El service worker solo guarda los
// archivos del build: nunca /api, que son respuestas con sesión. La cola de fotos sin conexión (AQ-05) llega después.
const pwa = VitePWA({
  registerType: 'autoUpdate',
  injectRegister: 'script', // registerSW.js como archivo aparte: la CSP no permite scripts en línea
  includeAssets: ['logo.svg', 'apple-touch-icon.png'],
  manifest: {
    name: 'SWEVEH · Piso del taller',
    short_name: 'SWEVEH',
    description: 'Tus órdenes del taller en el celular.',
    lang: 'es-CO',
    start_url: '/panel/piso',
    scope: '/',
    display: 'standalone',
    orientation: 'portrait',
    theme_color: '#005F54',
    background_color: '#F4F7F6',
    icons: [
      { src: 'pwa-192.png', sizes: '192x192', type: 'image/png' },
      { src: 'pwa-512.png', sizes: '512x512', type: 'image/png' },
      { src: 'pwa-maskable-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
    ],
  },
  workbox: {
    globPatterns: ['**/*.{js,css,html,svg,png,woff2}'],
    navigateFallback: '/index.html',
    navigateFallbackDenylist: [/^\/api\//],
    runtimeCaching: [],
    cleanupOutdatedCaches: true,
  },
});

// En desarrollo, Vite reenvía /api al backend en Go: el navegador ve un solo origen,
// como en producción (ADR-18), y las cookies HttpOnly funcionan igual.
export default defineConfig({
  plugins: [react(), tailwindcss(), pwa],
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
