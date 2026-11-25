import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Orden, Sesion } from '../../shared/api/client';
import { mockApi, orden, owner, renderPage } from '../../test/api';
import { sectionsFor } from '../panel/sections';
import { QuotePanel } from './OrderWork';

afterEach(() => vi.unstubAllGlobals());

// R4 (V-06, ADR-0039): "Llevo el inventario en otro programa". Datos Ficticio.

const o = (over: Record<string, unknown> = {}) => orden(over) as unknown as Orden;
const parametros = (externo: boolean) => ({
  usarEsperandoRepuesto: false, usarControlCalidad: false, fotosMinimas: 4, kmObligatorio: true, itemsCarro: [], itemsMoto: [],
  ivaPct: 19, preciosConIva: true, tarifaHora: null, descuentoMaxPct: 10, descuentoRecepcion: false, vigenciaDias: 15,
  inventarioExterno: externo, version: 1,
});
const borrador = { cotizacion: { id: 'q1', numero: 1, estado: 'borrador', total: 0, enviadaEn: null, respondidaEn: null, canalRespuesta: null, lineas: [] } };

describe('Inventario en otro programa (V-06)', () => {
  it('Repuestos sale del menú', () => {
    const dueno = { ...owner, permisos: [...owner.permisos, 'part:manage'] } as Sesion;
    expect(sectionsFor(dueno, false, false).map((s) => s.to)).toContain('/panel/repuestos');
    expect(sectionsFor(dueno, false, true).map((s) => s.to)).not.toContain('/panel/repuestos');
  });

  it('la cotización pide el repuesto a mano, con el valor del taller, y no muestra los del inventario', async () => {
    const calls = mockApi({
      'GET /me': owner,
      'GET /workshop/settings': parametros(true),
      'GET /orders/{id}/quote': borrador,
      'GET /services': { servicios: [] },
      'GET /parts': { repuestos: [{ id: 'p1', referencia: 'PF-20', nombre: 'Pastillas', tipoVehiculo: 'carro', precioVenta: 65000, existencia: 3, minimo: 1, bajoMinimo: false, activo: true, version: 1 }] },
      'POST /orders/{id}/quote/lines': borrador,
    });
    renderPage(<QuotePanel orden={o()} />);
    const form = await screen.findByRole('form', { name: 'Repuesto escrito a mano' });
    expect(screen.getByLabelText('Buscar servicio')).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText('Buscar servicio'), 'Past');
    await waitFor(() => expect(calls.some((c) => c.path.startsWith('/parts'))).toBe(true));
    expect(screen.queryByText(/Repuesto · Pastillas/)).not.toBeInTheDocument();

    await userEvent.type(screen.getByLabelText('Repuesto'), 'Kit de embrague');
    // Sin valor no deja agregar: nunca se sugiere un precio (RF-17).
    expect(screen.getByLabelText('Valor del repuesto (COP)')).toHaveValue('');
    await userEvent.click(screen.getByRole('button', { name: 'Agregar repuesto' }));
    expect(calls.filter((c) => c.method === 'POST')).toHaveLength(0);
    await userEvent.type(screen.getByLabelText('Valor del repuesto (COP)'), '480000');
    await userEvent.click(screen.getByRole('button', { name: 'Agregar repuesto' }));
    await waitFor(() =>
      expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ tipo: 'repuesto', descripcion: 'Kit de embrague', cantidad: 1, precioUnitario: 480000 }),
    );
    expect(form).toBeInTheDocument();
  });

  it('con el inventario de SWEVEH, no aparece el repuesto a mano', async () => {
    mockApi({ 'GET /me': owner, 'GET /workshop/settings': parametros(false), 'GET /orders/{id}/quote': borrador, 'GET /services': { servicios: [] }, 'GET /parts': { repuestos: [] } });
    renderPage(<QuotePanel orden={o()} />);
    expect(await screen.findByLabelText('Buscar servicio o repuesto')).toBeInTheDocument();
    expect(screen.queryByRole('form', { name: 'Repuesto escrito a mano' })).not.toBeInTheDocument();
  });
});
