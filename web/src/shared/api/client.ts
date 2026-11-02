import createClient from 'openapi-fetch';

import type { components, paths } from './schema';

export type Schemas = components['schemas'];
export type Orden = Schemas['Orden'];
export type Vehiculo = Schemas['Vehiculo'];
export type Cliente = Schemas['Cliente'];
export type Sesion = Schemas['Sesion'];
export type EstadoOrden = Schemas['EstadoOrden'];
export type Problem = Schemas['Problem'];

/** Error de la API con el código estable del backend (RFC 9457). */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly field: string | undefined;

  constructor(status: number, problem: Partial<Problem> | undefined) {
    super(problem?.detail ?? 'Algo falló de nuestro lado. Intenta de nuevo en un momento.');
    this.name = 'ApiError';
    this.status = status;
    this.code = problem?.code ?? 'error_desconocido';
    this.field = problem?.field;
  }
}

const AUTH_PREFIX = '/api/v1/auth/';

let refreshing: Promise<boolean> | null = null;

/** Un solo refresh a la vez aunque fallen varias solicitudes juntas (el servidor rota el token). */
function refreshOnce(): Promise<boolean> {
  refreshing ??= fetch('/api/v1/auth/refresh', {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'X-Requested-With': 'sweveh' },
  })
    .then((r) => r.ok)
    .catch(() => false)
    .finally(() => {
      refreshing = null;
    });
  return refreshing;
}

/**
 * fetch con sesión por cookies HttpOnly: el JS nunca ve el token (docs/06 §6.1).
 * Ante un 401 intenta un refresh y repite la solicitud una vez.
 */
async function sessionFetch(input: Request): Promise<Response> {
  const retry = input.clone();
  const res = await fetch(input);
  if (res.status !== 401 || new URL(input.url).pathname.startsWith(AUTH_PREFIX)) {
    return res;
  }
  if (!(await refreshOnce())) {
    window.dispatchEvent(new Event('sweveh:sesion-terminada'));
    return res;
  }
  return fetch(retry);
}

export const api = createClient<paths>({
  baseUrl: '/api/v1',
  credentials: 'same-origin',
  headers: { 'X-Requested-With': 'sweveh' },
  fetch: sessionFetch,
});

/** Convierte la respuesta de openapi-fetch en datos o en ApiError. */
export function unwrap<T>(r: { data?: T; error?: unknown; response: Response }): T {
  if (r.error !== undefined || r.data === undefined) {
    throw new ApiError(r.response.status, r.error as Partial<Problem> | undefined);
  }
  return r.data;
}

/** Igual que unwrap, pero también devuelve el ETag para editar con If-Match (R-07). */
export function unwrapWithETag<T>(r: { data?: T; error?: unknown; response: Response }): { data: T; etag: string } {
  return { data: unwrap(r), etag: r.response.headers.get('ETag') ?? '' };
}
