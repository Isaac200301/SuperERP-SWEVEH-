import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { mockApi, owner, renderPage } from '../../test/api';
import { PriceListReview, PriceListUpload } from './PriceList';

afterEach(() => vi.unstubAllGlobals());

const dueno = { ...owner, permisos: [...owner.permisos, 'service:manage'] };
const fila = (id: string, nombre: string, precio: number | null) => ({
  id,
  loteId: 'l1',
  nombre,
  precio,
  tipoVehiculo: 'ambos',
  estado: 'pendiente',
  servicioId: null,
});

describe('Lista de precios por foto (HU-93)', () => {
  it('sube la foto y dice cuántos servicios leyó', async () => {
    mockApi({
      'GET /me': dueno,
      'POST /services/photo': { status: 201, body: { propuestas: [fila('p1', 'Cambio de aceite', 45000), fila('p2', 'Frenos', 40000)] } },
    });
    renderPage(<PriceListUpload />);
    const foto = new File([new Uint8Array([0xff, 0xd8, 0xff])], 'lista.jpg', { type: 'image/jpeg' });
    await userEvent.upload(screen.getByLabelText(/Foto de la lista/), foto);
    expect(await screen.findByText(/Leímos 2 servicios/)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Por revisar/ })).toHaveAttribute('href', '/panel/por-revisar');
  });

  it('muestra las filas con la foto al lado y aprueba con la corrección', async () => {
    const calls = mockApi({
      'GET /me': dueno,
      'GET /services/proposals': { propuestas: [fila('p1', 'Cambio de aceite', 45000), fila('p2', 'Cambio de amortiguadores', null)] },
      'POST /services/proposals/{id}': fila('p1', 'Cambio de aceite', 50000),
    });
    renderPage(<PriceListReview />);
    expect(await screen.findByAltText('Foto de la lista de precios')).toHaveAttribute('src', '/api/v1/services/proposals/p1/photo');
    const precios = screen.getAllByLabelText(/Precio/);
    await userEvent.clear(precios[0] as HTMLElement);
    await userEvent.type(precios[0] as HTMLElement, '50.000');
    await userEvent.click(screen.getAllByRole('button', { name: 'Aprobar' })[0] as HTMLElement);
    await waitFor(() =>
      expect(calls.find(c => c.method === 'POST')?.body).toEqual({
        decision: 'aprobar',
        nombre: 'Cambio de aceite',
        precio: 50000,
        tipoVehiculo: 'ambos',
      }),
    );
  });
});
