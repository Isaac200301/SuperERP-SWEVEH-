import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Route, Routes } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { RequireFeature } from '../app/RequireFeature';
import type { Sesion } from '../shared/api/client';
import { mockApi, orden, owner, renderPage } from '../test/api';
import { PartsPage } from './inventory/PartsPage';
import { OrderDetailPage } from './orders/OrderDetailPage';
import { sectionsFor } from './panel/sections';

afterEach(() => vi.unstubAllGlobals());

// Entrega del 27-nov (R1, V-01, V-02): con WhatsApp y Plataforma apagados en el servidor, el panel no los muestra.
const apagado = {
  ...owner,
  adminPlataforma: true,
  funciones: { whatsapp: false, plataforma: false },
  permisos: [...owner.permisos, 'inbox:manage', 'conversation:read', 'automation:manage', 'kpi:read'],
} as Sesion;

describe('Funciones apagadas (V-01, V-02)', () => {
  it('el menú no tiene Por revisar, Conversaciones, Automatizaciones ni Plataforma', () => {
    const rutas = sectionsFor(apagado, false).map((s) => s.to);
    expect(rutas).not.toContain('/panel/por-revisar');
    expect(rutas).not.toContain('/panel/conversaciones');
    expect(rutas).not.toContain('/panel/automatizaciones');
    expect(rutas).not.toContain('/panel/plataforma');
    expect(rutas).toContain('/panel/ordenes');
  });

  it('con las funciones encendidas vuelven', () => {
    const rutas = sectionsFor({ ...apagado, funciones: { whatsapp: true, plataforma: true } }, false).map((s) => s.to);
    expect(rutas).toEqual(expect.arrayContaining(['/panel/por-revisar', '/panel/conversaciones', '/panel/automatizaciones', '/panel/plataforma']));
  });

  it('sin el dato funciones (servidor viejo), todo queda apagado', () => {
    const viejo: Partial<Sesion> = { ...apagado };
    delete viejo.funciones;
    const rutas = sectionsFor(viejo as Sesion, false).map((s) => s.to);
    expect(rutas).not.toContain('/panel/por-revisar');
  });

  it('una URL vieja de WhatsApp vuelve al inicio del panel', async () => {
    mockApi({ 'GET /me': apagado });
    renderPage(
      <Routes>
        <Route path="/panel" element={<p>Inicio del panel</p>} />
        <Route
          path="/panel/por-revisar"
          element={
            <RequireFeature funcion="whatsapp">
              <p>Por revisar</p>
            </RequireFeature>
          }
        />
      </Routes>,
      '/panel/por-revisar',
    );
    expect(await screen.findByText('Inicio del panel')).toBeInTheDocument();
    expect(screen.queryByText('Por revisar')).not.toBeInTheDocument();
  });

  it('el repuesto no muestra la vitrina de WhatsApp (ADR-0029)', async () => {
    const pastillas = {
      id: 'p1', referencia: 'PF-20', nombre: 'Pastillas', tipoVehiculo: 'carro', costo: 40000, precioVenta: 65000, existencia: 3, minimo: 2,
      bajoMinimo: false, activo: true, version: 1, ofrecerWhatsapp: true,
    };
    mockApi({ 'GET /me': { ...apagado, permisos: [...(apagado.permisos ?? []), 'part:manage', 'part:cost'] }, 'GET /parts': { repuestos: [pastillas] } });
    renderPage(<PartsPage />);
    await userEvent.click(await screen.findByText('Pastillas'));
    expect(await screen.findByRole('button', { name: 'Guardar cambios' })).toBeInTheDocument();
    expect(screen.queryByText(/WhatsApp/)).not.toBeInTheDocument();
    expect(screen.queryByText(/en vitrina/)).not.toBeInTheDocument();
  });

  it('la primera orden no muestra la vista previa del aviso de WhatsApp (HU-05)', async () => {
    mockApi({
      'GET /me': apagado,
      'GET /orders/{id}': orden({ estado: 'por_recibir' }),
      'GET /orders/{id}/checklist': { tipo: 'carro', kilometraje: null, combustible: null, accesorios: [], salvedades: [], ladosConFoto: [], ladosPedidos: [], falta: [], completadoEn: null },
      'GET /orders/{id}/evidence': { evidencias: [] },
    });
    renderPage(
      <Routes>
        <Route path="/panel/ordenes/:id" element={<OrderDetailPage />} />
      </Routes>,
      '/panel/ordenes/o1?primera=1',
    );
    expect(await screen.findByText(orden().codigo as string)).toBeInTheDocument();
    expect(screen.queryByRole('region', { name: 'Vista previa del aviso' })).not.toBeInTheDocument();
    expect(screen.queryByText(/WhatsApp/)).not.toBeInTheDocument();
  });
});
