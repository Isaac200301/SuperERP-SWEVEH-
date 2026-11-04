import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { mockApi, owner, renderPage } from '../../test/api';
import { WahaCard } from './WahaCard';

afterEach(() => vi.unstubAllGlobals());

describe('WahaCard (ADR-0027)', () => {
  it('no aparece si el servidor no tiene WAHA', async () => {
    const calls = mockApi({ 'GET /me': owner, 'GET /whatsapp/waha': { configurado: false }, 'GET /whatsapp/channel': { canal: null } });
    const { container } = renderPage(<WahaCard />);
    await waitFor(() => expect(calls.some((c) => c.path === '/whatsapp/waha')).toBe(true));
    expect(container).toBeEmptyDOMElement();
  });

  it('conecta el número, muestra el QR y pasa el taller a responder por WAHA', async () => {
    const calls = mockApi({
      'GET /me': owner,
      'GET /whatsapp/waha': { configurado: true, sesion: 'default', estado: 'NOT_FOUND' },
      'POST /whatsapp/waha/start': { configurado: true, sesion: 'default', estado: 'SCAN_QR_CODE', qr: 'data:image/png;base64,cG5n' },
      'GET /whatsapp/channel': { canal: { proveedor: 'prueba_local' } },
      'PUT /whatsapp/channel': { canal: { proveedor: 'waha' } },
    });
    renderPage(<WahaCard />);
    expect(await screen.findByText(/WhatsApp no oficial/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Conectar número de prueba' }));
    expect(await screen.findByAltText(/Código QR/)).toHaveAttribute('src', 'data:image/png;base64,cG5n');
    expect(calls.some((c) => c.method === 'POST' && c.path === '/whatsapp/waha/start')).toBe(true);
  });

  it('con la sesión conectada deja usar el número y volver al simulador', async () => {
    let puts = 0;
    const calls = mockApi({
      'GET /me': owner,
      'GET /whatsapp/waha': { configurado: true, sesion: 'default', estado: 'WORKING', numero: '573009999999@c.us' },
      'GET /whatsapp/channel': { canal: { proveedor: 'prueba_local' } },
      'PUT /whatsapp/channel': () => ({ canal: { proveedor: puts++ === 0 ? 'waha' : 'prueba_local' } }),
    });
    renderPage(<WahaCard />);
    expect(await screen.findByText(/\+57 300 999 9999/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Responder por este número' }));
    expect(await screen.findByText(/respondiendo por el WhatsApp de prueba/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Volver al simulador' }));
    await waitFor(() => expect(calls.filter((c) => c.method === 'PUT').map((c) => c.body)).toEqual([{ proveedor: 'waha' }, { proveedor: 'prueba_local' }]));
  });
});
