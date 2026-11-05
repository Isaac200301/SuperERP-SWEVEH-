import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { mockApi, orden, owner, renderPage } from '../../test/api';
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

  it('muestra los posibles repetidos lado a lado y une en el que el dueño elige (HU-15)', async () => {
    const carlos2 = { ...laura, id: 'c2', nombre: 'carlos ruiz', celular: '3009999999', autorizacionEstado: 'pendiente' };
    const carlos1 = { ...laura, id: 'c1', nombre: 'Carlos Ruiz', celular: '3001111111' };
    let merged = false;
    const calls = mockApi({
      'GET /me': { ...owner, permisos: [...owner.permisos, 'customer:merge', 'customer:read_all'] },
      'GET /customers': { clientes: [carlos1] },
      'GET /customers/duplicates': () => ({ grupos: merged ? [] : [{ clientes: [carlos1, carlos2] }] }),
      'GET /customers/{id}/vehicles': { vehiculos: [] },
      'POST /customers/merge': () => {
        merged = true;
        return { cliente: carlos2, movidos: { vehiculos: 1, ordenes: 0, conversaciones: 1 } };
      },
    });
    renderPage(<CustomersPage />);
    await screen.findByText('Posibles repetidos');
    await userEvent.click(screen.getByRole('radio', { name: /Conservar a carlos ruiz/ }));
    await userEvent.click(screen.getByRole('button', { name: 'Unir en carlos ruiz' }));
    expect(screen.getByRole('alert')).toHaveTextContent('Los vehículos, órdenes y conversaciones de Carlos Ruiz (3001111111) pasan a carlos ruiz (3009999999)');
    await userEvent.click(screen.getByRole('button', { name: 'Sí, unir' }));
    await waitFor(() => expect(screen.queryByText('Posibles repetidos')).not.toBeInTheDocument());
    expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ conservar: 'c2', unir: 'c1' });
  });

  it('el dueño descarga los datos del cliente y los borra con confirmación (HU-87)', async () => {
    const calls = mockApi({
      'GET /me': { ...owner, permisos: [...owner.permisos, 'data_request:manage', 'customer:read_all'] },
      'GET /customers': { clientes: [laura] },
      'GET /customers/{id}/vehicles': { vehiculos: [] },
      'GET /customers/{id}/export': { generadoEn: '2026-11-04T00:00:00Z', cliente: { nombre: 'Laura Gómez' }, autorizaciones: [], vehiculos: [],
        ordenes: [], mensajes: [] },
      'POST /customers/{id}/anonymize': { anonimizados: { vehiculos: 1, ordenes: 1, borradores: 0, mensajes: 3 } },
    });
    const createObjectURL = vi.fn(() => 'blob:datos');
    vi.stubGlobal('URL', Object.assign(URL, { createObjectURL, revokeObjectURL: vi.fn() }));
    renderPage(<CustomersPage />);
    await userEvent.click(await screen.findByRole('button', { name: /Laura Gómez/ }));
    await userEvent.click(await screen.findByRole('button', { name: 'Descargar sus datos' }));
    await waitFor(() => expect(createObjectURL).toHaveBeenCalled());

    await userEvent.click(screen.getByRole('button', { name: 'Borrar sus datos' }));
    expect(screen.getByRole('alert')).toHaveTextContent('No se puede deshacer');
    await userEvent.click(screen.getByRole('button', { name: 'Sí, borrar sus datos' }));
    await waitFor(() => expect(calls.some((c) => c.method === 'POST' && c.path === '/customers/c1/anonymize')).toBe(true));
  });

  it('muestra el historial del vehículo con la orden cancelada tachada y su motivo (HU-26)', async () => {
    mockApi({
      'GET /me': owner,
      'GET /customers': { clientes: [laura] },
      'GET /customers/{id}/vehicles': {
        vehiculos: [{ id: 'v1', clienteId: 'c1', tipo: 'carro', placa: 'CAN840', formatoPlaca: 'carro', marca: 'Renault', linea: 'Logan',
          modeloAnio: 2015, kilometraje: null, cilindrajeCc: null, turbo: null, motorPendiente: false, version: 1, creadoEn: '2026-11-01T00:00:00Z' }],
      },
      'GET /vehicles/{id}/history': {
        ordenes: [{ orden: orden({ id: 'o9', codigo: 'OS-0009', estado: 'cancelada', motivoCancelacion: 'El cliente no volvió' }), evidencias: [],
          cotizacion: null }],
      },
    });
    renderPage(<CustomersPage />);
    await userEvent.click(await screen.findByRole('button', { name: /Laura Gómez/ }));
    await userEvent.click(await screen.findByRole('button', { name: 'Ver historial' }));
    expect(await screen.findByText('Motivo: El cliente no volvió')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'OS-0009' })).toHaveClass('line-through');
  });
});
