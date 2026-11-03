import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Route, Routes } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { RequireSession } from '../app/RequireSession';
import { mockApi, orden, owner, renderPage } from '../test/api';
import { NewOrderPage } from './orders/NewOrderPage';
import { OrderDetailPage } from './orders/OrderDetailPage';
import { OnboardingPage } from './workshop/OnboardingPage';

afterEach(() => vi.unstubAllGlobals());

const taller = (over: Record<string, unknown> = {}) => ({
  id: 't1', nombre: 'El Pistón', atiende: '', especialidades: [], ciudad: '', tamano: '', contratoVersion: null, contratoAceptadoEn: null,
  onboardingCompleto: false, horario: {}, version: 1, ...over,
});

describe('Arranque del taller (HU-03, HU-04)', () => {
  it('un taller sin arranque entra primero a /panel/arranque', async () => {
    mockApi({ 'GET /me': owner, 'GET /workshop': taller() });
    renderPage(
      <Routes>
        <Route path="/panel" element={<RequireSession />}>
          <Route path="ordenes" element={<p>Lista de órdenes</p>} />
          <Route path="arranque" element={<p>Pantalla de arranque</p>} />
        </Route>
      </Routes>,
      '/panel/ordenes',
    );
    expect(await screen.findByText('Pantalla de arranque')).toBeInTheDocument();
  });

  it('pide el tipo de vehículo y luego deja revisar, renombrar, quitar y agregar servicios', async () => {
    let armado = false;
    const calls = mockApi({
      'GET /workshop': () => taller(armado ? { atiende: 'carro', onboardingCompleto: true, contratoAceptadoEn: '2026-11-03T00:00:00Z' } : {}),
      'POST /workshop/onboarding': () => {
        armado = true;
        return {};
      },
      'GET /services': { servicios: [{ id: 's1', nombre: 'Cambio de aceite', tipoVehiculo: 'carro', precio: null, duracionMin: null, activo: true, version: 1 }] },
      'PATCH /services/{id}': {},
      'DELETE /services/{id}': { status: 204 },
      'POST /services': { status: 201, body: {} },
    });
    renderPage(<OnboardingPage />);

    expect(await screen.findByText('PASO 2 DE 3')).toBeInTheDocument();
    expect(screen.getByText('Escoge qué vehículos atiendes.')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('radio', { name: 'Carros' }));
    await userEvent.click(screen.getByLabelText(/Acepto el contrato/));
    await userEvent.click(screen.getByRole('button', { name: 'Armar mi taller' }));

    expect(await screen.findByText('Tu taller ya está armado')).toBeInTheDocument();
    expect(await screen.findByText(/checklist de recepción de carro/)).toBeInTheDocument();
    const nombre = await screen.findByLabelText('Nombre de Cambio de aceite');
    await userEvent.clear(nombre);
    await userEvent.type(nombre, 'Cambio de aceite y filtro');
    await userEvent.tab();
    await userEvent.click(screen.getByRole('button', { name: 'Quitar Cambio de aceite' }));
    await userEvent.type(screen.getByLabelText('Agregar un servicio'), 'Sincronización');
    await userEvent.click(screen.getByRole('button', { name: 'Agregar' }));

    await waitFor(() => expect(calls.some((c) => c.method === 'POST' && c.path === '/services')).toBe(true));
    const body = (m: string, p: string) => calls.find((c) => c.method === m && c.path === p)?.body;
    expect((body('POST', '/workshop/onboarding') as { atiende: string }).atiende).toBe('carro');
    expect(body('PATCH', '/services/s1')).toEqual({ nombre: 'Cambio de aceite y filtro', version: 1 });
    expect(calls.some((c) => c.method === 'DELETE' && c.path === '/services/s1')).toBe(true);
    expect(body('POST', '/services')).toEqual({ nombre: 'Sincronización', tipoVehiculo: 'carro' });
  });
});

describe('Primera orden (HU-05)', () => {
  it('"Usar ejemplo" crea una orden marcada como ejemplo y muestra la vista previa del aviso', async () => {
    const calls = mockApi({
      'GET /me': owner,
      'GET /workshop': taller({ atiende: 'moto', onboardingCompleto: true }),
      'GET /vehicles/by-plate/{placa}': { status: 404, body: { detail: 'No existe' } },
      'GET /customers/by-phone/{celular}': { status: 404, body: { detail: 'No existe' } },
      'POST /customers': { status: 201, body: { cliente: { id: 'c1', nombre: 'Cliente de ejemplo', celular: '3000000000' } } },
      'POST /vehicles': { status: 201, body: { vehiculo: { id: 'v1', placa: 'EJM12A', tipo: 'moto', motorPendiente: true } } },
      'POST /orders': { status: 201, body: orden({ id: 'o9', placa: 'EJM12A', ejemplo: true }) },
      'GET /orders/{id}': orden({ id: 'o9', placa: 'EJM12A', estado: 'por_recibir', ejemplo: true, cliente: { id: 'c1', nombre: 'Cliente de ejemplo' },
        vehiculo: { ...orden().vehiculo, tipo: 'moto', placa: 'EJM12A' } }),
      'GET /orders/{id}/checklist': { tipo: 'moto', kilometraje: null, combustible: null, accesorios: [], salvedades: [], ladosConFoto: [], ladosPedidos: [], falta: [], completadoEn: null },
      'GET /orders/{id}/evidence': { evidencias: [] },
    });
    renderPage(
      <Routes>
        <Route path="/panel/ordenes/nueva" element={<NewOrderPage />} />
        <Route path="/panel/ordenes/:id" element={<OrderDetailPage />} />
      </Routes>,
      '/panel/ordenes/nueva?primera=1',
    );
    expect(await screen.findByText('Tu primera orden')).toBeInTheDocument();
    await waitFor(() => expect(calls.some((c) => c.path === '/workshop')).toBe(true));
    await userEvent.click(screen.getByRole('button', { name: 'Usar ejemplo' }));
    expect(screen.getByLabelText('Placa')).toHaveValue('EJM12A');
    await userEvent.click(await screen.findByRole('button', { name: 'Abrir orden' }));

    expect(await screen.findByText('Así le llegaría el aviso a tu cliente')).toBeInTheDocument();
    expect(screen.getByText(/no cuenta en tus números/)).toBeInTheDocument();
    expect((calls.find((c) => c.method === 'POST' && c.path === '/orders')?.body as { ejemplo?: boolean }).ejemplo).toBe(true);
  });
});
