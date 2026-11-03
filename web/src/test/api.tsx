import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render } from '@testing-library/react';
import type { ReactElement } from 'react';
import { MemoryRouter } from 'react-router';
import { vi } from 'vitest';

type Handler = (req: Request) => unknown;

export type Call = { method: string; path: string; body: unknown };

/**
 * Simula la API por método y ruta ("GET /orders/mine"). Lo que devuelve el handler va como JSON 200;
 * un objeto { status, body } permite otros códigos. Guarda las llamadas para revisar lo que se envió.
 */
export function mockApi(routes: Record<string, unknown>) {
  const calls: Call[] = [];
  vi.stubGlobal('fetch', async (req: Request) => {
    const url = new URL(req.url);
    const path = url.pathname.replace('/api/v1', '');
    const text = req.method === 'GET' ? '' : await req.text();
    let body: unknown = text;
    try {
      body = text ? JSON.parse(text) : undefined;
    } catch {
      // multipart: se deja como texto
    }
    calls.push({ method: req.method, path: path + url.search, body });
    const key = Object.keys(routes).find((k) => {
      const [m, p] = k.split(' ');
      return m === req.method && new RegExp(`^${(p ?? '').replace(/\{[^}]+\}/g, '[^/]+')}$`).test(path);
    });
    if (!key) return new Response(JSON.stringify({ detail: `sin mock: ${req.method} ${path}` }), { status: 500 });
    const value = routes[key];
    const out = typeof value === 'function' ? (value as Handler)(req) : value;
    const r = out as { status?: number; body?: unknown } | undefined;
    if (r && typeof r === 'object' && 'status' in r && typeof r.status === 'number') {
      return new Response(r.body === undefined ? null : JSON.stringify(r.body), { status: r.status, headers: { 'Content-Type': 'application/json' } });
    }
    return new Response(JSON.stringify(out), { status: 200, headers: { 'Content-Type': 'application/json' } });
  });
  return calls;
}

export function renderPage(ui: ReactElement, path = '/') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[path]}>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

export const owner = {
  usuario: { id: 'u1', nombre: 'Rosa', celular: '3001234567' },
  taller: { id: 't1', nombre: 'Servitaller El Pistón' },
  roles: ['dueno'],
  cuentaNueva: false,
  permisos: ['order:read', 'order:update_state', 'order:override_approval', 'quote:register_response', 'service:manage', 'user:manage', 'workshop:manage'],
};

export const mechanic = { ...owner, roles: ['mecanico'], permisos: ['order:read', 'order:update_state'] };

export function orden(over: Record<string, unknown> = {}) {
  return {
    id: 'o1', numero: 7, codigo: 'OS-0007', placa: 'KLM482', estado: 'en_diagnostico', sintoma: 'Suena al frenar', version: 3,
    creadoEn: '2026-11-02T15:00:00Z', mecanicoId: null, fechaPrometida: null, cerradaEn: null, ejemplo: false,
    vehiculo: { id: 'v1', placa: 'KLM482', tipo: 'carro', marca: 'Chevrolet', linea: 'Spark', modeloAnio: 2015, motorPendiente: false },
    cliente: { id: 'c1', nombre: 'Laura Gómez' }, historial: [], ...over,
  };
}
