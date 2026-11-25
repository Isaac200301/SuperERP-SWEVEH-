import { fireEvent, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Route, Routes } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { mockApi, owner, renderPage } from '../../test/api';
import { ChangePinPage } from './ChangePinPage';
import { LoginPage } from './LoginPage';
import { RegisterPage } from './RegisterPage';

afterEach(() => vi.unstubAllGlobals());

// R2 (HU-104, V-03, V-04): entrar con celular y PIN. Datos Ficticio.

const typePin = (label: string, pin: string) => userEvent.type(screen.getByLabelText(`${label}: número 1 de 4`), pin);

const conDestino = (ui: React.ReactElement, path: string) =>
  renderPage(
    <Routes>
      <Route path="/entrar" element={ui} />
      <Route path="/registro" element={ui} />
      <Route path="/panel" element={<p>Panel</p>} />
      <Route path="/panel/arranque" element={<p>Arranque</p>} />
      <Route path="/panel/cambiar-pin" element={<p>Cambiar PIN</p>} />
    </Routes>,
    path,
  );

describe('PinInput', () => {
  it('salta de casilla, borra hacia atrás y pegar llena las cuatro', async () => {
    mockApi({});
    conDestino(<LoginPage />, '/entrar');
    const uno = screen.getByLabelText('Tu PIN: número 1 de 4');
    await userEvent.type(uno, '25');
    expect(screen.getByLabelText('Tu PIN: número 3 de 4')).toHaveFocus();
    await userEvent.keyboard('{Backspace}');
    expect(screen.getByLabelText('Tu PIN: número 2 de 4')).toHaveFocus();
    fireEvent.paste(uno, { clipboardData: { getData: () => '9173' } });
    for (const [i, d] of ['9', '1', '7', '3'].entries()) expect(screen.getByLabelText(`Tu PIN: número ${i + 1} de 4`)).toHaveValue(d);
    // Se muestran puntos, no los números.
    expect(uno).toHaveAttribute('type', 'password');
    expect(uno).toHaveAttribute('inputmode', 'numeric');
  });
});

describe('LoginPage (HU-104)', () => {
  it('entra con celular y PIN y muestra el error del servidor tal cual', async () => {
    const calls = mockApi({
      'POST /auth/login': (req: Request) =>
        calls.length === 1
          ? { status: 422, body: { code: 'ingreso_fallido', detail: 'Celular o PIN incorrectos. Te quedan 4 intentos; luego se bloquea 15 minutos.' } }
          : { ...owner, cambiarPin: false, _req: req.url },
    });
    conDestino(<LoginPage />, '/entrar');
    await userEvent.click(screen.getByRole('button', { name: 'Entrar' }));
    expect(screen.getByText('Escribe un celular colombiano de 10 dígitos que empiece por 3.')).toBeInTheDocument();
    expect(calls).toHaveLength(0);

    await userEvent.type(screen.getByLabelText('Tu celular'), '300 000 0001');
    await typePin('Tu PIN', '9999');
    await userEvent.click(screen.getByRole('button', { name: 'Entrar' }));
    expect(await screen.findByText(/Celular o PIN incorrectos. Te quedan 4 intentos/)).toBeInTheDocument();
    expect(calls[0]?.body).toEqual({ celular: '3000000001', pin: '9999' });
    // El PIN equivocado se borra para volver a escribirlo.
    expect(screen.getByLabelText('Tu PIN: número 1 de 4')).toHaveValue('');

    await typePin('Tu PIN', '2580');
    await userEvent.click(screen.getByRole('button', { name: 'Entrar' }));
    expect(await screen.findByText('Panel')).toBeInTheDocument();
  });

  it('con un PIN temporal va a escoger uno propio', async () => {
    mockApi({ 'POST /auth/login': { ...owner, cambiarPin: true } });
    conDestino(<LoginPage />, '/entrar');
    await userEvent.type(screen.getByLabelText('Tu celular'), '3000000002');
    await typePin('Tu PIN', '4826');
    await userEvent.click(screen.getByRole('button', { name: 'Entrar' }));
    expect(await screen.findByText('Cambiar PIN')).toBeInTheDocument();
  });
});

