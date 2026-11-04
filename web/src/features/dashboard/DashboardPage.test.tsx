import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { mockApi, owner, renderPage } from '../../test/api';
import { AutomationsPage } from '../automations/AutomationsPage';
import { DashboardPage } from './DashboardPage';

afterEach(() => vi.unstubAllGlobals());

const kpi = (codigo: string, texto: string, ejemplo = false) => ({
  codigo, titulo: codigo, valor: 1, texto, detalle: '', queSignifica: `Qué significa ${codigo}`, ejemplo,
});
const tablero = (tipo: string, ejemplo: boolean) => ({
  kpis: [kpi('abiertas', tipo === 'moto' ? '1' : '3', ejemplo), kpi('aprobaciones', '1 · 26 h'), kpi('tiempo', '30 h · 1.3 días hábiles', ejemplo)],
  porEstado: { recibido: 1 }, ejemplo, tipo,
});

describe('DashboardPage (HU-68 a HU-71)', () => {
  it('marca los números de ejemplo y filtra por carros y motos en un taller de ambos', async () => {
    const calls = mockApi({
      'GET /me': { ...owner, permisos: [...owner.permisos, 'kpi:read'] },
      'GET /dashboard': (req: Request) => {
        const tipo = new URL(req.url).searchParams.get('tipo') ?? '';
        return { status: 200, body: tablero(tipo, tipo === '') };
      },
      'GET /today': { pendientes: [] },
      'GET /workshop': { atiende: 'ambos', nombre: 'Servitaller El Pistón' },
      'GET /workshop/settings': {},
    });
    renderPage(<DashboardPage />);
    expect(await screen.findByText('1 · 26 h')).toBeInTheDocument();
    expect(screen.getByRole('note')).toHaveTextContent(/datos|ejemplo/i);
    expect(screen.getAllByText('ejemplo', { selector: 'span' })).toHaveLength(2);

    await userEvent.click(await screen.findByRole('radio', { name: 'Motos' }));
    await waitFor(() => expect(calls.some((c) => c.path.includes('tipo=moto'))).toBe(true));
    await waitFor(() => expect(screen.queryAllByText('ejemplo', { selector: 'span' })).toHaveLength(0));
  });

  it('un taller de solo carros no muestra el filtro', async () => {
    mockApi({
      'GET /me': { ...owner, permisos: [...owner.permisos, 'kpi:read'] },
      'GET /dashboard': tablero('', false),
      'GET /today': { pendientes: [] },
      'GET /workshop': { atiende: 'carro', nombre: 'Taller' },
      'GET /workshop/settings': {},
    });
    renderPage(<DashboardPage />);
    expect(await screen.findByText('1 · 26 h')).toBeInTheDocument();
    expect(screen.queryByRole('radio', { name: 'Motos' })).not.toBeInTheDocument();
  });
});

describe('AutomationsPage (HU-42)', () => {
  it('avisa que en producción el texto de un aviso fuera de la ventana necesita aprobación de Meta', async () => {
    const tarjeta = (codigo: string, fueraDeVentana: boolean) => ({
      codigo, nombre: codigo === 'AU-06' ? 'Recordatorio de la cita' : 'Vehículo recibido', disparador: 'cuando pasa algo', activa: true,
      proximamente: false, enviaMensaje: true, editable: true, texto: 'Hola {cliente}', datos: ['cliente', 'vehiculo'], ejecucionesSemana: 0,
      costo: '1 mensaje por envío', fueraDeVentana,
    });
    mockApi({
      'GET /me': { ...owner, permisos: [...owner.permisos, 'automation:manage'] },
      'GET /automations': { automatizaciones: [tarjeta('AU-01', false), tarjeta('AU-06', true)] },
      'GET /automations/runs': { ejecuciones: [] },
    });
    renderPage(<AutomationsPage />);
    expect(await screen.findByText('Recordatorio de la cita')).toBeInTheDocument();
    expect(screen.getAllByRole('note')).toHaveLength(1);
    expect(screen.getByRole('note')).toHaveTextContent(/aprobada por Meta/);
  });
});
