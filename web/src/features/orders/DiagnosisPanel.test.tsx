import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Orden } from '../../shared/api/client';
import { mockApi, orden, owner, renderPage } from '../../test/api';
import { DiagnosisPanel } from './DiagnosisPanel';
import { cotizarEvento } from './OrderWork';

afterEach(() => vi.unstubAllGlobals());

const hallazgo = { id: 'h1', sistema: 'frenos', descripcion: 'Pastillas al 10 %', gravedad: 'urgente', codigosFalla: ['C1234'], creadoEn: '2026-11-03T15:00:00Z' };

describe('DiagnosisPanel (HU-103 propuesta)', () => {
  it('registra un hallazgo, lo quita antes de cotizar y manda "Cotizar esto" a la cotización', async () => {
    const calls = mockApi({
      'GET /me': owner,
      'GET /orders/{id}/findings': { hallazgos: [hallazgo] },
      'POST /orders/{id}/findings': { status: 201, body: hallazgo },
      'DELETE /orders/{id}/findings/{fid}': { status: 204 },
    });
    const pedidos: string[] = [];
    window.addEventListener(cotizarEvento, (e) => pedidos.push((e as CustomEvent<string>).detail));
    renderPage(<DiagnosisPanel orden={orden({ estado: 'en_diagnostico' }) as unknown as Orden} />);

    expect(await screen.findByText('Pastillas al 10 %')).toBeInTheDocument();
    expect(screen.getByText('Escoge el sistema.')).toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText('Sistema'), 'motor');
    await userEvent.type(screen.getByLabelText('Qué encontraste'), 'Falla de encendido en el cilindro 1');
    await userEvent.type(screen.getByLabelText('Códigos de falla (opcional)'), 'P0301, p0300');
    await userEvent.click(screen.getByRole('radio', { name: 'Recomendado' }));
    await userEvent.click(await screen.findByRole('button', { name: 'Agregar hallazgo' }));
    await userEvent.click(screen.getByRole('button', { name: 'Quitar' }));
    await userEvent.click(screen.getByRole('button', { name: 'Cotizar esto' }));

    await waitFor(() => expect(calls.some((c) => c.method === 'DELETE')).toBe(true));
    expect(calls.find((c) => c.method === 'POST')?.body).toEqual({
      sistema: 'motor', descripcion: 'Falla de encendido en el cilindro 1', gravedad: 'recomendado', codigosFalla: ['P0301', 'p0300'],
    });
    expect(pedidos).toEqual(['Frenos']);
  });

  it('después de cotizar no deja quitar hallazgos', async () => {
    mockApi({ 'GET /me': owner, 'GET /orders/{id}/findings': { hallazgos: [hallazgo] } });
    renderPage(<DiagnosisPanel orden={orden({ estado: 'esperando_aprobacion' }) as unknown as Orden} />);
    expect(await screen.findByText('Pastillas al 10 %')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Quitar' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Agregar hallazgo' })).not.toBeInTheDocument();
  });

  it('guarda la mano de obra, "Cotizar esto" la pasa a la cotización y sube fotos del hallazgo', async () => {
    const cotizador = { ...owner, permisos: [...owner.permisos, 'quote:edit'] };
    const calls = mockApi({
      'GET /me': cotizador,
      'GET /orders/{id}/findings': { hallazgos: [{ ...hallazgo, manoObra: 60000 }] },
      'GET /orders/{id}/evidence': { evidencias: [] },
      'POST /orders/{id}/findings': { status: 201, body: hallazgo },
      'POST /orders/{id}/quote/lines': { cotizacion: null },
      'POST /orders/{id}/evidence': { status: 201, body: { id: 'e1' } },
    });
    renderPage(<DiagnosisPanel orden={orden({ estado: 'en_diagnostico' }) as unknown as Orden} />);
    expect(await screen.findByText(/Mano de obra \$\s?60\.000/)).toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText('Sistema'), 'suspension');
    await userEvent.type(screen.getByLabelText('Qué encontraste'), 'Bujes gastados');
    await userEvent.type(screen.getByLabelText('Mano de obra estimada (COP)'), '45.000');
    await userEvent.click(screen.getByRole('radio', { name: 'Puede esperar' }));
    await userEvent.click(screen.getByRole('button', { name: 'Agregar hallazgo' }));
    await userEvent.click(screen.getByRole('button', { name: 'Cotizar esto' }));
    const fotos = [1, 2].map((i) => new File([new Uint8Array([0xff, 0xd8, 0xff, i])], `h${i}.jpg`, { type: 'image/jpeg' }));
    await userEvent.upload(screen.getByLabelText('Fotos de Pastillas al 10 %'), fotos);

    await waitFor(() => expect(calls.some((c) => c.method === 'POST' && c.path.endsWith('/evidence'))).toBe(true));
    expect(calls.find((c) => c.path.endsWith('/findings') && c.method === 'POST')?.body).toMatchObject({ sistema: 'suspension', manoObra: 45000 });
    expect(calls.find((c) => c.path.endsWith('/quote/lines'))?.body).toEqual({
      tipo: 'servicio', descripcion: 'Mano de obra · Frenos: Pastillas al 10 %', cantidad: 1, precioUnitario: 60000,
    });
    const envio = String(calls.find((c) => c.path.endsWith('/evidence') && c.method === 'POST')?.body);
    expect(envio).toContain('h1');
    expect(envio.match(/name="foto"/g)).toHaveLength(2);
    expect(envio).toContain('name="hallazgoId"');
  });
});