describe('RegisterPage (V-04, HU-02)', () => {
  it('pide celular, taller, PIN repetido y la casilla sin marcar; manda la versión de la política', async () => {
    const calls = mockApi({ 'POST /auth/register': { status: 201, body: { ...owner, cuentaNueva: true } } });
    conDestino(<RegisterPage />, '/registro');
    expect(screen.getByText('PASO 1 DE 2')).toBeInTheDocument();
    const casilla = screen.getByRole('checkbox');
    expect(casilla).not.toBeChecked();

    await userEvent.type(screen.getByLabelText('Tu celular'), '3000000003');
    await userEvent.type(screen.getByLabelText('Nombre del taller'), 'Servitaller El Pistón');
    await typePin('Crea tu PIN', '1234');
    await typePin('Repite el PIN', '1234');
    await userEvent.click(screen.getByRole('button', { name: 'Seguir' }));
    expect(screen.getByText('Ese PIN es muy fácil de adivinar. Escoge otros 4 números.')).toBeInTheDocument();
    expect(screen.getByText('Para continuar debes aceptar la política de tratamiento de datos.')).toBeInTheDocument();
    expect(calls).toHaveLength(0);

    await typePin('Crea tu PIN', '2580');
    await typePin('Repite el PIN', '2581');
    expect(screen.getByText('Los dos PIN no coinciden.')).toBeInTheDocument();
    await typePin('Repite el PIN', '2580');
    await userEvent.click(casilla);
    await userEvent.click(screen.getByRole('button', { name: 'Seguir' }));
    expect(await screen.findByText('Arranque')).toBeInTheDocument();
    expect(calls[0]?.body).toEqual({
      celular: '3000000003', nombreTaller: 'Servitaller El Pistón', pin: '2580',
      autorizacion: { version: 'borrador-sin-validar-2026-11-01', aceptada: true },
    });
  });

  it('si el celular ya tiene cuenta, ofrece entrar', async () => {
    mockApi({ 'POST /auth/register': { status: 409, body: { code: 'cuenta_existente', detail: 'Ese celular ya tiene cuenta. Entra con tu celular y tu PIN.' } } });
    conDestino(<RegisterPage />, '/registro');
    await userEvent.type(screen.getByLabelText('Tu celular'), '3000000004');
    await userEvent.type(screen.getByLabelText('Nombre del taller'), 'Taller');
    await typePin('Crea tu PIN', '2580');
    await typePin('Repite el PIN', '2580');
    await userEvent.click(screen.getByRole('checkbox'));
    await userEvent.click(screen.getByRole('button', { name: 'Seguir' }));
    expect(await screen.findByText(/Ese celular ya tiene cuenta/)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Entrar' })).toHaveAttribute('href', '/entrar');
  });
});

describe('ChangePinPage (HU-104)', () => {
  it('con un PIN temporal no pide el actual', async () => {
    const calls = mockApi({ 'GET /me': { ...owner, cambiarPin: true }, 'POST /auth/pin': { status: 204 } });
    renderPage(
      <Routes>
        <Route path="/panel/cambiar-pin" element={<ChangePinPage />} />
        <Route path="/panel" element={<p>Panel</p>} />
      </Routes>,
      '/panel/cambiar-pin',
    );
    expect(await screen.findByText('Escoge tu PIN')).toBeInTheDocument();
    expect(screen.queryByText('PIN actual')).not.toBeInTheDocument();
    await typePin('PIN nuevo', '7391');
    await typePin('Repite el PIN nuevo', '7391');
    await userEvent.click(screen.getByRole('button', { name: 'Guardar mi PIN' }));
    expect(await screen.findByText('Panel')).toBeInTheDocument();
    await waitFor(() => expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ pin: '7391' }));
  });

  it('con su propio PIN pide el actual', async () => {
    const calls = mockApi({ 'GET /me': { ...owner, cambiarPin: false }, 'POST /auth/pin': { status: 204 } });
    renderPage(<ChangePinPage />, '/panel/cambiar-pin');
    expect(await screen.findByText('Cambiar mi PIN')).toBeInTheDocument();
    await typePin('PIN actual', '2580');
    await typePin('PIN nuevo', '7391');
    await typePin('Repite el PIN nuevo', '7391');
    await userEvent.click(screen.getByRole('button', { name: 'Guardar mi PIN' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ pinActual: '2580', pin: '7391' }));
  });
});
