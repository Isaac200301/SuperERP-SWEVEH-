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
import { AutomationsPage } from '../automations/AutomationsPage';
import { CustomersPage } from '../customers/CustomersPage';
import { InboxPage } from '../inbox/InboxPage';
import { FloorPage } from '../orders/FloorPage';
import { ReceivePage } from '../orders/FloorMobile';
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

  // Entrega del 27-nov (V-05, V-11; Figma "04 Hoy"): la misma barra para todos los roles.
  it('el dueño ve Hoy, Órdenes, Asistente y Taller; Hoy es el tablero', () => {
    renderPage(<TabBar session={as(dueno)} />, '/panel/tablero');
    expect(labels()).toEqual(['Hoy', 'Órdenes', 'Asistente', 'Taller']);
    expect(screen.getByRole('link', { name: 'Hoy' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: 'Asistente' })).toHaveAttribute('href', '/panel/asistente');
    expect(screen.queryByRole('button', { name: 'Nueva orden' })).not.toBeInTheDocument();
  });

  it('recepción tiene la misma barra', () => {
    renderPage(<TabBar session={as(recepcion)} />);
    expect(labels()).toEqual(['Hoy', 'Órdenes', 'Asistente', 'Taller']);
  });

  it('el mecánico también; sin los números, Hoy es su piso', () => {
    renderPage(<TabBar session={as(mechanic)} />);
    expect(labels()).toEqual(['Hoy', 'Órdenes', 'Asistente', 'Taller']);
    expect(screen.getByRole('link', { name: 'Hoy' })).toHaveAttribute('href', '/panel/piso');
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
        <Route path="/panel/piso/recibir/:id" element={<p>recepción</p>} />
      </Routes>,
    );
    await userEvent.type(screen.getByLabelText('Placa'), 'klm482');
    expect(await screen.findByText(/Laura Gómez/)).toBeInTheDocument();
    expect(await screen.findByText('Ya vino 1 vez · última: OS-0003')).toBeInTheDocument();
    expect(screen.getByText('Falta lo que dice el cliente.')).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText('¿Qué dice el cliente?'), 'Chilla al frenar');
    await userEvent.click(screen.getByRole('button', { name: 'Crear y recibir ahora' }));
    expect(await screen.findByText('recepción')).toBeInTheDocument();
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
    expect(await screen.findByRole('link', { name: /^Clientes/ })).toHaveTextContent('Fichas, vehículos e historial');
    expect(screen.getByRole('link', { name: /^Conversaciones/ })).toBeInTheDocument();
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

