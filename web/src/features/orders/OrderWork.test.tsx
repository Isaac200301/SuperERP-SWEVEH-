import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Orden } from '../../shared/api/client';
import { mechanic, mockApi, orden, owner, renderPage } from '../../test/api';
import { FloorPage } from './FloorPage';
import { OrderActions, QuotePanel, ReceptionPanel } from './OrderWork';

afterEach(() => vi.unstubAllGlobals());
const o = (over: Record<string, unknown> = {}) => orden(over) as unknown as Orden;

const checklist = {
  tipo: 'carro', kilometraje: null, combustible: null, accesorios: [], salvedades: [],
  ladosConFoto: ['frente'], ladosPedidos: ['frente', 'trasera', 'izquierdo', 'derecho'], falta: ['fotos: trasera', 'kilometraje'], completadoEn: null,
  items: [{ clave: 'luces', nombre: 'Luces', estado: '' }, { clave: 'gato', nombre: 'Gato y herramienta', estado: '' }], fotosMinimas: 4, kmObligatorio: true,
};
const cotizacion = (over: Record<string, unknown> = {}) => ({
  cotizacion: {
    id: 'q1', numero: 1, estado: 'borrador', total: 145000, enviadaEn: null, respondidaEn: null, canalRespuesta: null,
    lineas: [{ id: 'l1', tipo: 'repuesto', refId: 'p1', descripcion: 'Pastillas (PF-20)', cantidad: 1, precioUnitario: 65000, subtotal: 65000, sinStock: true, aprobada: null }],
    ...over,
  },
});

describe('OrderActions (HU-20, HU-25)', () => {
  it('avanza con la versión y se devuelve un paso', async () => {
    const calls = mockApi({ 'GET /me': owner, 'POST /orders/{id}/transition': orden() });
    renderPage(<OrderActions orden={o({ estado: 'recibido' })} />);
    await userEvent.click(screen.getByRole('button', { name: 'Pasar a En diagnóstico' }));
    await userEvent.click(screen.getByRole('button', { name: 'Devolver a Por recibir' }));
    await waitFor(() => expect(calls.filter((c) => c.method === 'POST')).toHaveLength(2));
    expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ estado: 'en_diagnostico', version: 3 });
  });

  it('el dueño escribe el motivo para seguir sin aprobación; el mecánico no ve ese campo', async () => {
    const calls = mockApi({ 'GET /me': owner, 'POST /orders/{id}/transition': { status: 409, body: { detail: 'x', code: 'version_conflicto' } } });
    renderPage(<OrderActions orden={o({ estado: 'esperando_aprobacion' })} />);
    await userEvent.type(await screen.findByLabelText(/Motivo si el cliente aprobó/), 'aprobó por teléfono');
    await userEvent.click(screen.getByRole('button', { name: 'Pasar a En reparación' }));
    expect(await screen.findByText(/Alguien cambió esta orden/)).toBeInTheDocument();
    expect(calls.at(-1)?.body).toMatchObject({ motivo: 'aprobó por teléfono' });

    vi.unstubAllGlobals();
    mockApi({ 'GET /me': mechanic });
    renderPage(<OrderActions orden={o({ estado: 'esperando_aprobacion' })} />);
    await waitFor(() => expect(screen.getAllByRole('button', { name: 'Pasar a En reparación' })).toHaveLength(2));
    expect(screen.getAllByLabelText(/Motivo si el cliente aprobó/)).toHaveLength(1);
  });

  it('para entregar pide quién recoge y marcar cada accesorio que dejó; entregada no muestra acciones', async () => {
    const calls = mockApi({
      'GET /me': owner,
      'GET /orders/{id}/checklist': { ...checklist, tipo: 'moto', accesorios: ['casco', 'documentos'] },
      'POST /orders/{id}/transition': orden(),
    });
    const { unmount } = renderPage(<OrderActions orden={o({ estado: 'listo' })} />);
    const entregar = () => screen.getByRole('button', { name: /Pasar a Entregado/ });
    expect(await screen.findByLabelText('Casco')).toBeInTheDocument();
    expect(screen.getByText('Escribe quién recoge.')).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText('¿Quién recoge?'), ' Laura ');
    await userEvent.click(screen.getByLabelText('Casco'));
    expect(screen.getByText('Marca cada accesorio que devolviste.')).toBeInTheDocument();
    expect(entregar()).toBeDisabled();
    await userEvent.click(screen.getByLabelText('Documentos'));
    await userEvent.click(entregar());
    await waitFor(() => expect(calls.find((c) => c.method === 'POST')?.body).toMatchObject({ recogidoPor: 'Laura', accesoriosConfirmados: true }));
    unmount();
    renderPage(<OrderActions orden={o({ estado: 'entregado' })} />);
    expect(screen.queryByText('Mover la orden')).not.toBeInTheDocument();
  });

  it('si no dejó accesorios pide confirmarlo', async () => {
    const calls = mockApi({ 'GET /me': owner, 'GET /orders/{id}/checklist': checklist, 'POST /orders/{id}/transition': orden() });
    renderPage(<OrderActions orden={o({ estado: 'listo' })} />);
    await userEvent.type(screen.getByLabelText('¿Quién recoge?'), 'Laura');
    expect(await screen.findByText('Confirma que no dejó accesorios.')).toBeInTheDocument();
    await userEvent.click(screen.getByLabelText('No dejó accesorios'));
    await userEvent.click(screen.getByRole('button', { name: /Pasar a Entregado/ }));
    await waitFor(() => expect(calls.find((c) => c.method === 'POST')?.body).toMatchObject({ recogidoPor: 'Laura', accesoriosConfirmados: true }));
  });
});

