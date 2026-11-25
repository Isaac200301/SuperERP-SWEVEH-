import { act, fireEvent, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Route, Routes } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Orden } from '../../shared/api/client';
import { mockApi, orden, owner, renderPage } from '../../test/api';
import { ClientQuotePage } from './ClientQuotePage';
import { QuotePanel } from './OrderWork';

afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

// R7 (V-10, HU-109, HU-29, HU-86, ADR-0043): la cotización en pantalla completa y el PDF. Datos Ficticio.

const linea = (over: Record<string, unknown> = {}) => ({
  tipo: 'servicio', descripcion: 'Alineación y balanceo', cantidad: 1, precioUnitario: 60000, subtotal: 60000, descuentoPct: 0, descuento: 0, aprobada: null, ...over,
});
const partida = linea({
  tipo: 'partida', descripcion: 'Frenos completos', precioUnitario: 480000, subtotal: 480000, descuentoPct: 10, descuento: 48000,
  detalle: [{ descripcion: 'Pastillas delanteras', cantidad: 2 }, { descripcion: 'Rectificar discos', cantidad: 1 }],
});
const vista = (cot: Record<string, unknown> = {}, aut: Record<string, unknown> = {}) => ({
  documento: {
    taller: 'Servitaller El Pistón', orden: 'OS-0044', placa: 'JNX907', vehiculo: 'Mazda 2', cliente: 'Marta Ejemplo',
    cotizacion: { numero: 1, estado: 'enviada', lineas: [partida, linea()], subtotal: 540000, descuento: 48000, ivaPct: 19, iva: 78555, preciosConIva: true, total: 492000,
      enviadaEn: '2026-11-25T15:00:00Z', venceEn: '2099-12-10T15:00:00Z', vencida: false, ...cot },
  },
  autorizacion: { clienteId: 'c1', estado: 'autorizada', texto: 'Autorizo al taller a tratar mis datos.', ...aut },
});
const cotizacionOK = { cotizacion: { id: 'q1', numero: 1, estado: 'aprobada', total: 492000, lineas: [], version: 3 } };

function pantalla() {
  return renderPage(
    <Routes>
      <Route path="/panel/ordenes/:id/cliente" element={<ClientQuotePage />} />
      <Route path="/panel/ordenes/:id" element={<p>Detalle de la orden</p>} />
    </Routes>,
    '/panel/ordenes/o1/cliente',
  );
}

