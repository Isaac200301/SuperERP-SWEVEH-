import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Orden } from '../../shared/api/client';
import { mockApi, orden, owner, renderPage } from '../../test/api';
import { SpendPanel } from './SpendPanel';

afterEach(() => vi.unstubAllGlobals());

// R6 (V-08, HU-106, ADR-0040): lo gastado contra lo cotizado. Datos Ficticio.

const o = (over: Record<string, unknown> = {}) => orden(over) as unknown as Orden;
const gasto = (over: Record<string, unknown> = {}) => ({
  id: 'g1', descripcion: 'Pastillas delanteras', valor: 60000, origen: 'manual', fotoId: null, creadoEn: '2026-11-25T15:00:00Z', anulado: false, ...over,
});
const lista = (gastado: number, gastos: unknown[] = [gasto()]) => ({
  gastos,
  resumen: { cotizado: 100000, gastado, quedan: 100000 - gastado },
});
const conPermiso = { ...owner, permisos: [...owner.permisos, 'expense:manage'] };

describe('Lo gastado en la obra (V-08)', () => {
  it('muestra cotizado, gastado y lo que quedan, y avisa cuando se pasó', async () => {
    mockApi({ 'GET /me': conPermiso, 'GET /orders/{id}/expenses': lista(115000, [gasto({ valor: 115000 })]) });
    renderPage(<SpendPanel orden={o()} />);
    expect(await screen.findByText('Lo gastado en la obra')).toBeInTheDocument();
    expect(screen.getByText('Solo el taller: el cliente no lo ve.')).toBeInTheDocument();
    expect(screen.getByText('Se pasó')).toBeInTheDocument();
    expect(screen.getByRole('meter', { name: 'Gastado de lo cotizado' })).toHaveAttribute('aria-valuenow', '100');
  });

  it('con lo gastado dentro de lo cotizado dice "Quedan"', async () => {
    mockApi({ 'GET /me': conPermiso, 'GET /orders/{id}/expenses': lista(60000) });
    renderPage(<SpendPanel orden={o()} />);
    expect(await screen.findByText('Quedan')).toBeInTheDocument();
    expect(screen.getByRole('meter', { name: 'Gastado de lo cotizado' })).toHaveAttribute('aria-valuenow', '60');
  });

  it('anota un gasto con el valor que pone la persona y un id propio para no repetirlo', async () => {
    const calls = mockApi({
      'GET /me': conPermiso,
      'GET /orders/{id}/expenses': lista(0, []),
      'POST /orders/{id}/expenses': { status: 201, body: { gasto: gasto(), resumen: { cotizado: 100000, gastado: 60000, quedan: 40000 } } },
    });
    renderPage(<SpendPanel orden={o()} />);
    await userEvent.click(await screen.findByRole('button', { name: /Anotar un gasto/ }));
    expect(screen.getByLabelText('Valor del gasto (COP)')).toHaveValue('');
    await userEvent.type(screen.getByLabelText('¿Qué fue?'), 'Pastillas delanteras');
    await userEvent.click(screen.getByRole('button', { name: 'Guardar el gasto' }));
    expect(calls.filter((c) => c.method === 'POST')).toHaveLength(0); // sin valor no se anota
    await userEvent.type(screen.getByLabelText('Valor del gasto (COP)'), '60000');
    await userEvent.click(screen.getByRole('button', { name: 'Guardar el gasto' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'POST')).toBeDefined());
    const body = calls.find((c) => c.method === 'POST')?.body as Record<string, unknown>;
    expect(body).toMatchObject({ descripcion: 'Pastillas delanteras', valor: 60000, origen: 'manual' });
    expect(String(body.id)).toMatch(/^[0-9a-f-]{36}$/);
  });

  it('anula un gasto sin borrarlo', async () => {
    const calls = mockApi({
      'GET /me': conPermiso,
      'GET /orders/{id}/expenses': lista(60000),
      'POST /orders/{id}/expenses/{gid}/void': { resumen: { cotizado: 100000, gastado: 0, quedan: 100000 } },
    });
    renderPage(<SpendPanel orden={o()} />);
    await userEvent.click(await screen.findByRole('button', { name: 'Anular Pastillas delanteras' }));
    await waitFor(() => expect(calls.some((c) => c.method === 'POST' && c.path.endsWith('/void'))).toBe(true));
  });

  it('quien no tiene el permiso no ve el bloque', async () => {
    mockApi({ 'GET /me': owner });
    renderPage(<SpendPanel orden={o()} />);
    await waitFor(() => expect(screen.queryByText('Lo gastado en la obra')).not.toBeInTheDocument());
  });
});
