import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { mockApi, owner, renderPage } from '../../test/api';
import { AutomationsPage } from './AutomationsPage';

afterEach(() => vi.unstubAllGlobals());

const agente = (activa: boolean) => ({
  codigo: 'AU-04', nombre: 'Agente de WhatsApp', disparador: 'Llega un mensaje', activa, proximamente: false, enviaMensaje: true,
  editable: false, texto: '', datos: null, ejecucionesSemana: 0, costo: 'gratis',
});

describe('AutomationsPage (HU-06)', () => {
  it('pide el horario al encender el agente y lo enciende al guardarlo', async () => {
    let tries = 0;
    const calls = mockApi({
      'GET /me': { ...owner, permisos: [...owner.permisos, 'automation:manage'] },
      'GET /automations': { automatizaciones: [agente(false)] },
      'GET /automations/runs': { ejecuciones: [] },
      'GET /workshop': {
        id: 't1', nombre: 'El Pistón', atiende: 'carro', especialidades: [], ciudad: '', direccion: '', tamano: 'solo', horario: {},
        contratoVersion: 'v1', contratoAceptadoEn: '2026-11-01T00:00:00Z', onboardingCompleto: true,
      },
      'PATCH /automations/{code}': () => (++tries === 1
        ? { status: 422, body: { code: 'horario_requerido', detail: 'Antes de encender el agente carga el horario del taller.' } }
        : { automatizaciones: [agente(true)] }),
      'PUT /workshop/schedule': {},
    });
    renderPage(<AutomationsPage />);
    await userEvent.click(await screen.findByRole('switch'));
    expect(await screen.findByRole('alert')).toHaveTextContent('carga el horario');

    await userEvent.click(screen.getByLabelText('Lunes'));
    await userEvent.click(screen.getByRole('button', { name: 'Guardar horario' }));
    expect(await screen.findByText('Encendida')).toBeInTheDocument();
    expect(calls.filter((c) => c.method === 'PATCH').map((c) => c.body)).toEqual([{ activa: true }, { activa: true }]);
    expect(calls.find((c) => c.method === 'PUT')?.body).toMatchObject({ horario: { lun: { abre: '08:00', cierra: '17:00' } } });
  });
});