describe('Taller del dueño en el celular (Figma 12)', () => {
  const item = (clave: string) => ({ clave, nombre: clave });
  const parametros = {
    usarControlCalidad: false, usarEsperandoRepuesto: false, inventarioExterno: false, ivaPct: 19, preciosConIva: true, tarifaHora: 45000,
    vigenciaDias: 15, descuentoMaxPct: 10, fotosMinimas: 4, kmObligatorio: true, itemsCarro: [item('luces'), item('gato')], itemsMoto: [item('casco')],
  };

  it('lista Clientes, Servicios y Equipo con su subtítulo y resume Mi taller con el interruptor del inventario', async () => {
    mockMobile();
    const calls = mockApi({
      'GET /me': dueno,
      'GET /workshop': {
        tamano: 'pequeno', nombre: 'Servitaller El Pistón', ciudad: 'Medellín', atiende: 'ambos',
        horario: { lun: { abre: '07:30', cierra: '18:00' }, mar: { abre: '07:30', cierra: '18:00' }, sab: { abre: '08:00', cierra: '13:00' } },
      },
      'GET /workshop/settings': parametros,
      'PUT /workshop/settings': { ...parametros, inventarioExterno: true },
    });
    renderPage(<MorePage />);
    expect(await screen.findByRole('link', { name: /^Servicios/ })).toHaveTextContent('Tu lista de precios');
    expect(screen.getByRole('link', { name: /^Equipo/ })).toHaveTextContent('Personas, roles y PIN');
    // Mi taller es la tarjeta de abajo, no una fila de la lista.
    expect(screen.queryByRole('link', { name: /^Mi taller/ })).not.toBeInTheDocument();
    expect(await screen.findByText('Servitaller El Pistón · Medellín · carros y motos')).toBeInTheDocument();
    expect(screen.getByText('Lun y mar 7:30 a. m. – 6:00 p. m. · sáb 8:00 a. m. – 1:00 p. m.')).toBeInTheDocument();
    expect(screen.getByText('2 ítems de carro · 1 de moto')).toBeInTheDocument();
    const sw = screen.getByRole('switch', { name: /Llevo el inventario en otro programa/ });
    expect(sw).not.toBeChecked();
    await userEvent.click(sw);
    await waitFor(() => expect(calls.find((c) => c.method === 'PUT')?.body).toMatchObject({ inventarioExterno: true, ivaPct: 19 }));
  });

  it('un servicio de la lista lleva a su pantalla', async () => {
    mockMobile();
    mockApi({ 'GET /me': dueno, 'GET /workshop': { tamano: 'pequeno', nombre: 'T', horario: {} }, 'GET /workshop/settings': parametros });
    renderPage(<MorePage />);
    expect(await screen.findByRole('link', { name: /^Servicios/ })).toHaveAttribute('href', '/panel/servicios');
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

describe('Piso en el celular (HU-23, HU-81)', () => {
  it('pone arriba lo siguiente por recibir y cada orden trae su próximo paso', async () => {
    mockMobile();
    const calls = mockApi({
      'GET /me': mechanic,
      'GET /orders/mine': {
        ordenes: [
          orden({ id: 'o1', placa: 'KLM482', estado: 'por_recibir' }),
          orden({ id: 'o2', placa: 'HTR315', estado: 'en_diagnostico' }),
          orden({ id: 'o3', placa: 'GBW118', estado: 'esperando_aprobacion' }),
        ],
      },
      'GET /workshop/settings': { usarControlCalidad: false, usarEsperandoRepuesto: false },
      'POST /orders/{id}/transition': orden({ id: 'o2', estado: 'esperando_aprobacion' }),
    });
    renderPage(<FloorPage />);
    expect(await screen.findByText('LO SIGUIENTE')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Recibir vehículo' })).toHaveAttribute('href', '/panel/piso/recibir/o1');
    expect(screen.getByText('Espera la aprobación del cliente')).toBeInTheDocument();
    expect(screen.queryByText(/\$/)).not.toBeInTheDocument(); // RF-21: sin costos
    await userEvent.click(screen.getByRole('button', { name: 'Pasar a Esperando aprobación' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'POST')?.path).toBe('/orders/o2/transition'));
    expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ estado: 'esperando_aprobacion', version: 3 });
  });

  it('Recibir sin vehículos pendientes lo dice', async () => {
    mockMobile();
    mockApi({ 'GET /orders/mine': { ordenes: [orden({ estado: 'en_reparacion' })] } });
    renderPage(
      <Routes>
        <Route path="/panel/piso/recibir" element={<ReceivePage />} />
      </Routes>,
      '/panel/piso/recibir',
    );
    expect(await screen.findByText('No tienes vehículos por recibir.')).toBeInTheDocument();
  });
});

describe('Automatizaciones en el celular (HU-40, HU-41)', () => {
  it('muestra las cifras de la semana y filtra encendidas y apagadas', async () => {
    mockMobile();
    const au = (codigo: string, nombre: string, activa: boolean, ejecucionesSemana: number) => ({
      codigo, nombre, disparador: 'Cuando…', activa, proximamente: false, enviaMensaje: false, editable: false, texto: '', datos: [],
      ejecucionesSemana, costo: '1 mensaje',
    });
    mockApi({
      'GET /me': dueno,
      'GET /automations': { automatizaciones: [au('AU-01', 'Vehículo recibido', true, 30), au('AU-07', 'Sin recoger', false, 7)] },
      'GET /automations/runs': { ejecuciones: [] },
      'GET /inbox/stats': { pendientes: 4, enviados: 0, editados: 0, porcentajeEditados: 0 },
    });
    renderPage(<AutomationsPage />);
    expect(await screen.findByText('37')).toBeInTheDocument();
    expect(await screen.findByText('4')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('radio', { name: /Apagadas/ }));
    expect(screen.getByText('Sin recoger')).toBeInTheDocument();
    expect(screen.queryByText('Vehículo recibido')).not.toBeInTheDocument();
  });
});
