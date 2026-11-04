import { screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { mockApi, owner, renderPage } from '../../test/api';
import { PanelLayout } from '../panel/PanelLayout';
import { PlatformPage } from './PlatformPage';

afterEach(() => vi.unstubAllGlobals());

describe('PlatformPage (HU-74)', () => {
  it('muestra cada taller con "34 de 1.000" e "IA $1.200" y marca al que se pasa', async () => {
    mockApi({
      'GET /me': { ...owner, adminPlataforma: true },
      'GET /platform/usage': {
        mes: '2026-11',
        incluidos: 1000,
        pesosPorDolar: 4000,
        talleres: [
          {
            id: 't1',
            nombre: 'Servitaller El Pistón',
            mensajes: 34,
            incluidos: 1000,
            mensajesTexto: '34 de 1.000',
            iaMicroUsd: 300000,
            iaPesos: 1200,
            iaTexto: 'IA $1.200',
            sePasa: false,
          },
          {
            id: 't2',
            nombre: 'Motos Rápidas',
            mensajes: 1203,
            incluidos: 1000,
            mensajesTexto: '1.203 de 1.000',
            iaMicroUsd: 0,
            iaPesos: 0,
            iaTexto: 'IA $0',
            sePasa: true,
          },
        ],
      },
    });
    renderPage(<PlatformPage />);
    expect(await screen.findByText('34 de 1.000')).toBeInTheDocument();
    expect(screen.getByText('IA $1.200')).toBeInTheDocument();
    expect(screen.getByText(/1\.203 de 1\.000/)).toHaveTextContent('se pasó');
  });

  it('un usuario de taller recibe "sin permiso"', async () => {
    mockApi({
      'GET /me': owner,
      'GET /platform/usage': {
        status: 403,
        body: { code: 'sin_permiso', detail: 'No tienes permiso para ver esto.', status: 403, title: 'Forbidden' },
      },
    });
    renderPage(<PlatformPage />);
    expect(await screen.findByText(/No tienes permiso/)).toBeInTheDocument();
  });
});

describe('PanelLayout (HU-74)', () => {
  it('solo el administrador del ERP ve "Plataforma"', () => {
    const { unmount } = renderPage(<PanelLayout session={{ ...owner, adminPlataforma: false } as never} />);
    expect(screen.queryByRole('link', { name: /Plataforma/ })).not.toBeInTheDocument();
    unmount();
    renderPage(<PanelLayout session={{ ...owner, adminPlataforma: true } as never} />);
    expect(screen.getByRole('link', { name: /Plataforma/ })).toHaveAttribute('href', '/panel/plataforma');
  });
});
