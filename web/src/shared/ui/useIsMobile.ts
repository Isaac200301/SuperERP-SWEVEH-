import { useSyncExternalStore } from 'react';

/** Por debajo de este ancho el panel usa la versión de celular (mockups móviles v1, docs/10 §4.7). */
export const MOBILE_QUERY = '(max-width: 767px)';

function query(): MediaQueryList | null {
  return typeof window !== 'undefined' && typeof window.matchMedia === 'function' ? window.matchMedia(MOBILE_QUERY) : null;
}

function subscribe(onChange: () => void): () => void {
  const mq = query();
  if (!mq) return () => undefined;
  mq.addEventListener('change', onChange);
  return () => mq.removeEventListener('change', onChange);
}

/**
 * ¿La pantalla es de celular? Se decide con JS (no con clases md:hidden) para que solo una de las dos navegaciones
 * exista en el DOM. Sin matchMedia (jsdom en las pruebas) gana el escritorio.
 */
export function useIsMobile(): boolean {
  return useSyncExternalStore(
    subscribe,
    () => query()?.matches ?? false,
    () => false,
  );
}