describe('Cotización para el cliente (V-10)', () => {
  it('muestra el taller, el vehículo, las partidas con su detalle, los descuentos y el total', async () => {
    mockApi({ 'GET /me': owner, 'GET /orders/{id}/quote/client-view': vista() });
    pantalla();
    expect(await screen.findByRole('heading', { name: 'Servitaller El Pistón' })).toBeInTheDocument();
    expect(screen.getByText('Mazda 2 ·', { exact: false })).toBeInTheDocument();
    expect(screen.getByText('Frenos completos')).toBeInTheDocument();
    expect(screen.getByLabelText('Detalle de Frenos completos')).toHaveTextContent('Pastillas delanteras × 2');
    expect(screen.getByText(/Descuento 10 %/)).toBeInTheDocument();
    expect(screen.getByText('Total con IVA incluido')).toBeInTheDocument();
    expect(screen.getByText(/492\.000/, { selector: 'dd' })).toBeInTheDocument();
    // Sin menú del panel: solo la cotización y sus acciones.
    expect(screen.queryByRole('navigation')).not.toBeInTheDocument();
  });

  it('no muestra detalle cuando la API no lo manda, ni nada de gastos o costos', async () => {
    mockApi({ 'GET /me': owner, 'GET /orders/{id}/quote/client-view': vista({ lineas: [{ ...partida, detalle: undefined }] }) });
    pantalla();
    await screen.findByText('Frenos completos');
    expect(screen.queryByLabelText('Detalle de Frenos completos')).not.toBeInTheDocument();
    expect(screen.queryByText(/gast|costo|nota interna/i)).not.toBeInTheDocument();
  });

  it('"El cliente aprobó" registra la respuesta por el mostrador cuando ya autorizó sus datos', async () => {
    const calls = mockApi({
      'GET /me': owner, 'GET /orders/{id}/quote/client-view': vista(), 'POST /orders/{id}/quote/response': cotizacionOK,
    });
    pantalla();
    await userEvent.click(await screen.findByRole('button', { name: 'El cliente aprobó' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ decision: 'aprobar', canal: 'mostrador' }));
    expect(calls.some((c) => c.path.includes('/consent'))).toBe(false);
  });

  it('pide la autorización de datos en la misma pantalla antes de aprobar (HU-86) y la guarda con la aprobación', async () => {
    const calls = mockApi({
      'GET /me': owner, 'GET /orders/{id}/quote/client-view': vista({}, { estado: 'pendiente' }),
      'POST /customers/{id}/consent': { id: 'c1' }, 'POST /orders/{id}/quote/response': cotizacionOK,
    });
    pantalla();
    await userEvent.click(await screen.findByRole('button', { name: 'El cliente aprobó' }));
    expect(screen.getByText('Autorizo al taller a tratar mis datos.')).toBeInTheDocument();
    expect(calls.some((c) => c.path.includes('/quote/response'))).toBe(false); // todavía no se aprueba
    await userEvent.click(screen.getByRole('button', { name: 'El cliente autoriza y aprueba' }));
    await waitFor(() => expect(calls.filter((c) => c.method === 'POST').map((c) => c.path)).toEqual(['/customers/c1/consent', '/orders/o1/quote/response']));
    expect(calls.find((c) => c.path.includes('/consent'))?.body).toEqual({ acepta: true });
  });

  it('si el cliente no autoriza, igual se registra su aprobación, con la negativa como prueba', async () => {
    const calls = mockApi({
      'GET /me': owner, 'GET /orders/{id}/quote/client-view': vista({}, { estado: 'pendiente' }),
      'POST /customers/{id}/consent': { id: 'c1' }, 'POST /orders/{id}/quote/response': cotizacionOK,
    });
    pantalla();
    await userEvent.click(await screen.findByRole('button', { name: 'El cliente aprobó' }));
    await userEvent.click(screen.getByRole('button', { name: 'No autoriza, aprueba igual' }));
    await waitFor(() => expect(calls.find((c) => c.path.includes('/consent'))?.body).toEqual({ acepta: false }));
    await waitFor(() => expect(calls.some((c) => c.path.includes('/quote/response'))).toBe(true));
  });

  it('si falla guardar la autorización, no se aprueba y se dice por qué', async () => {
    const calls = mockApi({
      'GET /me': owner, 'GET /orders/{id}/quote/client-view': vista({}, { estado: 'pendiente' }),
      'POST /customers/{id}/consent': { status: 403, body: { detail: 'Tu rol no puede registrar autorizaciones.' } },
      'POST /orders/{id}/quote/response': cotizacionOK,
    });
    pantalla();
    await userEvent.click(await screen.findByRole('button', { name: 'El cliente aprobó' }));
    await userEvent.click(screen.getByRole('button', { name: 'El cliente autoriza y aprueba' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Tu rol no puede registrar autorizaciones.');
    expect(calls.some((c) => c.path.includes('/quote/response'))).toBe(false);
  });

  it('si la autorización se guardó y falla la aprobación, reintentar no repite la autorización', async () => {
    let intentos = 0;
    const calls = mockApi({
      'GET /me': owner, 'GET /orders/{id}/quote/client-view': vista({}, { estado: 'pendiente' }), 'POST /customers/{id}/consent': { id: 'c1' },
      'POST /orders/{id}/quote/response': () => (++intentos === 1 ? { status: 500, body: { detail: 'Intenta de nuevo.' } } : cotizacionOK),
    });
    pantalla();
    await userEvent.click(await screen.findByRole('button', { name: 'El cliente aprobó' }));
    await userEvent.click(screen.getByRole('button', { name: 'El cliente autoriza y aprueba' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Intenta de nuevo.');
    await userEvent.click(await screen.findByRole('button', { name: 'El cliente aprobó' }));
    await waitFor(() => expect(intentos).toBe(2));
    expect(calls.filter((c) => c.path.includes('/consent'))).toHaveLength(1);
  });

  it('"No aprobó" lleva un motivo opcional', async () => {
    const calls = mockApi({ 'GET /me': owner, 'GET /orders/{id}/quote/client-view': vista(), 'POST /orders/{id}/quote/response': cotizacionOK });
    pantalla();
    await userEvent.click(await screen.findByRole('button', { name: 'No aprobó' }));
    await userEvent.type(screen.getByLabelText('Motivo (opcional)'), 'Está muy cara');
    await userEvent.click(screen.getByRole('button', { name: 'Confirmar: no aprobó' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ decision: 'rechazar', canal: 'mostrador', motivo: 'Está muy cara' }));
  });

  it('sin motivo, no manda el campo', async () => {
    const calls = mockApi({ 'GET /me': owner, 'GET /orders/{id}/quote/client-view': vista(), 'POST /orders/{id}/quote/response': cotizacionOK });
    pantalla();
    await userEvent.click(await screen.findByRole('button', { name: 'No aprobó' }));
    await userEvent.click(screen.getByRole('button', { name: 'Confirmar: no aprobó' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ decision: 'rechazar', canal: 'mostrador' }));
  });

  it('un borrador, una aprobada o una vencida no ofrecen aprobar', async () => {
    for (const [cot, texto] of [
      [{ estado: 'borrador' }, /Borrador: todavía no se ha enviado/],
      [{ estado: 'aprobada' }, /Aprobada\. El trabajo puede empezar/],
      [{ estado: 'enviada', vencida: true }, /Esta cotización venció/],
    ] as const) {
      mockApi({ 'GET /me': owner, 'GET /orders/{id}/quote/client-view': vista(cot) });
      const { unmount } = pantalla();
      expect(await screen.findByText(texto)).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'El cliente aprobó' })).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Compartir PDF' })).toBeInTheDocument();
      unmount();
    }
  });

  it('quien no puede registrar respuestas solo ve la cotización y el PDF', async () => {
    mockApi({ 'GET /me': { ...owner, permisos: ['order:read'] }, 'GET /orders/{id}/quote/client-view': vista() });
    pantalla();
    await screen.findByText('Frenos completos');
    expect(screen.queryByRole('button', { name: 'El cliente aprobó' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Compartir PDF' })).toBeInTheDocument();
  });
});

describe('Compartir PDF (V-10)', () => {
  const pdf = () =>
    new Response('%PDF-1.4 prueba', { status: 200, headers: { 'Content-Type': 'application/pdf', 'Content-Disposition': 'attachment; filename="cotizacion-OS-0044.pdf"' } });

  function mockPdf() {
    mockApi({ 'GET /me': owner, 'GET /orders/{id}/quote/client-view': vista() });
    const base = globalThis.fetch;
    vi.stubGlobal('fetch', async (req: Request) => (new URL(req.url).pathname.endsWith('/quote.pdf') ? pdf() : base(req)));
  }

  it('con soporte, abre el menú de compartir del sistema con el archivo PDF', async () => {
    mockPdf();
    const share = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal('navigator', { ...navigator, canShare: () => true, share });
    pantalla();
    await userEvent.click(await screen.findByRole('button', { name: 'Compartir PDF' }));
    await waitFor(() => expect(share).toHaveBeenCalledTimes(1));
    const arg = share.mock.calls[0]?.[0] as { files: File[]; title: string };
    expect(arg.files[0]?.name).toBe('cotizacion-OS-0044.pdf');
    expect(arg.files[0]?.type).toBe('application/pdf');
    expect(arg.title).toBe('Cotización OS-0044');
  });

  it('sin soporte, descarga el archivo y lo dice', async () => {
    mockPdf();
    vi.stubGlobal('navigator', { ...navigator, canShare: undefined, share: undefined });
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined);
    URL.createObjectURL = () => 'blob:prueba';
    URL.revokeObjectURL = () => undefined;
    pantalla();
    await userEvent.click(await screen.findByRole('button', { name: 'Compartir PDF' }));
    expect(await screen.findByText('Se descargó el PDF.')).toBeInTheDocument();
    expect(click).toHaveBeenCalled();
    click.mockRestore();
  });

  it('cerrar el menú de compartir sin elegir no es un error ni descarga', async () => {
    mockPdf();
    const share = vi.fn().mockRejectedValue(new DOMException('cancelado', 'AbortError'));
    vi.stubGlobal('navigator', { ...navigator, canShare: () => true, share });
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined);
    pantalla();
    await userEvent.click(await screen.findByRole('button', { name: 'Compartir PDF' }));
    await waitFor(() => expect(share).toHaveBeenCalled());
    expect(click).not.toHaveBeenCalled();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    click.mockRestore();
  });

  it('si el PDF falla, lo dice sin romper la pantalla', async () => {
    mockApi({
      'GET /me': owner, 'GET /orders/{id}/quote/client-view': vista(),
      'GET /orders/{id}/quote.pdf': { status: 404, body: { detail: 'No encontramos la cotización de esa orden.' } },
    });
    pantalla();
    await userEvent.click(await screen.findByRole('button', { name: 'Compartir PDF' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('No encontramos la cotización de esa orden.');
  });
});

describe('Volver con el dedo sostenido (V-10)', () => {
  it('un toque corto solo avisa cómo se sale; mantenerlo presionado vuelve a la orden', async () => {
    mockApi({ 'GET /me': owner, 'GET /orders/{id}/quote/client-view': vista() });
    pantalla();
    const volver = await screen.findByRole('button', { name: 'Volver' });
    vi.useFakeTimers({ shouldAdvanceTime: true });

    fireEvent.pointerDown(volver);
    act(() => void vi.advanceTimersByTime(200));
    fireEvent.pointerUp(volver);
    expect(screen.getByText('Mantén presionado «Volver» para salir.')).toBeInTheDocument();
    expect(screen.queryByText('Detalle de la orden')).not.toBeInTheDocument();

    fireEvent.pointerDown(volver);
    act(() => void vi.advanceTimersByTime(800));
    expect(await screen.findByText('Detalle de la orden')).toBeInTheDocument();
  });

  it('soltar o salir del botón antes de tiempo no vuelve', async () => {
    mockApi({ 'GET /me': owner, 'GET /orders/{id}/quote/client-view': vista() });
    pantalla();
    const volver = await screen.findByRole('button', { name: 'Volver' });
    vi.useFakeTimers({ shouldAdvanceTime: true });
    fireEvent.pointerDown(volver);
    act(() => void vi.advanceTimersByTime(300));
    fireEvent.pointerLeave(volver);
    act(() => void vi.advanceTimersByTime(1000));
    expect(screen.queryByText('Detalle de la orden')).not.toBeInTheDocument();
  });
});

describe('Entrada desde la orden (V-10)', () => {
  const o = orden() as unknown as Orden;
  const base = { 'GET /me': owner, 'GET /services': { servicios: [] }, 'GET /parts': { repuestos: [] } };

  it('"Mostrar al cliente" lleva a la pantalla completa cuando la cotización ya tiene ítems', async () => {
    mockApi({ ...base, 'GET /orders/{id}/quote': { cotizacion: { id: 'q1', numero: 1, estado: 'enviada', total: 60000, lineas: [{ id: 'l1', tipo: 'servicio', descripcion: 'Alineación', cantidad: 1, precioUnitario: 60000, subtotal: 60000, sinStock: false, precioLibre: false, descuentoPct: 0 }], version: 2 } } });
    renderPage(<QuotePanel orden={o} />);
    expect(await screen.findByRole('link', { name: 'Mostrar al cliente' })).toHaveAttribute('href', '/panel/ordenes/o1/cliente');
  });

  it('sin ítems todavía no hay nada que mostrar', async () => {
    mockApi({ ...base, 'GET /orders/{id}/quote': { cotizacion: null } });
    renderPage(<QuotePanel orden={o} />);
    await screen.findByText('Aún no hay ítems.');
    expect(screen.queryByRole('link', { name: 'Mostrar al cliente' })).not.toBeInTheDocument();
  });
});
