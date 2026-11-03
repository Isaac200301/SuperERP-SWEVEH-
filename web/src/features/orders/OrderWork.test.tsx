import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Orden } from '../../shared/api/client';
import { mechanic, mockApi, orden, owner, renderPage } from '../../test/api';
import { FloorPage } from './FloorPage';
import { OrderActions, QuotePanel, ReceptionPanel } from './OrderWork';

afterEach(() => vi.unstubAllGlobals());
const o = (over: Record<string, unknown> = {}) => orden(over) as unknown as Orden;

const checklist = {
  tipo: 'carro', kilometraje: null, combustible: null, accesorios: [], salvedades: [],
  ladosConFoto: ['frente'], ladosPedidos: ['frente', 'trasera', 'izquierdo', 'derecho'], falta: ['fotos: trasera', 'kilometraje'], completadoEn: null,
};
const cotizacion = (over: Record<string, unknown> = {}) => ({
  cotizacion: {
    id: 'q1', numero: 1, estado: 'borrador', total: 145000, enviadaEn: null, respondidaEn: null, canalRespuesta: null,
    lineas: [{ id: 'l1', tipo: 'repuesto', refId: 'p1', descripcion: 'Pastillas (PF-20)', cantidad: 1, precioUnitario: 65000, subtotal: 65000, sinStock: true, aprobada: null }],
    ...over,
  },
});

describe('OrderActions (HU-20, HU-25)', () => {
  it('avanza con la versión y se devuelve un paso', async () => {
    const calls = mockApi({ 'GET /me': owner, 'POST /orders/{id}/transition': orden() });
    renderPage(<OrderActions orden={o({ estado: 'recibido' })} />);
    await userEvent.click(screen.getByRole('button', { name: 'Pasar a En diagnóstico' }));
    await userEvent.click(screen.getByRole('button', { name: 'Devolver a Por recibir' }));
    await waitFor(() => expect(calls.filter((c) => c.method === 'POST')).toHaveLength(2));
    expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ estado: 'en_diagnostico', version: 3 });
  });

  it('el dueño escribe el motivo para seguir sin aprobación; el mecánico no ve ese campo', async () => {
    const calls = mockApi({ 'GET /me': owner, 'POST /orders/{id}/transition': { status: 409, body: { detail: 'x', code: 'version_conflicto' } } });
    renderPage(<OrderActions orden={o({ estado: 'esperando_aprobacion' })} />);
    await userEvent.type(await screen.findByLabelText(/Motivo si el cliente aprobó/), 'aprobó por teléfono');
    await userEvent.click(screen.getByRole('button', { name: 'Pasar a En reparación' }));
    expect(await screen.findByText(/Alguien cambió esta orden/)).toBeInTheDocument();
    expect(calls.at(-1)?.body).toMatchObject({ motivo: 'aprobó por teléfono' });

    vi.unstubAllGlobals();
    mockApi({ 'GET /me': mechanic });
    renderPage(<OrderActions orden={o({ estado: 'esperando_aprobacion' })} />);
    await waitFor(() => expect(screen.getAllByRole('button', { name: 'Pasar a En reparación' })).toHaveLength(2));
    expect(screen.getAllByLabelText(/Motivo si el cliente aprobó/)).toHaveLength(1);
  });

  it('para entregar pide quién recoge y los accesorios; entregada no muestra acciones', async () => {
    const calls = mockApi({ 'GET /me': owner, 'POST /orders/{id}/transition': orden() });
    const { unmount } = renderPage(<OrderActions orden={o({ estado: 'listo' })} />);
    await userEvent.type(screen.getByLabelText('¿Quién recoge?'), 'Laura');
    await userEvent.click(screen.getByLabelText(/Revisé con el cliente/));
    await userEvent.click(screen.getByRole('button', { name: 'Pasar a Entregado' }));
    await waitFor(() => expect(calls.at(-1)?.body).toMatchObject({ recogidoPor: 'Laura', accesoriosConfirmados: true }));
    unmount();
    renderPage(<OrderActions orden={o({ estado: 'entregado' })} />);
    expect(screen.queryByText('Mover la orden')).not.toBeInTheDocument();
  });
});

