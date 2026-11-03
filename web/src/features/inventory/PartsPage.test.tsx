import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { mockApi, owner, renderPage } from '../../test/api';
import { ServicesPage } from '../catalog/ServicesPage';
import { PartsPage } from './PartsPage';

afterEach(() => vi.unstubAllGlobals());

const dueno = { ...owner, permisos: [...owner.permisos, 'part:manage', 'part:cost', 'import:run'] };
const pastillas = {
  id: 'p1', referencia: 'PF-20', nombre: 'Pastillas', tipoVehiculo: 'carro', costo: 40000, precioVenta: 65000, existencia: 1, minimo: 2,
  bajoMinimo: true, activo: true, version: 3, marca: 'Bosch', categoria: 'frenos', ubicacion: 'Estante A3', referenciaOem: '0986494',
};
const body = (calls: { method: string; path: string; body: unknown }[], m: string, p: string) => calls.find((c) => c.method === m && c.path.startsWith(p))?.body;

describe('PartsPage (HU-32, HU-34 a HU-37)', () => {
  it('crea con la ficha, edita, registra entrada y muestra los movimientos', async () => {
    const calls = mockApi({
      'GET /me': dueno,
      'GET /parts': { repuestos: [pastillas] },
      'POST /parts': { status: 201, body: pastillas },
      'PATCH /parts/{id}': pastillas,
      'POST /parts/{id}/entries': pastillas,
      'GET /parts/{id}/movements': { movimientos: [{ id: 'm1', cantidad: -1, motivo: 'salida_orden', ordenId: 'o1', creadoEn: '2026-11-03T15:00:00Z' }] },
    });
    renderPage(<PartsPage />);
    expect(await screen.findByText('1 / mín. 2')).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText('Referencia'), 'FA-10');
    await userEvent.type(screen.getByLabelText('Nombre'), 'Filtro de aceite');
    await userEvent.type(screen.getByLabelText('Marca'), 'Mann');
    await userEvent.selectOptions(screen.getByLabelText('Categoría'), 'filtros');
    await userEvent.type(screen.getByLabelText('Ubicación'), 'Estante B1');
    await userEvent.type(screen.getByLabelText('Precio de venta (COP)'), '20.000');
    await userEvent.click(screen.getByRole('button', { name: 'Agregar repuesto' }));
    await waitFor(() => expect(body(calls, 'POST', '/parts')).toMatchObject({ referencia: 'FA-10', marca: 'Mann', categoria: 'filtros', ubicacion: 'Estante B1', precioVenta: 20000 }));

    await userEvent.click(screen.getByRole('button', { name: /Pastillas/ }));
    expect(await screen.findByText('Salida por orden')).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText('Cantidad que entró'), '5');
    await userEvent.click(screen.getByRole('button', { name: 'Registrar entrada' }));
    await userEvent.clear(screen.getByLabelText('Ubicación en bodega'));
    await userEvent.type(screen.getByLabelText('Ubicación en bodega'), 'Bodega 2');
    await userEvent.click(screen.getByRole('button', { name: 'Guardar cambios' }));
    await userEvent.click(screen.getByRole('button', { name: 'Desactivar' }));

    await waitFor(() => expect(calls.filter((c) => c.method === 'PATCH')).toHaveLength(2));
    expect(body(calls, 'POST', '/parts/p1/entries')).toEqual({ cantidad: 5 });
    expect(body(calls, 'PATCH', '/parts/p1')).toMatchObject({ ubicacion: 'Bodega 2', version: 3, costo: 40000 });
    expect(calls.filter((c) => c.method === 'PATCH')[1]?.body).toEqual({ activo: false, version: 3 });
  });

  it('el filtro "bajo el mínimo" usa /parts/low', async () => {
    const calls = mockApi({ 'GET /me': dueno, 'GET /parts': { repuestos: [] }, 'GET /parts/low': { repuestos: [pastillas] } });
    renderPage(<PartsPage />, '/panel/repuestos?bajo=1');
    expect(await screen.findByText('Pastillas')).toBeInTheDocument();
    expect(calls.some((c) => c.path === '/parts/low')).toBe(true);
  });

  it('importa con vista previa: con errores no deja confirmar', async () => {
    const calls = mockApi({
      'GET /me': dueno,
      'GET /parts': { repuestos: [] },
      'POST /parts/import': {
        filas: [
          { fila: 2, referencia: 'FA-10', nombre: 'Filtro', estado: 'ok', errores: [] },
          { fila: 3, referencia: 'BH-1', nombre: 'Bombillo', estado: 'error', errores: ['precio: es obligatorio'] },
        ],
        conError: 1, importadas: 0, avisos: ['La columna "Notas" no la reconocemos y se ignora.'],
      },
    });
    renderPage(<PartsPage />);
    await screen.findByText('Importar desde Excel (CSV)');
    const file = new File(['referencia,nombre,precio\n'], 'repuestos.csv', { type: 'text/csv' });
    await userEvent.upload(screen.getByLabelText('Archivo CSV'), file);
    expect(await screen.findByText('precio: es obligatorio')).toBeInTheDocument();
    expect(screen.getByText(/1 de 2 filas tienen errores/)).toBeInTheDocument();
    expect(screen.getByText(/La columna "Notas" no la reconocemos/)).toBeInTheDocument();
    expect(screen.getByText('Corrige las filas con error antes de importar.')).toBeInTheDocument();
    expect(calls.find((c) => c.method === 'POST')?.path).toBe('/parts/import?confirmar=0');
  });
});

describe('ServicesPage (HU-07)', () => {
  it('crea un servicio con categoría y garantía, y lo desactiva', async () => {
    const calls = mockApi({
      'GET /services': { servicios: [{ id: 's1', nombre: 'Alineación', tipoVehiculo: 'carro', precio: null, duracionMin: null, activo: true, version: 2, categoria: 'llantas', garantiaDias: 30 }] },
      'POST /services': { status: 201, body: {} },
      'PATCH /services/{id}': {},
    });
    renderPage(<ServicesPage />);
    expect(await screen.findByText('Garantía 30 días')).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText('Nombre'), 'Cambio de aceite');
    await userEvent.selectOptions(screen.getByLabelText('Categoría'), 'lubricantes');
    await userEvent.type(screen.getByLabelText('Precio (COP)'), '80.000');
    await userEvent.type(screen.getByLabelText('Garantía (días)'), '15');
    await userEvent.click(screen.getByRole('button', { name: 'Agregar servicio' }));
    await userEvent.click(screen.getByRole('button', { name: 'Desactivar' }));
    await waitFor(() => expect(calls.some((c) => c.method === 'PATCH')).toBe(true));
    expect(body(calls, 'POST', '/services')).toEqual({ nombre: 'Cambio de aceite', tipoVehiculo: 'ambos', precio: 80000, categoria: 'lubricantes', garantiaDias: 15 });
  });
});
