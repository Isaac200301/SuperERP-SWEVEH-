import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Route, Routes } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { mockApi, orden, owner, renderPage } from '../../test/api';
import { NewOrderPage } from './NewOrderPage';

afterEach(() => vi.unstubAllGlobals());

// R11 (HU-86, R-10): al abrir una orden con un cliente nuevo, se le lee la autorización y se marca lo que respondió
// antes de guardar sus datos. Datos Ficticio.
describe('Autorización en el mostrador al crear el cliente (HU-86)', () => {
  const rutas = (posts: unknown[]) => ({
    'GET /me': owner,
    'GET /vehicles/by-plate/{placa}': { status: 404, body: { detail: 'No existe' } },
    'GET /customers/by-phone/{celular}': { status: 404, body: { detail: 'No existe' } },
    'POST /customers': (req: Request) => {
      posts.push(req.url);
      return { status: 201, body: { cliente: { id: 'c1', nombre: 'Pedro Ríos', celular: '3000000761' } } };
    },
    'POST /vehicles': { status: 201, body: { vehiculo: { id: 'v1', placa: 'TPR418', tipo: 'carro', motorPendiente: true } } },
    'POST /orders': { status: 201, body: orden({ id: 'o9', placa: 'TPR418' }) },
  });

  it('no guarda al cliente nuevo hasta marcar su respuesta, y la manda con él', async () => {
    const posts: unknown[] = [];
    const calls = mockApi(rutas(posts));
    renderPage(
      <Routes>
        <Route path="/panel/ordenes/nueva" element={<NewOrderPage />} />
        <Route path="/panel/ordenes/:id" element={<p>Orden abierta</p>} />
      </Routes>,
      '/panel/ordenes/nueva',
    );
    await userEvent.type(screen.getByLabelText('Placa'), 'TPR418');
    await userEvent.type(await screen.findByLabelText('Celular del cliente'), '3000000761');
    await userEvent.type(await screen.findByLabelText('Nombre'), 'Pedro Ríos');
    await userEvent.type(screen.getByLabelText('Lo que dice el cliente'), 'Revisión general');
    expect(screen.getByText(/Autorizo al taller a tratar mis datos/)).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Abrir orden' }));
    expect(await screen.findByText('Marca lo que respondió el cliente.')).toBeInTheDocument();
    expect(posts).toHaveLength(0);

    await userEvent.click(screen.getByRole('radio', { name: 'Autoriza' }));
    await userEvent.click(screen.getByRole('button', { name: 'Abrir orden' }));
    expect(await screen.findByText('Orden abierta')).toBeInTheDocument();
    const body = calls.find((c) => c.method === 'POST' && c.path === '/customers')?.body as { autorizacion?: { acepta: boolean } };
    expect(body.autorizacion).toEqual({ acepta: true });
  });

  it('si cambia el celular, se vuelve a preguntar', async () => {
    mockApi(rutas([]));
    renderPage(<NewOrderPage />);
    await userEvent.type(screen.getByLabelText('Placa'), 'TPR418');
    await userEvent.type(await screen.findByLabelText('Celular del cliente'), '3000000761');
    await userEvent.click(await screen.findByRole('radio', { name: 'No autoriza' }));
    expect(screen.getByRole('radio', { name: 'No autoriza' })).toHaveAttribute('aria-checked', 'true');
    await userEvent.type(screen.getByLabelText('Celular del cliente'), '{backspace}2');
    await waitFor(() => expect(screen.getByRole('radio', { name: 'No autoriza' })).toHaveAttribute('aria-checked', 'false'));
  });
});
