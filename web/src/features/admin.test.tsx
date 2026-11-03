import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Route, Routes } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Sesion } from '../shared/api/client';
import { mechanic, mockApi, orden, owner, renderPage } from '../test/api';
import { CatalogPage } from './catalog/CatalogPage';
import { OrderDetailPage } from './orders/OrderDetailPage';
import { PanelLayout } from './panel/PanelLayout';
import { TeamPage } from './team/TeamPage';
import { WorkshopPage } from './workshop/WorkshopPage';

afterEach(() => vi.unstubAllGlobals());

describe('CatalogPage (HU-07, HU-32, HU-35)', () => {
  it('crea servicios y repuestos, activa y registra entradas', async () => {
    const calls = mockApi({
      'GET /services': { servicios: [{ id: 's1', nombre: 'Alineación', tipoVehiculo: 'carro', precio: null, duracionMin: null, activo: true, version: 2 }] },
      'POST /services': { status: 201, body: {} },
      'PATCH /services/{id}': {},
      'GET /parts': { repuestos: [{ id: 'p1', referencia: 'PF-20', nombre: 'Pastillas', tipoVehiculo: 'carro', precioVenta: 65000, existencia: 1, minimo: 2, bajoMinimo: true, activo: true, version: 1 }] },
      'POST /parts': { status: 201, body: {} },
      'POST /parts/{id}/entries': {},
    });
    renderPage(<CatalogPage />);
    expect(await screen.findByText('por confirmar')).toBeInTheDocument();
    expect(await screen.findByText(/bajo el mínimo/)).toBeInTheDocument();

    expect(screen.getByText('Escribe el nombre del servicio.')).toBeInTheDocument();
    await userEvent.type(screen.getAllByLabelText('Nombre')[0] as HTMLElement, 'Cambio de aceite');
    await userEvent.type(screen.getByLabelText('Precio (COP)'), '80.000');
    await userEvent.type(screen.getByLabelText('Duración (min)'), '45');
    await userEvent.click(screen.getByRole('button', { name: 'Agregar servicio' }));
    await userEvent.click(screen.getByRole('button', { name: 'Desactivar' }));

    await userEvent.type(screen.getByLabelText('Referencia'), 'FA-10');
    await userEvent.type(screen.getAllByLabelText('Nombre')[1] as HTMLElement, 'Filtro');
    expect(screen.getByText('Escribe el precio de venta.')).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText('Precio de venta (COP)'), '20000');
    await userEvent.type(screen.getByLabelText('Costo (COP)'), '12000');
    await userEvent.type(screen.getByLabelText('Existencia'), '3');
    await userEvent.type(screen.getByLabelText('Mínimo'), '1');
    await userEvent.click(screen.getByRole('button', { name: 'Agregar repuesto' }));

    vi.spyOn(window, 'prompt').mockReturnValue('4');
    await userEvent.click(screen.getByRole('button', { name: 'Registrar entrada' }));

    await waitFor(() => expect(calls.some((c) => c.path === '/parts/p1/entries')).toBe(true));
    const body = (m: string, p: string) => calls.find((c) => c.method === m && c.path === p)?.body;
    expect(body('POST', '/services')).toEqual({ nombre: 'Cambio de aceite', tipoVehiculo: 'ambos', precio: 80000, duracionMin: 45 });
    expect(body('PATCH', '/services/s1')).toEqual({ activo: false, version: 2 });
    expect(body('POST', '/parts')).toEqual({ referencia: 'FA-10', nombre: 'Filtro', tipoVehiculo: 'ambos', costo: 12000, precioVenta: 20000, existencia: 3, minimo: 1 });
    expect(body('POST', '/parts/p1/entries')).toEqual({ cantidad: 4 });
  });
});

describe('TeamPage (HU-80, HU-84)', () => {
  it('agrega un mecánico y desactiva a quien se va; al dueño no', async () => {
    const calls = mockApi({
      'GET /users': {
        usuarios: [
          { id: 'u1', nombre: 'Rosa', celular: '3001234567', rol: 'dueno', activo: true, creadoEn: '2026-11-01T00:00:00Z' },
          { id: 'u2', nombre: 'Andrés', celular: '3007654321', rol: 'mecanico', activo: true, creadoEn: '2026-11-01T00:00:00Z' },
        ],
      },
      'POST /users': { status: 409, body: { detail: 'Ese celular ya está en el equipo.' } },
      'PATCH /users/{id}': {},
    });
    renderPage(<TeamPage />);
    expect(await screen.findByText('Andrés')).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: 'Desactivar' })).toHaveLength(1);
    await userEvent.type(screen.getByLabelText('Nombre'), 'Luis');
    await userEvent.type(screen.getByLabelText('Celular'), '300 111');
    expect(screen.getByText('El celular debe tener 10 dígitos.')).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText('Celular'), ' 2233');
    await userEvent.selectOptions(screen.getByLabelText('Rol'), 'recepcion');
    await userEvent.click(screen.getByRole('button', { name: 'Agregar al equipo' }));
    expect(await screen.findByText('Ese celular ya está en el equipo.')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Desactivar' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'PATCH')?.body).toEqual({ activo: false }));
    expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ celular: '3001112233', nombre: 'Luis', rol: 'recepcion' });
  });
});