describe('ReceptionPanel (HU-17, HU-21)', () => {
  it('dice qué falta, guarda el checklist, sube fotos y notas', async () => {
    const calls = mockApi({
      'GET /orders/{id}/checklist': checklist,
      'PUT /orders/{id}/checklist': checklist,
      'GET /orders/{id}/evidence': {
        evidencias: [
          { id: 'e1', lado: 'frente', nota: null, tieneFoto: true, mostrarCliente: true, etapa: 'recibido', autorId: null, creadoEn: '2026-11-02T15:00:00Z' },
          { id: 'e2', lado: null, nota: 'Buje gastado', tieneFoto: false, mostrarCliente: false, etapa: 'en_diagnostico', autorId: null, creadoEn: '2026-11-02T16:00:00Z' },
        ],
      },
      'POST /orders/{id}/evidence': { id: 'e3' },
    });
    renderPage(<ReceptionPanel orden={o({ estado: 'por_recibir' })} />);
    expect(await screen.findByText('fotos: trasera; kilometraje')).toBeInTheDocument();
    expect(await screen.findByText('Buje gastado')).toBeInTheDocument();
    expect(screen.getByAltText('Foto frente')).toHaveAttribute('src', '/api/v1/orders/o1/evidence/e1/file');

    await userEvent.type(screen.getByLabelText('Kilometraje'), '45a000');
    await userEvent.selectOptions(screen.getByLabelText('Combustible'), '1/2');
    await userEvent.type(screen.getByLabelText(/Accesorios/), 'gato, llanta');
    await userEvent.click(screen.getByRole('button', { name: 'Guardar checklist' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'PUT')?.body).toEqual({ kilometraje: 45000, combustible: '1/2', accesorios: ['gato', 'llanta'] }));

    const foto = new File([new Uint8Array([0xff, 0xd8, 0xff])], 'f.jpg', { type: 'image/jpeg' });
    const inputs = document.querySelectorAll<HTMLInputElement>('input[type=file]');
    await userEvent.upload(inputs[1] as HTMLInputElement, foto);
    await userEvent.type(screen.getByLabelText(/Nota/), 'Cambié el buje');
    await userEvent.click(screen.getByRole('button', { name: 'Agregar nota' }));
    await waitFor(() => expect(calls.filter((c) => c.method === 'POST' && c.path.endsWith('/evidence'))).toHaveLength(2));
  });

  it('una orden cerrada no se edita', async () => {
    mockApi({ 'GET /orders/{id}/checklist': { ...checklist, falta: [], completadoEn: '2026-11-02T15:00:00Z' }, 'GET /orders/{id}/evidence': { evidencias: [] } });
    renderPage(<ReceptionPanel orden={o({ estado: 'entregado' })} />);
    expect(await screen.findByText('Recepción · completa')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Guardar checklist' })).not.toBeInTheDocument();
  });
});

describe('QuotePanel (HU-27, HU-29)', () => {
  it('agrega de la lista del taller, quita y envía', async () => {
    const calls = mockApi({
      'GET /me': owner,
      'GET /orders/{id}/quote': cotizacion(),
      'GET /services': { servicios: [{ id: 's1', nombre: 'Alineación', tipoVehiculo: 'carro', precio: 60000, duracionMin: 30, activo: true, version: 1 }, { id: 's2', nombre: 'Revisión', tipoVehiculo: 'carro', precio: null, duracionMin: null, activo: true, version: 1 }] },
      'GET /parts': { repuestos: [{ id: 'p1', referencia: 'PF-20', nombre: 'Pastillas', tipoVehiculo: 'carro', precioVenta: 65000, existencia: 0, minimo: 1, bajoMinimo: true, activo: true, version: 1 }] },
      'POST /orders/{id}/quote/lines': cotizacion(),
      'DELETE /orders/{id}/quote/lines/{lineId}': cotizacion(),
      'POST /orders/{id}/quote/send': cotizacion({ estado: 'enviada' }),
    });
    renderPage(<QuotePanel orden={o()} />);
    expect(await screen.findByText('sin existencia')).toBeInTheDocument();
    expect(screen.getByText('precio por confirmar')).toBeInTheDocument();
    await userEvent.click(await screen.findByRole('button', { name: /Servicio · Alineación/ }));
    await userEvent.click(screen.getByRole('button', { name: /Repuesto · Pastillas/ }));
    await userEvent.click(screen.getByRole('button', { name: 'Quitar' }));
    await userEvent.click(screen.getByRole('button', { name: 'Enviar cotización al cliente' }));
    await waitFor(() => expect(calls.some((c) => c.path.endsWith('/quote/send'))).toBe(true));
    expect(calls.filter((c) => c.path.endsWith('/quote/lines')).map((c) => c.body)).toEqual([
      { tipo: 'servicio', refId: 's1', cantidad: 1 },
      { tipo: 'repuesto', refId: 'p1', cantidad: 1 },
    ]);
  });

  it('sin cotización muestra una vacía y no deja enviar', async () => {
    mockApi({ 'GET /me': owner, 'GET /orders/{id}/quote': { cotizacion: null }, 'GET /services': { servicios: [] }, 'GET /parts': { repuestos: [] } });
    renderPage(<QuotePanel orden={o()} />);
    expect(await screen.findByText('Aún no hay ítems.')).toBeInTheDocument();
    expect(screen.getByText('Agrega al menos un ítem para enviarla.')).toBeInTheDocument();
  });

  it('registra la respuesta solo si el rol puede', async () => {
    const calls = mockApi({ 'GET /me': owner, 'GET /orders/{id}/quote': cotizacion({ estado: 'enviada' }), 'POST /orders/{id}/quote/response': { status: 422, body: { detail: 'No se pudo.' } } });
    renderPage(<QuotePanel orden={o()} />);
    await userEvent.click(await screen.findByRole('button', { name: 'Aprobó por teléfono' }));
    await userEvent.click(screen.getByRole('button', { name: 'Aprobó en el mostrador' }));
    await userEvent.click(screen.getByRole('button', { name: 'No aprobó' }));
    expect(await screen.findByText('No se pudo.')).toBeInTheDocument();
    expect(calls.filter((c) => c.path.endsWith('/response')).map((c) => c.body)).toEqual([
      { decision: 'aprobar', canal: 'telefono' },
      { decision: 'aprobar', canal: 'mostrador' },
      { decision: 'rechazar', canal: 'telefono' },
    ]);
  });

  it('con la cotización aprobada devuelve al inventario un repuesto que no se usó (HU-38)', async () => {
    const calls = mockApi({
      'GET /me': owner,
      'GET /orders/{id}/quote': cotizacion({ estado: 'aprobada', lineas: [{ ...cotizacion().cotizacion.lineas[0], aprobada: true, sinStock: false }] }),
      'POST /parts/{id}/returns': { status: 422, body: { detail: 'No puedes devolver más unidades de las que esa orden descontó.' } },
    });
    renderPage(<QuotePanel orden={o({ estado: 'en_reparacion' })} />);
    await userEvent.click(await screen.findByRole('button', { name: 'Devolver 1 al inventario' }));
    expect(await screen.findByText(/No puedes devolver más/)).toBeInTheDocument();
    expect(calls.find((c) => c.path === '/parts/p1/returns')?.body).toEqual({ cantidad: 1, ordenId: 'o1' });
  });
});

describe('FloorPage (HU-23)', () => {
  it('muestra las órdenes del mecánico y el estado vacío', async () => {
    mockApi({ 'GET /orders/mine': { ordenes: [orden({ fechaPrometida: '2026-11-03T17:00:00Z' }), orden({ id: 'o2', vehiculo: { ...orden().vehiculo, marca: null, linea: null, tipo: 'moto' } })] } });
    const { unmount } = renderPage(<FloorPage />);
    expect(await screen.findByText(/Prometida:/)).toBeInTheDocument();
    expect(screen.getByText('Moto')).toBeInTheDocument();
    unmount();
    mockApi({ 'GET /orders/mine': { ordenes: [] } });
    renderPage(<FloorPage />);
    expect(await screen.findByText(/No hay órdenes abiertas/)).toBeInTheDocument();
  });
});
