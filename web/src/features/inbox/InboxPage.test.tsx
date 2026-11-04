import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { mockApi, owner, renderPage } from '../../test/api';
import { InboxPage } from './InboxPage';

afterEach(() => vi.unstubAllGlobals());

const borrador = {
  id: 'b1', conversacionId: 'c1', celular: '3001234567', mensajeCliente: 'Hola, ¿a cómo el cambio de aceite?', intencion: 'precio',
  propuesto: 'El cambio de aceite cuesta $45.000.', final: null, estado: 'pendiente', origen: 'ia', accion: null,
  creadoEn: '2026-11-04T15:00:00Z', resueltoEn: null,
  fuentes: [{ campo: 'precio', valor: '$45.000', origen: 'Servicio: Cambio de aceite' }],
};

describe('InboxPage (HU-62, HU-63, HU-64)', () => {
  it('muestra de dónde salió cada dato y envía el texto editado', async () => {
    const calls = mockApi({
      'GET /me': owner,
      'GET /inbox': { borradores: [borrador] },
      'GET /inbox/stats': { enviados: 0, editados: 0, porcentajeEditados: 0 },
      'GET /assistant/settings': { autoEnvio: false },
      'POST /inbox/{id}': { ok: true },
    });
    renderPage(<InboxPage />);
    expect(await screen.findByText(/a cómo el cambio de aceite/)).toBeInTheDocument();
    expect(screen.getByText('Precio: $45.000 · Servicio: Cambio de aceite')).toBeInTheDocument();
    expect(screen.queryByText(/object Object/)).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Probar WhatsApp como cliente/ })).toHaveAttribute('href', '/panel/whatsapp-prueba');

    const caja = screen.getByLabelText(/Respuesta propuesta/);
    await userEvent.clear(caja);
    await userEvent.type(caja, 'Cuesta $45.000 con filtro.');
    await userEvent.click(screen.getByRole('button', { name: 'Enviar' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ decision: 'enviar', texto: 'Cuesta $45.000 con filtro.' }));
  });

  it('ADR-0028: el dueño ve las respuestas automáticas y puede apagar el asistente', async () => {
    const auto = { ...borrador, id: 'b2', estado: 'auto_enviado', final: 'El cambio de aceite cuesta $45.000.' };
    const calls = mockApi({
      'GET /me': { ...owner, permisos: [...owner.permisos, 'automation:manage'] },
      'GET /inbox': (req: Request) => ({ status: 200, body: { borradores: new URL(req.url).search ? [auto] : [] } }),
      'GET /inbox/stats': { enviados: 0, editados: 0, porcentajeEditados: 0 },
      'GET /assistant/settings': { autoEnvio: true },
      'PUT /assistant/settings': { autoEnvio: false },
    });
    renderPage(<InboxPage />);
    const sw = await screen.findByRole('switch', { name: 'El asistente responde solo' });
    expect(sw).toBeChecked();
    await waitFor(() => expect(sw).toBeEnabled());
    expect(screen.getByText(/salen sin tu revisión/)).toBeInTheDocument();
    await userEvent.click(await screen.findByText('Últimas respuestas automáticas'));
    expect(screen.getByText(/cuesta \$45\.000\./)).toBeInTheDocument();

    await userEvent.click(sw);
    await waitFor(() => expect(calls.find((c) => c.method === 'PUT')?.body).toEqual({ autoEnvio: false }));
    expect(await screen.findByText(/cada respuesta espera aquí/)).toBeInTheDocument();
  });

  it('recepción ve el interruptor sin poder cambiarlo', async () => {
    mockApi({
      'GET /me': { ...owner, roles: ['recepcion'], permisos: ['order:read', 'inbox:manage'] },
      'GET /inbox': { borradores: [] },
      'GET /inbox/stats': { enviados: 0, editados: 0, porcentajeEditados: 0 },
      'GET /assistant/settings': { autoEnvio: true },
    });
    renderPage(<InboxPage />);
    expect(await screen.findByRole('switch', { name: 'El asistente responde solo' })).toBeDisabled();
    expect(screen.getByText(/Solo el dueño lo cambia/)).toBeInTheDocument();
  });
});
