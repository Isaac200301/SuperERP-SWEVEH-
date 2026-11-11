import { vi } from 'vitest';

/** Simula un celular: matchMedia responde que la pantalla es angosta (useIsMobile). Se limpia con vi.unstubAllGlobals. */
export function mockMobile(on = true) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: on,
    media: query,
    onchange: null,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    addListener: () => undefined,
    removeListener: () => undefined,
    dispatchEvent: () => false,
  }));
}