describe('OrderActions con estados opcionales del taller', () => {
  const params = { usarEsperandoRepuesto: true, usarControlCalidad: true };
  it('en reparación permite Esperando repuesto con motivo y avanza a Control de calidad, no a Listo', async () => {
    const calls = mockApi({ 'GET /me': owner, 'GET /workshop/settings': params, 'POST /orders/{id}/transition': orden() });
    renderPage(<OrderActions orden={o({ estado: 'en_reparacion' })} />);
    expect(await screen.findByRole('button', { name: 'Pasar a Control de calidad' })).toBeInTheDocument();
    expect(screen.getByText('Escribe qué repuesto falta.')).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText('¿Qué repuesto falta?'), 'Kit de embrague');
    await userEvent.click(screen.getByRole('button', { name: 'Poner en Esperando repuesto' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ estado: 'esperando_repuesto', version: 3, motivo: 'Kit de embrague' }));
  });

  it('esperando repuesto solo vuelve a En reparación', async () => {
    mockApi({ 'GET /me': owner, 'GET /workshop/settings': params });
    renderPage(<OrderActions orden={o({ estado: 'esperando_repuesto' })} />);
    expect(await screen.findByRole('button', { name: 'Pasar a En reparación' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Devolver a/ })).not.toBeInTheDocument();
  });
});

