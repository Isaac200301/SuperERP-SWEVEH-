import { renderHook, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { Route, Routes } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Sesion } from '../../shared/api/client';
import { BottomSheet } from '../../shared/ui/BottomSheet';
import { Tabs } from '../../shared/ui/Tabs';
import { useIsMobile } from '../../shared/ui/useIsMobile';
import { mechanic, mockApi, orden, owner, renderPage } from '../../test/api';
import { mockMobile } from '../../test/mobile';
import { CustomersPage } from '../customers/CustomersPage';
import { InboxPage } from '../inbox/InboxPage';
import { NewOrderSheet } from '../orders/NewOrderSheet';
import { OrderDetailPage } from '../orders/OrderDetailPage';
import { OrdersPage } from '../orders/OrdersPage';
import { MorePage } from './MorePage';
import { TabBar } from './TabBar';

afterEach(() => vi.unstubAllGlobals());

const dueno = { ...owner, permisos: [...owner.permisos, 'order:create', 'kpi:read', 'inbox:manage', 'customer:read_all', 'automation:manage'] };
const recepcion = { ...owner, roles: ['recepcion'], permisos: ['order:create', 'order:read', 'inbox:manage', 'customer:read_all', 'conversation:read'] };
const as = (s: unknown) => s as Sesion;

describe('useIsMobile', () => {
  it('en escritorio (sin matchMedia) es false; con pantalla angosta es true', () => {
    expect(renderHook(() => useIsMobile()).result.current).toBe(false);
    mockMobile();
    expect(renderHook(() => useIsMobile()).result.current).toBe(true);
  });
});

