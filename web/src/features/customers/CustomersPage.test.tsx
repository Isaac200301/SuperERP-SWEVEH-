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
});
