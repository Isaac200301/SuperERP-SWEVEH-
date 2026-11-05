import { act, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Sesion } from '../../shared/api/client';
import { mockApi, owner, renderPage } from '../../test/api';
import { PanelLayout } from '../panel/PanelLayout';
import { PENDING_POLL_MS } from './usePendingCards';

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe('Aviso de tarjetas nuevas (HU-66)', () => {
  it('muestra el contador en Por revisar y suena cuando llega una tarjeta', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const sounds = vi.fn();
    vi.stubGlobal('AudioContext', class {
      currentTime = 0;
      destination = {};
      createOscillator() {
        sounds();
        return { frequency: {}, connect: (g: unknown) => g, start() {}, stop() {}, onended: null };
      }
      createGain() {
        return { gain: { setValueAtTime() {}, exponentialRampToValueAtTime() {} }, connect: () => ({}) };
      }
      close() {}
    });
    let pendientes = 1;
    mockApi({ 'GET /inbox/stats': () => ({ enviados: 0, editados: 0, porcentajeEditados: 0, pendientes }) });
    renderPage(<PanelLayout session={{ ...owner, permisos: [...owner.permisos, 'inbox:manage'] } as unknown as Sesion} />);

    await waitFor(() => expect(screen.getByRole('link', { name: /^Por revisar/ })).toHaveTextContent('Por revisar1 por revisar'));
    expect(sounds).not.toHaveBeenCalled(); // la primera carga no suena
    expect(document.title).toMatch(/^\(1\)/);

    pendientes = 2;
    await act(async () => {
      await vi.advanceTimersByTimeAsync(PENDING_POLL_MS + 100);
    });
    await waitFor(() => expect(screen.getByRole('link', { name: /^Por revisar/ })).toHaveTextContent('Por revisar2 por revisar'));
    expect(sounds).toHaveBeenCalledTimes(1);
  });
});
