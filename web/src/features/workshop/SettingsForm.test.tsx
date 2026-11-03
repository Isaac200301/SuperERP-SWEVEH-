import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { mockApi, renderPage } from '../../test/api';
import { SettingsForm } from './SettingsForm';

afterEach(() => vi.unstubAllGlobals());

const porDefecto = {
  usarEsperandoRepuesto: false, usarControlCalidad: false, fotosMinimas: 4, kmObligatorio: true,
  itemsCarro: [{ clave: 'luces', nombre: 'Luces' }, { clave: 'radio', nombre: 'Radio' }], itemsMoto: [{ clave: 'casco', nombre: 'Casco' }],
  ivaPct: 19, preciosConIva: true, tarifaHora: null, descuentoMaxPct: 10, descuentoRecepcion: false, vigenciaDias: 15, version: 0,
};

describe('SettingsForm (parámetros del taller)', () => {
  it('activa un estado, cambia IVA y tarifa, edita los ítems de recepción y guarda con la versión', async () => {
    const calls = mockApi({ 'GET /workshop/settings': porDefecto, 'PUT /workshop/settings': { ...porDefecto, version: 1 } });
    renderPage(<SettingsForm />);
    await userEvent.click(await screen.findByLabelText(/Usar el estado Esperando repuesto/));
    await userEvent.clear(screen.getByLabelText('IVA (%)'));
    await userEvent.type(screen.getByLabelText('IVA (%)'), '0');
    await userEvent.type(screen.getByLabelText('Tarifa por hora de mano de obra (COP)'), '80.000');
    await userEvent.click(screen.getByRole('button', { name: 'Quitar Radio de Ítems de carro' }));
    await userEvent.type(screen.getByLabelText('Agregar a Ítems de carro'), 'Extintor');
    await userEvent.click(screen.getAllByRole('button', { name: 'Agregar' })[0] as HTMLElement);
    await userEvent.click(screen.getByRole('button', { name: 'Guardar parámetros' }));
    expect(await screen.findByText('Parámetros guardados.')).toBeInTheDocument();
    await waitFor(() => expect(calls.find((c) => c.method === 'PUT')?.body).toMatchObject({
      usarEsperandoRepuesto: true, ivaPct: 0, tarifaHora: 80000, version: 0,
      itemsCarro: [{ clave: 'luces', nombre: 'Luces' }, { clave: '', nombre: 'Extintor' }],
    }));
  });
});