describe('ReceptionPanel (HU-17, HU-21)', () => {
  it('dice qué falta, guarda el checklist, sube fotos y notas', async () => {
    const calls = mockApi({
      'GET /orders/{id}/checklist': checklist,
      'PUT /orders/{id}/checklist': checklist,
      'GET /orders/{id}/evidence': {
        evidencias: [
          { id: 'e1', lado: 'frente', nota: null, tieneFoto: true, mostrarCliente: true, etapa: 'recibido', autorId: null, creadoEn: '2026-11-02T15:00:00Z' },
          { id: 'e2', lado: null, nota: 'Buje gastado', tieneFoto: false, mostrarCliente: false, etapa: 'en_diagnostico', autorId: null, creadoEn: '2026-11-02T16:00:00Z' },
        ],
      },
      'POST /orders/{id}/evidence': { id: 'e3' },
    });
    renderPage(<ReceptionPanel orden={o({ estado: 'por_recibir' })} />);
    expect(await screen.findByText('fotos: trasera; kilometraje')).toBeInTheDocument();
    expect(await screen.findByText('Buje gastado')).toBeInTheDocument();
    expect(screen.getByAltText('Foto frente')).toHaveAttribute('src', '/api/v1/orders/o1/evidence/e1/file');

    await userEvent.type(screen.getByLabelText('Kilometraje'), '45a000');
    await userEvent.selectOptions(screen.getByLabelText('Combustible'), '1/2');
    await userEvent.type(screen.getByLabelText(/Accesorios/), 'gato, llanta');
    // Ítems del taller: luces mal con nota, gato bien.
    const luces = screen.getByRole('radiogroup', { name: 'Luces' });
    await userEvent.click(within(luces).getByRole('radio', { name: 'Mal' }));
    await userEvent.type(screen.getByLabelText('Qué tiene Luces'), 'Stop derecho fundido');
    await userEvent.click(within(screen.getByRole('radiogroup', { name: 'Gato y herramienta' })).getByRole('radio', { name: 'Bien' }));
    await userEvent.click(screen.getByRole('button', { name: 'Guardar checklist' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'PUT')?.body).toEqual({
      kilometraje: 45000, combustible: '1/2', accesorios: ['gato', 'llanta'],
      items: [{ clave: 'luces', nombre: 'Luces', estado: 'mal', nota: 'Stop derecho fundido' }, { clave: 'gato', nombre: 'Gato y herramienta', estado: 'bien' }],
    }));

    const foto = new File([new Uint8Array([0xff, 0xd8, 0xff])], 'f.jpg', { type: 'image/jpeg' });
    const inputs = document.querySelectorAll<HTMLInputElement>('input[type=file]');
    await userEvent.upload(inputs[1] as HTMLInputElement, foto);
    await userEvent.type(screen.getByLabelText(/Nota/), 'Rayón en la puerta');
    // Una nota con varias fotos va en un solo envío con todas las fotos (rayones, piezas encontradas).
    const tres = [1, 2, 3].map((i) => new File([new Uint8Array([0xff, 0xd8, 0xff, i])], `r${i}.jpg`, { type: 'image/jpeg' }));
    await userEvent.upload(screen.getByLabelText('Fotos de la nota'), tres);
    expect(screen.getByText('3 fotos')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Guardar nota' }));
    await waitFor(() => expect(calls.filter((c) => c.method === 'POST' && c.path.endsWith('/evidence'))).toHaveLength(2));
    const nota = String(calls.filter((c) => c.method === 'POST')[1]?.body);
    expect(nota.match(/name="foto"/g)).toHaveLength(3);
    expect(nota).toContain('Rayón en la puerta');
  });

  it('en moto marca casco y demás con un toque, deja escribir otros y ofrece "no aplica" (HU-18)', async () => {
    const moto = { ...checklist, tipo: 'moto', accesorios: ['documentos', 'impermeable'], items: [] };
    const calls = mockApi({ 'GET /orders/{id}/checklist': moto, 'PUT /orders/{id}/checklist': moto, 'GET /orders/{id}/evidence': { evidencias: [] } });
    renderPage(<ReceptionPanel orden={o({ estado: 'por_recibir' })} />);
    const casco = await screen.findByRole('button', { name: 'Casco' });
    expect(screen.getByRole('button', { name: 'Documentos' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('option', { name: 'No aplica (moto)' })).toBeInTheDocument();
    await userEvent.click(casco);
    expect(casco).toHaveAttribute('aria-pressed', 'true');
    await userEvent.click(screen.getByRole('button', { name: 'Documentos' }));
    await userEvent.selectOptions(screen.getByLabelText('Combustible'), 'no_aplica');
    await userEvent.click(screen.getByRole('button', { name: 'Guardar checklist' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'PUT')?.body).toMatchObject({ combustible: 'no_aplica', accesorios: ['casco', 'impermeable'] }));
  });

  it('en carro no ofrece "no aplica" ni los botones de moto', async () => {
    mockApi({ 'GET /orders/{id}/checklist': checklist, 'GET /orders/{id}/evidence': { evidencias: [] } });
    renderPage(<ReceptionPanel orden={o({ estado: 'por_recibir' })} />);
    expect(await screen.findByLabelText('Combustible')).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: 'No aplica (moto)' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Casco' })).not.toBeInTheDocument();
  });

  it('una orden cerrada no se edita', async () => {
    mockApi({ 'GET /orders/{id}/checklist': { ...checklist, falta: [], completadoEn: '2026-11-02T15:00:00Z' }, 'GET /orders/{id}/evidence': { evidencias: [] } });
    renderPage(<ReceptionPanel orden={o({ estado: 'entregado' })} />);
    expect(await screen.findByText('Recepción · completa')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Guardar checklist' })).not.toBeInTheDocument();
  });
});

describe('QuotePanel (HU-27, HU-29)', () => {
  it('agrega servicios con y sin precio, repuestos con cantidad, mano de obra libre, cambia cantidades y envía', async () => {
    const calls = mockApi({
      'GET /me': owner,
      'GET /orders/{id}/quote': cotizacion(),
      'GET /services': { servicios: [{ id: 's1', nombre: 'Alineación', tipoVehiculo: 'carro', precio: 60000, duracionMin: 30, activo: true, version: 1 }, { id: 's2', nombre: 'Revisión', tipoVehiculo: 'carro', precio: null, duracionMin: null, activo: true, version: 1 }] },
      'GET /parts': { repuestos: [{ id: 'p1', referencia: 'PF-20', nombre: 'Pastillas', tipoVehiculo: 'carro', precioVenta: 65000, existencia: 0, minimo: 1, bajoMinimo: true, activo: true, version: 1 }] },
      'POST /orders/{id}/quote/lines': cotizacion(),
      'PATCH /orders/{id}/quote/lines/{lineId}': cotizacion(),
      'DELETE /orders/{id}/quote/lines/{lineId}': cotizacion(),
      'POST /orders/{id}/quote/send': cotizacion({ estado: 'enviada' }),
    });
    renderPage(<QuotePanel orden={o()} />);
    expect(await screen.findByText('sin existencia')).toBeInTheDocument();
    expect(await screen.findByText('sin precio de lista')).toBeInTheDocument();
    const agregar = () => screen.getAllByRole('button', { name: 'Agregar' });
    await userEvent.click(agregar()[0] as HTMLElement); // Alineación: precio de lista
    // Revisión sin precio: no deja agregar hasta escribir el valor.
    expect(screen.getByText('Escribe el precio.')).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText('Precio de Servicio · Revisión'), '45.000');
    await userEvent.click(agregar()[1] as HTMLElement);
    await userEvent.clear(screen.getByLabelText('Cantidad de Repuesto · Pastillas'));
    await userEvent.type(screen.getByLabelText('Cantidad de Repuesto · Pastillas'), '2');
    await userEvent.click(agregar()[2] as HTMLElement);
    await userEvent.type(screen.getByLabelText('Mano de obra u otro ítem'), 'Mano de obra frenos');
    await userEvent.type(screen.getByLabelText('Valor (COP)'), '70000');
    await userEvent.click(screen.getByRole('button', { name: 'Agregar ítem' }));
    await userEvent.click(screen.getByRole('button', { name: 'Más Pastillas (PF-20)' }));
    await userEvent.click(screen.getByRole('button', { name: 'Quitar' }));
    await userEvent.click(screen.getByRole('button', { name: 'Enviar cotización al cliente' }));
    await waitFor(() => expect(calls.some((c) => c.path.endsWith('/quote/send'))).toBe(true));
    expect(calls.filter((c) => c.method === 'POST' && c.path.endsWith('/quote/lines')).map((c) => c.body)).toEqual([
      { tipo: 'servicio', refId: 's1', cantidad: 1 },
      { tipo: 'servicio', refId: 's2', cantidad: 1, precioUnitario: 45000 },
      { tipo: 'repuesto', refId: 'p1', cantidad: 2 },
      { tipo: 'servicio', descripcion: 'Mano de obra frenos', precioUnitario: 70000, cantidad: 1 },
    ]);
    expect(calls.find((c) => c.method === 'PATCH')?.body).toEqual({ cantidad: 2 });
  });

  it('sin cotización muestra una vacía y no deja enviar', async () => {
    mockApi({ 'GET /me': owner, 'GET /orders/{id}/quote': { cotizacion: null }, 'GET /services': { servicios: [] }, 'GET /parts': { repuestos: [] } });
    renderPage(<QuotePanel orden={o()} />);
    expect(await screen.findByText('Aún no hay ítems.')).toBeInTheDocument();
    expect(screen.getByText('Agrega al menos un ítem para enviarla.')).toBeInTheDocument();
  });

  it('registra la respuesta solo si el rol puede', async () => {
    const calls = mockApi({ 'GET /me': owner, 'GET /orders/{id}/quote': cotizacion({ estado: 'enviada' }), 'POST /orders/{id}/quote/response': { status: 422, body: { detail: 'No se pudo.' } } });
    renderPage(<QuotePanel orden={o()} />);
    await userEvent.click(await screen.findByRole('button', { name: 'Aprobó por teléfono' }));
    await userEvent.click(screen.getByRole('button', { name: 'Aprobó en el mostrador' }));
    await userEvent.click(screen.getByRole('button', { name: 'No aprobó' }));
    expect(await screen.findByText('No se pudo.')).toBeInTheDocument();
    expect(calls.filter((c) => c.path.endsWith('/response')).map((c) => c.body)).toEqual([
      { decision: 'aprobar', canal: 'telefono' },
      { decision: 'aprobar', canal: 'mostrador' },
      { decision: 'rechazar', canal: 'telefono' },
    ]);
  });

  it('con la cotización aprobada devuelve al inventario un repuesto que no se usó (HU-38)', async () => {
    const calls = mockApi({
      'GET /me': owner,
      'GET /orders/{id}/quote': cotizacion({ estado: 'aprobada', lineas: [{ ...cotizacion().cotizacion.lineas[0], aprobada: true, sinStock: false }] }),
      'GET /orders/{id}/part-movements': { repuestos: [{ repuestoId: 'p1', referencia: 'PF-20', nombre: 'Pastillas', descontado: 2, devuelto: 0, pendiente: 0, porDevolver: 2 }] },
      'POST /parts/{id}/returns': { devueltas: 1, quedanPorDevolver: 1, existencia: 4 },
    });
    renderPage(<QuotePanel orden={o({ estado: 'en_reparacion' })} />);
    expect(await screen.findByText(/Descontado 2 · Devuelto 0/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Devolver 1 al inventario' }));
    expect(await screen.findByText(/Se devolvió 1 de Pastillas \(PF-20\)\. Quedan 1 por devolver · 4 en bodega/)).toBeInTheDocument();
    expect(calls.find((c) => c.path === '/parts/p1/returns')?.body).toEqual({ cantidad: 1, ordenId: 'o1' });
  });

  it('un repuesto que quedó pendiente (sin existencia) no muestra el botón de devolver', async () => {
    mockApi({
      'GET /me': owner,
      'GET /orders/{id}/quote': cotizacion({ estado: 'aprobada', lineas: [{ ...cotizacion().cotizacion.lineas[0], aprobada: true }] }),
      'GET /orders/{id}/part-movements': { repuestos: [{ repuestoId: 'p1', referencia: 'PF-20', nombre: 'Pastillas', descontado: 0, devuelto: 0, pendiente: 1, porDevolver: 0 }] },
    });
    renderPage(<QuotePanel orden={o({ estado: 'en_reparacion' })} />);
    expect(await screen.findByText(/Pendiente 1 \(no había existencia\)/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Devolver 1 al inventario' })).not.toBeInTheDocument();
  });
});

describe('QuotePanel con parámetros del taller (IVA, horas, descuento, vigencia)', () => {
  const dueno = { ...owner, permisos: [...owner.permisos, 'quote:discount', 'quote:send', 'quote:edit'] };
  it('muestra el desglose, descuenta por línea y cotiza mano de obra por horas', async () => {
    const calls = mockApi({
      'GET /me': dueno,
      'GET /workshop/settings': { tarifaHora: 80000, usarEsperandoRepuesto: false, usarControlCalidad: false },
      'GET /orders/{id}/quote': cotizacion({ subtotal: 130000, descuento: 13000, ivaPct: 19, iva: 22230, preciosConIva: false, total: 139230 }),
      'GET /services': { servicios: [] },
      'GET /parts': { repuestos: [] },
      'PATCH /orders/{id}/quote/lines/{lineId}': cotizacion(),
      'POST /orders/{id}/quote/lines': cotizacion(),
    });
    renderPage(<QuotePanel orden={o()} />);
    expect(await screen.findByText('IVA 19 %')).toBeInTheDocument();
    expect(screen.getByText(/−\$\s?13\.000/)).toBeInTheDocument();
    const desc = screen.getByLabelText('Descuento de Pastillas (PF-20)');
    await userEvent.type(desc, '10');
    await userEvent.tab();
    await userEvent.type(await screen.findByLabelText('Mano de obra u otro ítem'), 'Mano de obra frenos');
    await userEvent.type(screen.getByLabelText('Horas'), '1,5');
    await userEvent.click(screen.getByRole('button', { name: 'Agregar ítem' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ tipo: 'servicio', descripcion: 'Mano de obra frenos', horas: 1.5, cantidad: 1 }));
    expect(calls.find((c) => c.method === 'PATCH')?.body).toEqual({ descuentoPct: 10 });
  });

  it('una cotización vencida pide renovar la vigencia', async () => {
    const calls = mockApi({
      'GET /me': dueno,
      'GET /workshop/settings': { tarifaHora: null },
      'GET /orders/{id}/quote': cotizacion({ estado: 'enviada', venceEn: '2026-10-01T15:00:00Z', vencida: true }),
      'POST /orders/{id}/quote/renew': cotizacion({ estado: 'enviada', venceEn: '2026-11-18T15:00:00Z', vencida: false }),
    });
    renderPage(<QuotePanel orden={o({ estado: 'esperando_aprobacion' })} />);
    expect(await screen.findByText(/Venció el/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Renovar vigencia' }));
    await waitFor(() => expect(calls.some((c) => c.path.endsWith('/quote/renew'))).toBe(true));
  });
});

describe('FloorPage (HU-23)', () => {
  it('muestra las órdenes del mecánico y el estado vacío', async () => {
    mockApi({ 'GET /orders/mine': { ordenes: [orden({ fechaPrometida: '2026-11-03T17:00:00Z' }), orden({ id: 'o2', vehiculo: { ...orden().vehiculo, marca: null, linea: null, tipo: 'moto' } })] } });
    const { unmount } = renderPage(<FloorPage />);
    expect(await screen.findByText(/Prometida:/)).toBeInTheDocument();
    expect(screen.getByText('Moto')).toBeInTheDocument();
    unmount();
    mockApi({ 'GET /orders/mine': { ordenes: [] } });
    renderPage(<FloorPage />);
    expect(await screen.findByText(/No hay órdenes abiertas/)).toBeInTheDocument();
  });
});
