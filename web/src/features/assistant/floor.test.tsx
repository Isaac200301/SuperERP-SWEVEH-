import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { mockApi, owner, renderPage } from '../../test/api';
import { AssistantPage } from './AssistantPage';
import { OrderAssistantCard } from './OrderAssistantCard';

afterEach(() => vi.unstubAllGlobals());

// R8 (V-11, HU-107, ADR-0041): el asistente del piso y su ficha en vivo. Datos Ficticio.

const propuesta = (over: Record<string, unknown> = {}) => ({
  placa: { valor: 'ABC123', origen: 'dijiste' },
  tipo: 'carro',
  nuevaOrden: true,
  vehiculoNuevo: true,
  cliente: { nombre: { valor: 'Carlos Gómez', origen: 'dijiste' }, celular: { valor: '3001112233', origen: 'dijiste' } },
  sintoma: { valor: 'le suenan los frenos', origen: 'dijiste' },
  cotizacion: [
    { id: 'l1', tipo: 'servicio', nombre: 'Cambio de aceite', cantidad: 1, precio: 45000, origenPrecio: 'lista' },
    { id: 'l2', tipo: 'repuesto', nombre: 'Filtro de aceite', cantidad: 1, precio: null },
  ],
  ...over,
});

const chat = (over: Record<string, unknown> = {}) => ({
  id: 'c1',
  ordenId: null,
  version: 3,
  propuesta: propuesta(),
  pendiente: true,
  progreso: { tiene: 7, total: 8, falta: ['la parte trasera'] },
  disponible: true,
  voz: false,
  mensajes: [
    { id: 'm1', rol: 'usuario', texto: 'llegó el Mazda ABC123', adjuntos: [], opciones: [], creadoEn: '2026-11-27T15:00:00Z' },
    { id: 'm2', rol: 'asistente', texto: 'Orden nueva para ABC123. ¿Cuánto vale el filtro?', adjuntos: [], opciones: [], creadoEn: '2026-11-27T15:00:01Z', conPropuesta: true },
  ],
  ...over,
});