describe('WorkshopPage (HU-03, HU-08, HU-89)', () => {
  it('termina el arranque con el contrato y guarda el horario', async () => {
    const calls = mockApi({
      'GET /workshop': {
        id: 't1', nombre: 'El Pistón', atiende: '', especialidades: [], ciudad: '', tamano: '', contratoVersion: null, contratoAceptadoEn: null,
        onboardingCompleto: false, horario: { lun: { abre: '08:00', cierra: '17:00' } },
      },
      'POST /workshop/onboarding': {},
      'PUT /workshop/schedule': {},
    });
    renderPage(<WorkshopPage />);
    expect(await screen.findByText('Escoge qué vehículos atiendes.')).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText('Ciudad'), 'Medellín');
    await userEvent.click(screen.getByRole('radio', { name: 'Motos' }));
    expect(screen.getByText('Para continuar acepta el contrato.')).toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText('¿Cuántos trabajan?'), '2a5');
    await userEvent.click(screen.getByLabelText('Frenos y suspensión'));
    await userEvent.click(screen.getByLabelText(/^Sistema eléctrico/));
    await userEvent.click(screen.getByLabelText(/^Sistema eléctrico/));
    await userEvent.click(screen.getByLabelText(/Acepto el contrato/));
    await userEvent.click(screen.getByRole('button', { name: 'Armar mi taller' }));

    await userEvent.click(screen.getByLabelText('Lunes'));
    await userEvent.click(screen.getByLabelText('Sábado'));
    await userEvent.clear(screen.getByLabelText('Sábado cierra'));
    await userEvent.type(screen.getByLabelText('Sábado cierra'), '12:00');
    await userEvent.click(screen.getByRole('button', { name: 'Guardar horario' }));
    expect(await screen.findByText('Horario guardado.')).toBeInTheDocument();

    expect(calls.find((c) => c.method === 'POST')?.body).toEqual({
      nombre: 'El Pistón', atiende: 'moto', especialidades: ['frenos_suspension'], ciudad: 'Medellín', tamano: '2a5', contratoAceptado: true,
    });
    const h = (calls.find((c) => c.method === 'PUT')?.body as { horario: Record<string, unknown> }).horario;
    expect(h.lun).toBeNull();
    expect(h.sab).toEqual({ abre: '08:00', cierra: '12:00' });
  });
});

describe('OrderDetailPage', () => {
  it('muestra la orden con sus paneles y el error si no existe', async () => {
    mockApi({
      'GET /me': owner,
      'GET /orders/{id}': orden({ estado: 'por_recibir', vehiculo: { ...orden().vehiculo, motorPendiente: true } }),
      'GET /orders/{id}/checklist': { tipo: 'carro', kilometraje: null, combustible: null, accesorios: [], salvedades: [], ladosConFoto: [], ladosPedidos: [], falta: [], completadoEn: null },
      'GET /orders/{id}/evidence': { evidencias: [] },
    });
    const ui = (
      <Routes>
        <Route path="/panel/ordenes/:id" element={<OrderDetailPage />} />
      </Routes>
    );
    const { unmount } = renderPage(ui, '/panel/ordenes/o1');
    expect(await screen.findByText(/Motor pendiente/)).toBeInTheDocument();
    expect(screen.queryByText(/Cotización/)).not.toBeInTheDocument();
    unmount();
    mockApi({ 'GET /orders/{id}': { status: 404, body: { detail: 'No encontramos esa orden.' } } });
    renderPage(ui, '/panel/ordenes/o1');
    expect(await screen.findByText('No encontramos esa orden.')).toBeInTheDocument();
  });
});

describe('PanelLayout (HU-81)', () => {
  it('el mecánico no ve las secciones de configuración', () => {
    const { unmount } = renderPage(<PanelLayout session={mechanic as unknown as Sesion} />);
    expect(screen.getByRole('link', { name: 'Piso' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Equipo' })).not.toBeInTheDocument();
    unmount();
    renderPage(<PanelLayout session={owner as unknown as Sesion} />);
    expect(screen.getByRole('link', { name: 'Equipo' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Mi taller' })).toBeInTheDocument();
  });

  it('si el taller es de una sola persona no muestra Equipo (HU-83)', () => {
    renderPage(<PanelLayout session={owner as unknown as Sesion} soloYo />);
    expect(screen.queryByRole('link', { name: 'Equipo' })).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Mi taller' })).toBeInTheDocument();
  });
});
