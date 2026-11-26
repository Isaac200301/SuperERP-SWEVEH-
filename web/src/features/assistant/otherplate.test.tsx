import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { mockApi, owner, renderPage } from '../../test/api';
import { AssistantPage } from './AssistantPage';
import { OrderAssistantCard } from './OrderAssistantCard';

// Prueba de Pablo (26-nov): en el chat de la moto OS-0041 se mostró un Toyota. El código arma tarjetas y la persona
// decide: una para el otro vehículo (con sus datos y fotos) y otra para guardar. Datos Ficticio.

const otra = {
  placa: { valor: 'TPR418', origen: 'video' },
  tipo: 'carro',
  nuevaOrden: true,
  vehiculoNuevo: true,
  ficha: { marca: { valor: 'Toyota', origen: 'dijiste' }, linea: { valor: 'Prado', origen: 'dijiste' } },
  cliente: { nombre: { valor: 'Pedro Ríos', origen: 'dijiste' }, celular: { valor: '3001234567', origen: 'dijiste' } },
  sintoma: { valor: 'facelift', origen: 'dijiste' },
  fotos: [
    { ref: 't1/f1', origen: 'video' },
    { ref: 't1/f2', origen: 'video' },
  ],
};

const base = (over: Record<string, unknown> = {}) => ({
  id: 'c41',
  ordenId: 'o41',
  version: 5,
  propuesta: { placa: { valor: 'XVN43F', origen: 'taller' }, tipo: 'moto', ordenId: 'o41', ordenCodigo: 'OS-0041', ordenEstado: 'por_recibir', otraOrden: otra },
  pendiente: false,
  progreso: { tiene: 1, total: 8, falta: ['la placa'] },
  otraFalta: [],
  disponible: true,
  voz: false,
  mensajes: [
    { id: 'u1', rol: 'usuario', texto: '', adjuntos: [{ tipo: 'video', segundos: 43 }], opciones: [], creadoEn: '2026-11-26T13:10:00Z' },
    { id: 'a1', rol: 'asistente', texto: 'En los cuadros se ve la placa TPR-418.', adjuntos: [], opciones: [], creadoEn: '2026-11-26T13:10:01Z' },
  ],
  ...over,
});

describe('Tarjeta del otro vehículo', () => {
  it('muestra los datos y las fotos del otro vehículo y lo abre en una conversación nueva con una clave', async () => {
    let key = '';
    const calls = mockApi({
      'GET /orders/{id}/assistant': { chat: base(), disponible: true, voz: false },
      'POST /assistant/floor/chats/{id}/other-order': (req: Request) => {
        key = req.headers.get('Idempotency-Key') ?? '';
        return { chat: base({ id: 'c99', ordenId: null, propuesta: otra, pendiente: true }) };
      },
    });
    renderPage(<OrderAssistantCard ordenId="o41" estado="recibido" />);
    const t = await screen.findByRole('region', { name: 'Otro vehículo: TPR418' });
    expect(t).toHaveTextContent('¿Crear una orden nueva para TPR418?');
    expect(t).toHaveTextContent('Esta conversación es de la orden OS-0041 (XVN43F): aquí no se guarda nada de TPR418.');
    expect(t).toHaveTextContent('Pedro Ríos');
    expect(t).toHaveTextContent('Toyota Prado');
    expect(t).toHaveTextContent('facelift');
    expect(within(t).getAllByRole('img', { name: 'Foto de TPR418' })[0]).toHaveAttribute('src', '/api/v1/assistant/floor/chats/c41/photos/f1');
    expect(screen.queryByRole('region', { name: 'Guardar' })).not.toBeInTheDocument(); // la moto no tiene nada por guardar
    await userEvent.click(within(t).getByRole('button', { name: 'Abrir orden nueva con estos datos' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'POST')).toBeDefined());
    expect(key).toMatch(/^[0-9a-f-]{36}$/);
    expect((calls.find((c) => c.method === 'POST')?.body as { version: number }).version).toBe(5);
  });

  it('"No, sigo aquí" quita la tarjeta (sus fotos vuelven a esta orden en el servidor)', async () => {
    const calls = mockApi({
      'GET /orders/{id}/assistant': { chat: base(), disponible: true, voz: false },
      'PATCH /assistant/floor/chats/{id}': { chat: base({ version: 6, propuesta: { ...base().propuesta, otraOrden: undefined } }) },
    });
    renderPage(<OrderAssistantCard ordenId="o41" estado="recibido" />);
    const t = await screen.findByRole('region', { name: 'Otro vehículo: TPR418' });
    await userEvent.click(within(t).getByRole('button', { name: 'No, sigo aquí' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'PATCH')).toBeDefined());
    const body = calls.find((c) => c.method === 'PATCH')?.body as { version: number; propuesta: Record<string, unknown> };
    expect(body.version).toBe(5);
    expect(body.propuesta).not.toHaveProperty('otraOrden');
    expect(body.propuesta['ordenCodigo']).toBe('OS-0041');
  });

  it('si el otro vehículo ya tiene orden abierta, lo dice y ofrece seguir en ella', async () => {
    const abierta = { ...otra, nuevaOrden: false, vehiculoNuevo: false, ordenId: 'o50', ordenCodigo: 'OS-0050' };
    mockApi({ 'GET /orders/{id}/assistant': { chat: base({ propuesta: { ...base().propuesta, otraOrden: abierta } }), disponible: true, voz: false } });
    renderPage(<OrderAssistantCard ordenId="o41" estado="recibido" />);
    const t = await screen.findByRole('region', { name: 'Otro vehículo: TPR418' });
    expect(t).toHaveTextContent('TPR418 ya tiene la orden OS-0050 abierta');
    expect(within(t).getByRole('button', { name: 'Seguir en OS-0050 con estos datos' })).toBeInTheDocument();
  });
});

