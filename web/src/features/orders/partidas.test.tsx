import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Orden } from '../../shared/api/client';
import { mockApi, orden, owner, renderPage } from '../../test/api';
import { QuotePanel } from './OrderWork';

afterEach(() => vi.unstubAllGlobals());

// R5 (V-07, HU-105, ADR-0040): cotización por partidas. Datos Ficticio.

const o = (over: Record<string, unknown> = {}) => orden(over) as unknown as Orden;
const partida = {
  id: 'l1', tipo: 'partida', refId: null, descripcion: 'Frenos completos', cantidad: 1, precioUnitario: 480000, subtotal: 480000,
  sinStock: false, aprobada: null, precioLibre: true, descuentoPct: 0, horas: null,
  detalle: [{ descripcion: 'Pastillas', cantidad: 2, valorRef: null }, { descripcion: 'Rectificar discos', cantidad: 1, valorRef: null }],
};
const cotizacion = (over: Record<string, unknown> = {}) => ({
  cotizacion: { id: 'q1', numero: 1, estado: 'borrador', total: 480000, enviadaEn: null, respondidaEn: null, canalRespuesta: null,
    lineas: [partida], mostrarDetalle: false, version: 7, subtotal: 480000, descuento: 0, ivaPct: 19, iva: 76639, preciosConIva: true, ...over },
});

describe('Cotización por partidas (V-07)', () => {
  it('agrega una partida con su nombre y el valor que pone el taller, sin sugerir ninguno', async () => {
    const calls = mockApi({
      'GET /me': owner, 'GET /orders/{id}/quote': { cotizacion: null }, 'GET /services': { servicios: [] }, 'GET /parts': { repuestos: [] },
      'POST /orders/{id}/quote/lines': cotizacion(),
    });
    renderPage(<QuotePanel orden={o()} />);
    await userEvent.type(await screen.findByLabelText('Nombre de la partida'), 'Frenos completos');
    expect(screen.getByLabelText('Valor de la partida (COP)')).toHaveValue('');
    await userEvent.click(screen.getByRole('button', { name: '+ Partida' }));
    expect(calls.filter((c) => c.method === 'POST')).toHaveLength(0);
    await userEvent.type(screen.getByLabelText('Valor de la partida (COP)'), '480000');
    await userEvent.click(screen.getByRole('button', { name: '+ Partida' }));
    await waitFor(() =>
      expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ tipo: 'partida', cantidad: 1, descripcion: 'Frenos completos', precioUnitario: 480000 }),
    );
  });

  it('el detalle interno va plegado, se amplía y "Mostrar detalle al cliente" se guarda', async () => {
    const calls = mockApi({
      'GET /me': owner, 'GET /orders/{id}/quote': cotizacion(), 'GET /services': { servicios: [] }, 'GET /parts': { repuestos: [] },
      'PUT /orders/{id}/quote/lines/{lineId}/detail': cotizacion(),
      'PATCH /orders/{id}/quote': cotizacion({ mostrarDetalle: true }),
    });
    renderPage(<QuotePanel orden={o()} />);
    const resumen = await screen.findByRole('button', { name: 'Detalle interno: 2 ítems · solo el taller' });
    expect(resumen).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByText('Rectificar discos')).not.toBeInTheDocument();
    expect(screen.getByText('Total con IVA incluido')).toBeInTheDocument();

    await userEvent.click(resumen);
    expect(screen.getByText('Rectificar discos')).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText('Agregar al detalle de Frenos completos'), 'Líquido de frenos');
    await userEvent.click(screen.getByRole('button', { name: 'Agregar' }));
    await waitFor(() =>
      expect(calls.find((c) => c.method === 'PUT')?.body).toEqual({
        detalle: [{ descripcion: 'Pastillas', cantidad: 2 }, { descripcion: 'Rectificar discos', cantidad: 1 }, { descripcion: 'Líquido de frenos', cantidad: 1 }],
        version: 7,
      }),
    );

    const sw = screen.getByRole('switch', { name: 'Mostrar detalle al cliente' });
    expect(sw).not.toBeChecked();
    await userEvent.click(sw);
    await waitFor(() => expect(calls.find((c) => c.method === 'PATCH')?.body).toEqual({ mostrarDetalle: true, version: 7 }));
  });

  it('enviada, el detalle se ve pero no se edita', async () => {
    mockApi({ 'GET /me': owner, 'GET /orders/{id}/quote': cotizacion({ estado: 'enviada' }) });
    renderPage(<QuotePanel orden={o()} />);
    await userEvent.click(await screen.findByRole('button', { name: /Detalle interno: 2 ítems/ }));
    expect(screen.queryByLabelText(/Agregar al detalle/)).not.toBeInTheDocument();
    expect(screen.getByRole('switch', { name: 'Mostrar detalle al cliente' })).toBeDisabled();
  });
});
