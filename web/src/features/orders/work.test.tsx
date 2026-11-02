import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { renderHook, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { useAddEvidence, useQuote } from './work';

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

afterEach(() => vi.unstubAllGlobals());

describe('useAddEvidence (HU-21)', () => {
  it('envía multipart con la foto al final y sin Content-Type JSON', async () => {
    const fetchMock = vi.fn<(req: Request) => Promise<Response>>(() => Promise.resolve(json({ id: 'e1' }, 201)));
    vi.stubGlobal('fetch', fetchMock);
    const { result } = renderHook(() => useAddEvidence('o1'), { wrapper });
    const foto = new File([new Uint8Array([0xff, 0xd8, 0xff])], 'f.jpg', { type: 'image/jpeg' });
    result.current.mutate({ lado: 'frente', foto, mostrarCliente: true });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const [req] = fetchMock.mock.calls[0] ?? [];
    if (!req) throw new Error('no se llamó a fetch');
    expect(req.url).toContain('/api/v1/orders/o1/evidence');
    expect(req.headers.get('Content-Type')).toMatch(/^multipart\/form-data; boundary=/);
    const body = await req.text();
    const at = (k: string) => body.indexOf(`name="${k}"`);
    expect(at('lado')).toBeGreaterThan(-1);
    expect(at('lado')).toBeLessThan(at('mostrarCliente'));
    expect(at('mostrarCliente')).toBeLessThan(at('foto'));
  });
});

describe('useQuote (HU-27)', () => {
  it('devuelve null si la orden aún no tiene cotización', async () => {
    vi.stubGlobal('fetch', () => Promise.resolve(json({ cotizacion: null })));
    const { result } = renderHook(() => useQuote('o1'), { wrapper });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toBeNull();
  });
});