describe('TabBar (mockups móviles)', () => {
  const labels = () => screen.getAllByRole('link').map((l) => l.textContent);

  it('el dueño ve Hoy, Órdenes, Nueva, Revisar con su contador y Taller', () => {
    renderPage(<TabBar session={as(dueno)} pending={2} />, '/panel/tablero');
    expect(labels()).toEqual(['Hoy', 'Órdenes', '2 por revisarRevisar', 'Taller']);
    expect(screen.getByRole('button', { name: 'Nueva orden' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Hoy' })).toHaveAttribute('aria-current', 'page');
  });

  it('recepción no ve los números (HU-82): Órdenes, Nueva, Revisar y Más', () => {
    renderPage(<TabBar session={as(recepcion)} pending={0} />);
    expect(labels()).toEqual(['Órdenes', 'Revisar', 'Más']);
  });

  it('el mecánico solo ve sus órdenes y Yo, sin crear órdenes (HU-81)', () => {
    renderPage(<TabBar session={as(mechanic)} pending={0} />);
    expect(labels()).toEqual(['Órdenes', 'Yo']);
    expect(screen.getByRole('link', { name: 'Órdenes' })).toHaveAttribute('href', '/panel/piso');
    expect(screen.queryByRole('button', { name: 'Nueva orden' })).not.toBeInTheDocument();
  });
});

describe('BottomSheet y Tabs', () => {
  function Sheet() {
    const [open, setOpen] = useState(true);
    return (
      <BottomSheet open={open} onClose={() => setOpen(false)} title="Nueva orden">
        <p>contenido</p>
      </BottomSheet>
    );
  }

  it('la hoja es un diálogo con título y se cierra con Escape', async () => {
    renderPage(<Sheet />);
    expect(screen.getByRole('dialog', { name: 'Nueva orden' })).toBeInTheDocument();
    await userEvent.keyboard('{Escape}');
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('las pestañas se mueven con las flechas (DR-14)', async () => {
    function T() {
      const [v, setV] = useState<'a' | 'b'>('a');
      return (
        <Tabs label="Secciones" value={v} onChange={setV} tabs={[{ value: 'a', label: 'Estado' }, { value: 'b', label: 'WhatsApp' }]}>
          <p>{v}</p>
        </Tabs>
      );
    }
    renderPage(<T />);
    screen.getByRole('tab', { name: 'Estado' }).focus();
    await userEvent.keyboard('{ArrowRight}');
    expect(screen.getByRole('tab', { name: 'WhatsApp' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tabpanel')).toHaveTextContent('b');
  });
});

describe('Órdenes en el celular', () => {
  it('muestra tarjetas y los chips filtran por estado', async () => {
    mockMobile();
    mockApi({
      'GET /orders': {
        ordenes: [
          orden(),
          orden({ id: 'o2', codigo: 'OS-0008', placa: 'GBW118', estado: 'esperando_aprobacion' }),
          orden({ id: 'o3', codigo: 'OS-0009', placa: 'JNX907', estado: 'entregado' }),
        ],
      },
    });
    renderPage(<OrdersPage />);
    expect(await screen.findByText('OS-0007')).toBeInTheDocument();
    expect(screen.getByText('2 abiertas · 1 espera al cliente')).toBeInTheDocument();
    expect(screen.queryByText('OS-0009')).not.toBeInTheDocument(); // entregada no es abierta
    await userEvent.click(screen.getByRole('radio', { name: /Esperando aprobación/ }));
    expect(screen.getByText('OS-0008')).toBeInTheDocument();
    expect(screen.queryByText('OS-0007')).not.toBeInTheDocument();
    expect(screen.queryByRole('table')).not.toBeInTheDocument();
  });
});

describe('Orden en el celular', () => {
  it('va en pestañas y la acción principal queda abajo (DR-05)', async () => {
    mockMobile();
    mockApi({
      'GET /orders/{id}': orden({ historial: [{ desde: null, hacia: 'por_recibir', usuarioId: 'u1', ocurridoEn: '2026-11-02T15:00:00Z' }] }),
      'GET /orders/{id}/messages': { mensajes: [] },
      'GET /vehicles/{id}/history': { ordenes: [] },
    });
    renderPage(
      <Routes>
        <Route path="/panel/ordenes/:id" element={<OrderDetailPage />} />
      </Routes>,
      '/panel/ordenes/o1',
    );
    expect(await screen.findByRole('tab', { name: 'Estado' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('region', { name: 'Mover la orden' })).toHaveTextContent('Pasar a Esperando aprobación');
    await userEvent.click(screen.getByRole('tab', { name: 'Historial' }));
    expect(await screen.findByText('Es la primera vez que viene.')).toBeInTheDocument();
    expect(screen.getByText('Cambios de estado')).toBeInTheDocument();
  });
});

describe('Hoja Nueva orden (HU-16)', () => {
  it('un vehículo que ya vino abre la orden con lo que dice el cliente', async () => {
    const calls = mockApi({
      'GET /vehicles/by-plate/{placa}': { id: 'v1', placa: 'KLM482', tipo: 'carro', marca: 'Chevrolet', linea: 'Spark', motorPendiente: false, version: 1 },
      'GET /search': {
        vehiculos: [{ id: 'v1', placa: 'KLM482', tipo: 'carro', descripcion: 'Chevrolet Spark 2015', clienteId: 'c1', cliente: 'Laura Gómez', ordenAbiertaId: null, ordenAbierta: null, estadoOrden: null }],
        clientes: [],
      },
      'GET /vehicles/{id}/history': { ordenes: [{ orden: orden({ codigo: 'OS-0003', estado: 'entregado' }) }] },
      'POST /orders': orden({ id: 'o9', estado: 'por_recibir' }),
    });
    renderPage(
      <Routes>
        <Route path="/" element={<NewOrderSheet open onClose={() => undefined} />} />
        <Route path="/panel/ordenes/:id" element={<p>orden abierta</p>} />
      </Routes>,
    );
    await userEvent.type(screen.getByLabelText('Placa'), 'klm482');
    expect(await screen.findByText(/Laura Gómez/)).toBeInTheDocument();
    expect(await screen.findByText('Ya vino 1 vez · última: OS-0003')).toBeInTheDocument();
    expect(screen.getByText('Falta lo que dice el cliente.')).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText('¿Qué dice el cliente?'), 'Chilla al frenar');
    await userEvent.click(screen.getByRole('button', { name: 'Crear y recibir ahora' }));
    expect(await screen.findByText('orden abierta')).toBeInTheDocument();
    expect(calls.find((c) => c.method === 'POST' && c.path === '/orders')?.body).toMatchObject({
      vehiculoId: 'v1',
      sintoma: 'Chilla al frenar',
      estadoInicial: 'por_recibir',
    });
  });

  it('un vehículo nuevo lleva a la pantalla completa con la placa', async () => {
    mockApi({ 'GET /vehicles/by-plate/{placa}': { status: 404, body: { detail: 'no' } } });
    renderPage(
      <Routes>
        <Route path="/" element={<NewOrderSheet open onClose={() => undefined} />} />
        <Route path="/panel/ordenes/nueva" element={<p>pantalla completa</p>} />
      </Routes>,
    );
    await userEvent.type(screen.getByLabelText('Placa'), 'ZZZ999');
    await userEvent.click(await screen.findByRole('button', { name: 'Registrar vehículo nuevo' }));
    expect(await screen.findByText('pantalla completa')).toBeInTheDocument();
  });
});

describe('Más (celular)', () => {
  it('lista solo las secciones del rol que no están en la barra', async () => {
    mockMobile();
    mockApi({ 'GET /me': recepcion });
    renderPage(<MorePage />);
    expect(await screen.findByRole('link', { name: 'Clientes' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Conversaciones' })).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole('link', { name: 'Órdenes' })).not.toBeInTheDocument());
    expect(screen.queryByRole('link', { name: 'Mi taller' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Cerrar sesión' })).toBeInTheDocument();
  });
});

describe('Por revisar en el celular (HU-62, HU-64)', () => {
  const borrador = (id: string, celular: string, mensaje: string) => ({
    id, conversacionId: `cv-${id}`, celular, mensajeCliente: mensaje, intencion: 'cotizar', propuesto: `Hola, ${mensaje}`,
    fuentes: [{ campo: 'precio', valor: '$40.000', origen: 'lista de precios' }], estado: 'pendiente', origen: 'plantilla',
    creadoEn: '2026-11-10T15:00:00Z',
  });

  it('muestra un borrador a la vez, con sus fuentes, y Enviar usa el mismo endpoint', async () => {
    mockMobile();
    const calls = mockApi({
      'GET /me': dueno,
      'GET /inbox': { borradores: [borrador('d1', '+573001112233', 'chilla al frenar'), borrador('d2', '+573004445566', 'cambio de aceite')] },
      'GET /inbox/stats': { pendientes: 2, enviados: 0, editados: 0, porcentajeEditados: 0 },
      'GET /services/proposals': { propuestas: [] },
      'GET /assistant/settings': { autoEnvio: false },
      'POST /inbox/{id}': {},
    });
    renderPage(<InboxPage />);
    expect(await screen.findByText('chilla al frenar')).toBeInTheDocument();
    expect(screen.queryByText('cambio de aceite')).not.toBeInTheDocument();
    expect(screen.getByText('lista de precios')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('radio', { name: /300 444 5566/ }));
    expect(await screen.findByText('cambio de aceite')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Enviar' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'POST')?.path).toBe('/inbox/d2'));
    expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ decision: 'enviar' });
  });
});

describe('Taller del dueño en el celular', () => {
  it('muestra los números con "¿Qué significa?" y las barras llevan a las órdenes filtradas', async () => {
    mockMobile();
    mockApi({
      'GET /me': dueno,
      'GET /workshop': { tamano: 'pequeno' },
      'GET /workshop/settings': { usarControlCalidad: false, usarEsperandoRepuesto: false },
      'GET /dashboard': {
        ejemplo: false,
        kpis: [{ codigo: 'abiertas', titulo: 'Órdenes abiertas', valor: 7, texto: '7', detalle: '', queSignifica: 'Son los vehículos que no se han entregado.', ejemplo: false }],
        porEstado: { esperando_aprobacion: 1, en_reparacion: 2 },
      },
    });
    renderPage(<MorePage />);
    await userEvent.click(await screen.findByRole('button', { name: '¿Qué significa?' }));
    expect(screen.getByRole('dialog', { name: 'Órdenes abiertas' })).toHaveTextContent('Son los vehículos que no se han entregado.');
    await userEvent.click(screen.getByRole('button', { name: 'Entendido' }));
    expect(screen.getByRole('link', { name: 'En reparación: 2' })).toHaveAttribute('href', '/panel/ordenes?estado=en_reparacion');
  });
});

describe('Clientes en el celular', () => {
  it('abre la ficha del cliente con su autorización y vehículos en una hoja', async () => {
    mockMobile();
    mockApi({
      'GET /me': dueno,
      'GET /customers': { clientes: [{ id: 'c1', nombre: 'Laura Gómez', celular: '3001234567', correo: null, autorizacionEstado: 'autorizada', creadoEn: '2026-11-01T00:00:00Z' }] },
      'GET /customers/{id}/vehicles': { vehiculos: [{ id: 'v1', placa: 'KLM482', tipo: 'carro', marca: 'Chevrolet', linea: 'Spark', motorPendiente: false }] },
    });
    renderPage(<CustomersPage />);
    await userEvent.click(await screen.findByRole('button', { name: /Laura Gómez/ }));
    const ficha = screen.getByRole('dialog', { name: 'Laura Gómez' });
    expect(ficha).toHaveTextContent('Autorizó sus datos');
    expect(await screen.findByText('KLM482')).toBeInTheDocument();
  });
});