describe('Cuadro de guardar', () => {
  it('sale solo con algo pendiente, dice "orden nueva" y comparte la clave con la ficha', async () => {
    const nueva = base({
      ordenId: null,
      pendiente: true,
      propuesta: { placa: { valor: 'TPR418', origen: 'dijiste' }, nuevaOrden: true, sintoma: { valor: 'facelift', origen: 'dijiste' } },
      progreso: { tiene: 3, total: 8, falta: ['el kilometraje'] },
      mensajes: [],
    });
    const keys: string[] = [];
    const listo = base({ ordenId: 'o42', pendiente: false, propuesta: { placa: { valor: 'TPR418', origen: 'dijiste' }, ordenId: 'o42', ordenCodigo: 'OS-0042' } });
    mockApi({
      'GET /me': owner,
      'GET /assistant/floor/latest': { chat: nueva, disponible: true, voz: false },
      'POST /assistant/floor/chats/{id}/confirm': (req: Request) => {
        keys.push(req.headers.get('Idempotency-Key') ?? '');
        return { resultado: { ordenId: 'o42', ordenCodigo: 'OS-0042', ordenEstado: 'por_recibir', ordenNueva: true, hecho: ['la orden OS-0042'], fotos: 0 }, chat: listo };
      },
    });
    renderPage(<AssistantPage />);
    const guardar = await screen.findByRole('region', { name: 'Guardar' });
    expect(guardar).toHaveTextContent('¿Guardar la orden nueva para TPR418?');
    expect(guardar).toHaveTextContent('Todavía falta el kilometraje; lo puedes agregar después.');
    await userEvent.click(within(guardar).getByRole('button', { name: 'Guardar orden' }));
    expect(await screen.findByText('Quedó guardado: la orden OS-0042.')).toBeInTheDocument();
    expect(keys).toHaveLength(1);
    expect(screen.queryByRole('region', { name: 'Guardar' })).not.toBeInTheDocument(); // ya no hay nada pendiente
  });

  it('en Por recibir avisa que los hallazgos se guardan cuando la orden pase a Recibido', async () => {
    const hallazgos = [{ id: 'h1', sistema: 'frenos', descripcion: 'Pastillas gastadas', gravedad: 'urgente', origen: 'video' }];
    const conHallazgo = base({ pendiente: true, propuesta: { ...base().propuesta, otraOrden: undefined, hallazgos } });
    mockApi({ 'GET /orders/{id}/assistant': { chat: conHallazgo, disponible: true, voz: false } });
    renderPage(<OrderAssistantCard ordenId="o41" estado="por_recibir" />);
    const guardar = await screen.findByRole('region', { name: 'Guardar' });
    expect(guardar).toHaveTextContent('Los hallazgos se guardan cuando la orden pase a Recibido; mientras tanto quedan en la ficha.');
  });

  it('sin nada pendiente ni otro vehículo no hay tarjetas', async () => {
    mockApi({ 'GET /orders/{id}/assistant': { chat: base({ propuesta: { ...base().propuesta, otraOrden: undefined } }), disponible: true, voz: false } });
    renderPage(<OrderAssistantCard ordenId="o41" estado="recibido" />);
    await screen.findByText('En los cuadros se ve la placa TPR-418.');
    expect(screen.queryByRole('region', { name: 'Guardar' })).not.toBeInTheDocument();
    expect(screen.queryByRole('region', { name: /Otro vehículo/ })).not.toBeInTheDocument();
  });
});

describe('Asistente general con ?chat=', () => {
  it('abre la conversación pedida (la que se creó con los datos del otro vehículo), no la última', async () => {
    const nueva = base({
      id: 'c99',
      ordenId: null,
      pendiente: true,
      propuesta: otra,
      mensajes: [{ id: 'i1', rol: 'asistente', texto: 'Traje lo que me contaste de TPR418.', adjuntos: [], opciones: [], creadoEn: '2026-11-26T13:20:00Z' }],
    });
    mockApi({
      'GET /me': owner,
      'GET /assistant/floor/latest': { chat: base(), disponible: true, voz: false },
      'GET /assistant/floor/chats/{id}': { chat: nueva },
    });
    renderPage(<AssistantPage />, '/panel/asistente?chat=c99');
    expect(await screen.findByText('Traje lo que me contaste de TPR418.')).toBeInTheDocument();
    expect(screen.getByRole('region', { name: 'Guardar' })).toHaveTextContent('¿Guardar la orden nueva para TPR418?');
    expect(screen.queryByText('En los cuadros se ve la placa TPR-418.')).not.toBeInTheDocument();
  });
});
