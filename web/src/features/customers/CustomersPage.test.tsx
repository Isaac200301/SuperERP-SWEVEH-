import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { mockApi, owner, renderPage } from '../../test/api';
import { CustomersPage } from './CustomersPage';

afterEach(() => vi.unstubAllGlobals());

const laura = { id: 'c1', nombre: 'Laura Gómez', celular: '3001234567', correo: null, autorizacionEstado: 'autorizada', creadoEn: '2026-11-01T00:00:00Z' };

describe('CustomersPage (HU-82)', () => {
  it('busca clientes y muestra sus vehículos al abrirlos', async () => {
    const calls = mockApi({
      'GET /me': owner,
      'GET /customers': (req: Request) => ({ clientes: new URL(req.url).searchParams.get('q') === 'nadie' ? [] : [laura] }),
      'GET /customers/{id}/vehicles': {
        vehiculos: [{ id: 'v1', clienteId: 'c1', tipo: 'moto', placa: 'GNO66F', formatoPlaca: 'moto', marca: 'Yamaha', linea: 'NMAX',
          modeloAnio: 2022, kilometraje: null, cilindrajeCc: null, turbo: null, motorPendiente: true, version: 1, creadoEn: '2026-11-01T00:00:00Z' }],
      },
    });
    renderPage(<CustomersPage />);
    await userEvent.click(await screen.findByRole('button', { name: /Laura Gómez/ }));
    expect(await screen.findByText('Yamaha NMAX 2022')).toBeInTheDocument();
    expect(screen.getByText('Motor pendiente')).toBeInTheDocument();
    expect(screen.getByText('Autorizó sus datos')).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText('Buscar por nombre o celular'), 'nadie');
    expect(await screen.findByText('Ningún cliente coincide con la búsqueda.')).toBeInTheDocument();
    expect(calls.some((c) => c.path === '/customers?q=nadie')).toBe(true);
  });

  it('el dueño sube la plantilla, ve las filas con error y confirma (HU-92)', async () => {
    let confirmed = false;
    const calls = mockApi({
      'GET /me': { ...owner, permisos: [...owner.permisos, 'import:run', 'customer:read_all'] },
      'GET /customers': { clientes: [] },
      'POST /vehicles/import': (req: Request) => {
        if (new URL(req.url).searchParams.get('confirmar') === '1') {
          confirmed = true;
          return { filas: [{ fila: 2, referencia: 'ABC123', nombre: 'Carlos Ruiz', estado: 'ok', errores: [] }], conError: 0, importadas: 1,
            clientesNuevos: 1, avisos: [] };
        }
        return { filas: [{ fila: 2, referencia: 'ABC123', nombre: 'Carlos Ruiz', estado: 'ok', errores: [] }], conError: 0, importadas: 0,
          clientesNuevos: 0, avisos: [] };
      },
    });
    renderPage(<CustomersPage />);
    await screen.findByText('Subir clientes y vehículos (CSV)');
    await userEvent.upload(screen.getByLabelText('Archivo CSV'), new File(['Nombre;Celular;Placa;Tipo\n'], 'clientes.csv', { type: 'text/csv' }));
    expect(await screen.findByText('Las 1 filas están bien.')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Importar 1 vehículos' }));
    expect(await screen.findByText('Listo: se importaron 1 vehículos (1 cliente nuevo).')).toBeInTheDocument();
    expect(confirmed).toBe(true);
    expect(calls.filter((c) => c.method === 'POST').map((c) => c.path)).toEqual(['/vehicles/import?confirmar=0', '/vehicles/import?confirmar=1']);
  });
});