describe('Asistente del piso (R8)', () => {
  it('muestra la ficha en vivo con lo que falta, de dónde salió cada dato y "Sin precio · ponlo"', async () => {
    mockApi({ 'GET /me': owner, 'GET /assistant/floor/latest': { chat: chat(), disponible: true, voz: false } });
    renderPage(<AssistantPage />);
    expect(await screen.findByText('7 de 8 datos · falta la parte trasera')).toBeInTheDocument();
    expect(screen.getByText('Carlos Gómez')).toBeInTheDocument();
    expect(screen.getByText('de tu lista')).toBeInTheDocument();
    expect(screen.getByText('Propuesta · revísala arriba')).toBeInTheDocument();
    expect(screen.getByLabelText('Sin precio · ponlo')).toHaveValue('');
    // DR-06: Confirmar dice qué falta en vez de quedar gris y mudo.
    expect(screen.getByRole('button', { name: 'Confirmar' })).toBeDisabled();
    // El cuadro de guardar del chat dice lo mismo (los dos usan el mismo confirmar).
    expect(screen.getByRole('button', { name: 'Guardar orden' })).toBeDisabled();
    expect(screen.getAllByText('Falta el precio de 1 ítem de la cotización.')).toHaveLength(2);
  });

  it('el precio que pone la persona se guarda en la ficha con su versión', async () => {
    const calls = mockApi({
      'GET /me': owner,
      'GET /assistant/floor/latest': { chat: chat(), disponible: true, voz: false },
      'PATCH /assistant/floor/chats/{id}': { chat: chat({ version: 4 }) },
    });
    renderPage(<AssistantPage />);
    await userEvent.type(await screen.findByLabelText('Sin precio · ponlo'), '32.000');
    await userEvent.click(screen.getByRole('button', { name: 'Poner' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'PATCH')).toBeDefined());
    const body = calls.find((c) => c.method === 'PATCH')?.body as { version: number; propuesta: { cotizacion: { precio: number | null }[] } };
    expect(body.version).toBe(3);
    expect(body.propuesta.cotizacion[1]?.precio).toBe(32000);
  });

  it('quitar un dato de la ficha lo saca de la propuesta', async () => {
    const calls = mockApi({
      'GET /me': owner,
      'GET /assistant/floor/latest': { chat: chat(), disponible: true, voz: false },
      'PATCH /assistant/floor/chats/{id}': { chat: chat({ version: 4 }) },
    });
    renderPage(<AssistantPage />);
    await userEvent.click(await screen.findByRole('button', { name: 'Quitar Qué trae' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'PATCH')).toBeDefined());
    expect((calls.find((c) => c.method === 'PATCH')?.body as { propuesta: Record<string, unknown> }).propuesta.sintoma).toBeUndefined();
  });

  it('Confirmar manda una Idempotency-Key y muestra lo que quedó guardado', async () => {
    const listo = chat({ propuesta: { placa: { valor: 'ABC123', origen: 'dijiste' }, ordenId: 'o1', ordenCodigo: 'OS-0001' }, pendiente: false, ordenId: 'o1' });
    let key = '';
    mockApi({
      'GET /me': owner,
      'GET /assistant/floor/latest': { chat: chat({ propuesta: propuesta({ cotizacion: [] }) }), disponible: true, voz: false },
      'POST /assistant/floor/chats/{id}/confirm': (req: Request) => {
        key = req.headers.get('Idempotency-Key') ?? '';
        return {
          resultado: { ordenId: 'o1', ordenCodigo: 'OS-0001', ordenEstado: 'por_recibir', ordenNueva: true, hecho: ['la orden OS-0001', '4 fotos'], fotos: 4 },
          chat: listo,
        };
      },
    });
    renderPage(<AssistantPage />);
    await userEvent.click(await screen.findByRole('button', { name: 'Confirmar' }));
    expect(await screen.findByText('Quedó guardado: la orden OS-0001, 4 fotos.')).toBeInTheDocument();
    expect(key).toMatch(/^[0-9a-f-]{36}$/);
    expect(screen.getByRole('link', { name: /Seguimos en la orden OS-0001/ })).toHaveAttribute('href', '/panel/ordenes/o1');
  });

  it('dos placas: un botón por placa manda la escogida', async () => {
    const calls = mockApi({
      'GET /me': owner,
      'GET /assistant/floor/latest': {
        chat: chat({
          pendiente: false,
          progreso: { tiene: 0, total: 0, falta: [] },
          propuesta: {},
          mensajes: [{ id: 'm9', rol: 'asistente', texto: '¿Seguimos con ABC123 o con XYZ98D?', adjuntos: [], opciones: ['ABC123', 'XYZ98D'], creadoEn: '2026-11-27T15:00:00Z' }],
        }),
        disponible: true,
        voz: false,
      },
      'POST /assistant/floor/messages': { chat: chat() },
    });
    renderPage(<AssistantPage />);
    await userEvent.click(await screen.findByRole('button', { name: 'Seguir con XYZ98D' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'POST')).toBeDefined());
    expect(String(calls.find((c) => c.method === 'POST')?.body)).toContain('XYZ98D');
  });

  it('sin IA dice que siga con el formulario', async () => {
    mockApi({ 'GET /me': owner, 'GET /assistant/floor/latest': { chat: null, disponible: false, voz: false } });
    renderPage(<AssistantPage />);
    expect(await screen.findByText('El asistente no está disponible')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Hablar' })).not.toBeInTheDocument();
  });

  it('en la orden, el primer mensaje empieza la conversación de esa orden', async () => {
    const calls = mockApi({
      'GET /me': owner,
      'GET /orders/{id}/assistant': { chat: null, disponible: true, voz: true },
      'POST /assistant/floor/messages': { chat: chat({ ordenId: 'o7' }) },
    });
    renderPage(<OrderAssistantCard ordenId="o7" />);
    expect(await screen.findByRole('button', { name: 'Hablar' })).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText('Mensaje para el asistente'), 'pásala a diagnóstico');
    await userEvent.click(screen.getByRole('button', { name: 'Enviar' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'POST')).toBeDefined());
    const body = String(calls.find((c) => c.method === 'POST')?.body);
    expect(body).toContain('o7');
    expect(body).toContain('pásala a diagnóstico');
  });
});
