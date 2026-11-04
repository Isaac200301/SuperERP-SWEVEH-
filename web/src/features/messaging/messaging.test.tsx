import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Orden } from '../../shared/api/client';
import { mockApi, orden, owner, renderPage } from '../../test/api';
import { OrderMessages } from '../orders/OrderDetailPage';
import { SimulatorPage } from './SimulatorPage';
import { quien, Thread, type Mensaje } from './Thread';

afterEach(() => vi.unstubAllGlobals());

const msg = (over: Partial<Mensaje>): Mensaje => ({
  id: Math.random().toString(36), direccion: 'saliente', tipo: 'texto', cuerpo: 'Hola', estado: 'enviado', origen: 'automatizacion',
  creadoEn: '2026-11-04T15:00:00Z', automatizacion: null, payload: { botones: [] }, ...over,
}) as Mensaje;

describe('Thread (HU-24)', () => {
  it('dice quién envió cada mensaje y marca los que no se entregaron', () => {
    expect(quien(msg({ direccion: 'entrante', origen: 'cliente' }))).toBe('Cliente');
    expect(quien(msg({ origen: 'automatizacion', automatizacion: 'AU-01' }))).toBe('Automático · AU-01');
    expect(quien(msg({ origen: 'agente', automatizacion: 'consentimiento' }))).toBe('Autorización de datos');
    expect(quien(msg({ origen: 'agente', automatizacion: 'AU-04' }))).toBe('Asistente');
    expect(quien(msg({ origen: 'persona' }))).toBe('Taller');
    renderPage(<Thread mensajes={[msg({ cuerpo: 'Recibimos tu carro', estado: 'fallido', error: 'whatsapp: respuesta 400 (código 131026)' })]} />);
    expect(screen.getByText('Recibimos tu carro')).toBeInTheDocument();
    expect(screen.getByText(/No se entregó/)).toBeInTheDocument();
    expect(screen.getByText(/código 131026/)).toBeInTheDocument();
  });
});

describe('OrderMessages (HU-24)', () => {
  it('muestra la conversación de la orden con sus botones', async () => {
    mockApi({
      'GET /orders/{id}/messages': {
        mensajes: [
          msg({ direccion: 'entrante', origen: 'cliente', cuerpo: '¿Ya está mi carro?' }),
          msg({ cuerpo: 'Esta es la cotización', automatizacion: 'AU-02', payload: { botones: [{ id: 'quote:aprobar:o1', titulo: 'Aprobar' }] } }),
        ],
      },
    });
    renderPage(<OrderMessages orden={orden() as unknown as Orden} />);
    expect(await screen.findByText('¿Ya está mi carro?')).toBeInTheDocument();
    expect(screen.getByText('Aprobar')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Aprobar' })).not.toBeInTheDocument(); // en la orden no se pulsan
  });
});

describe('SimulatorPage', () => {
  it('escribe como cliente y pulsa los botones que le llegan', async () => {
    const calls = mockApi({
      'GET /me': owner,
      'GET /conversations': { conversaciones: [{ id: 'c1', celular: '3001234567', modo: 'agente', pideHumano: false, clienteId: null, ventanaAbiertaHasta: null }] },
      'GET /conversations/{id}': {
        conversacion: { id: 'c1', celular: '3001234567', modo: 'agente', pideHumano: false, clienteId: null, ventanaAbiertaHasta: null },
        mensajes: [msg({ origen: 'agente', automatizacion: 'consentimiento', cuerpo: '¿Aceptas el tratamiento de tus datos?', tipo: 'botones',
          payload: { botones: [{ id: 'consent:si', titulo: 'Acepto' }, { id: 'consent:no', titulo: 'No acepto' }] } })],
      },
      'POST /simulator/inbound': { status: 202, body: { ok: true } },
    });
    renderPage(<SimulatorPage />);
    await userEvent.click(await screen.findByRole('button', { name: 'Acepto' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ celular: '3001234567', nombre: 'Cliente de prueba', texto: 'Acepto', boton: 'consent:si' }));
    await userEvent.type(screen.getByLabelText('Mensaje'), '¿A cómo la alineación?');
    await userEvent.click(screen.getByRole('button', { name: 'Enviar' }));
    await waitFor(() => expect(calls.filter((c) => c.method === 'POST').at(-1)?.body).toEqual({ celular: '3001234567', nombre: 'Cliente de prueba', texto: '¿A cómo la alineación?' }));
  });

  it('no deja escribir con un celular inválido', async () => {
    mockApi({ 'GET /me': owner });
    renderPage(<SimulatorPage />);
    const cel = screen.getByLabelText('Celular del cliente');
    await userEvent.clear(cel);
    await userEvent.type(cel, '12345');
    expect(screen.getByText(/10 dígitos/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Enviar' })).toBeDisabled();
  });
});
