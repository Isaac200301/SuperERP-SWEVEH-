import { screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { formatoCelular } from '../../shared/lib/format';
import { mockApi, owner, renderPage } from '../../test/api';
import { WhatsappDown } from './WhatsappDown';

afterEach(() => vi.unstubAllGlobals());

describe('WhatsappDown', () => {
  it('avisa en el panel cuando WhatsApp cerró la sesión del número que usa el taller', async () => {
    mockApi({
      'GET /me': owner,
      'GET /whatsapp/channel': { canal: { proveedor: 'waha' } },
      'GET /whatsapp/waha': { configurado: true, sesion: 't-1', estado: 'FAILED' },
    });
    renderPage(<WhatsappDown enabled />);
    expect(await screen.findByText(/WhatsApp desconectado/)).toBeInTheDocument();
    expect(screen.getByText(/cerró la sesión/)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Reconectar en Mi taller' })).toHaveAttribute('href', '/panel/taller');
  });

  it('no avisa con la sesión conectada ni si el taller usa el simulador', async () => {
    const calls = mockApi({
      'GET /me': owner,
      'GET /whatsapp/channel': { canal: { proveedor: 'waha' } },
      'GET /whatsapp/waha': { configurado: true, sesion: 't-1', estado: 'WORKING' },
    });
    const { container } = renderPage(<WhatsappDown enabled />);
    await waitFor(() => expect(calls.some((c) => c.path === '/whatsapp/waha')).toBe(true));
    expect(container).toBeEmptyDOMElement();

    const local = mockApi({ 'GET /me': owner, 'GET /whatsapp/channel': { canal: { proveedor: 'prueba_local' } } });
    const otro = renderPage(<WhatsappDown enabled />);
    await waitFor(() => expect(local.some((c) => c.path === '/whatsapp/channel')).toBe(true));
    expect(local.some((c) => c.path === '/whatsapp/waha')).toBe(false);
    expect(otro.container).toBeEmptyDOMElement();
  });

  it('un chat con el número oculto por WhatsApp no muestra el id interno', () => {
    expect(formatoCelular('123456789012345@lid')).toBe('Número oculto por WhatsApp');
    expect(formatoCelular('+573177251924')).toBe('317 725 1924');
  });
});
