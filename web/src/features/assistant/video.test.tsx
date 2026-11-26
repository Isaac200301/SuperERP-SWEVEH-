import { act, fireEvent, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { mockApi, renderPage } from '../../test/api';
import { OrderAssistantCard } from './OrderAssistantCard';

// R9 (V-12, HU-108, ADR-0042): recibir con un video. El video no sale del celular: solo el audio y los cuadros.
// Datos Ficticio.

const guia = (vistos: string[]) =>
  [
    ['placa', 'Placa'],
    ['tablero', 'Tablero'],
    ['frente', 'Frente'],
    ['izquierdo', 'Lado izq.'],
    ['trasera', 'Atrás'],
    ['derecho', 'Lado der.'],
  ].map(([clave, nombre]) => ({ clave, nombre, visto: vistos.includes(clave ?? '') }));

const chat = () => ({
  id: 'c9',
  ordenId: 'o7',
  version: 2,
  propuesta: { ordenId: 'o7', ordenCodigo: 'OS-0007', ordenEstado: 'por_recibir', placa: { valor: 'KLM482', origen: 'taller' }, kilometraje: { valor: '45000', origen: 'video' } },
  pendiente: true,
  progreso: { tiene: 5, total: 8, falta: ['la parte trasera'], guia: guia(['placa', 'tablero', 'frente']) },
  disponible: true,
  voz: false,
  mensajes: [
    {
      id: 'm1', rol: 'usuario', texto: 'tiene 45 mil kilómetros', opciones: [], creadoEn: '2026-11-27T15:00:00Z',
      adjuntos: [{ tipo: 'video', segundos: 62 }, { tipo: 'foto', url: '/api/v1/x/1' }, { tipo: 'foto', url: '/api/v1/x/2' }],
    },
    { id: 'm2', rol: 'asistente', texto: 'Anoté el kilometraje. No vi la parte trasera.', adjuntos: [], opciones: [], creadoEn: '2026-11-27T15:00:01Z', conPropuesta: true },
  ],
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('Recibir con un video (R9)', () => {
  it('sin cámara usa el recorte: fotos por lado al mismo endpoint, con su clave, y la guía marca lo visto', async () => {
    const keys: string[] = [];
    const calls = mockApi({
      'GET /orders/{id}/assistant': { chat: null, disponible: true, voz: false },
      'POST /orders/{id}/reception/video': (req: Request) => {
        keys.push(req.headers.get('Idempotency-Key') ?? '');
        return { chat: chat() };
      },
    });
    renderPage(<OrderAssistantCard ordenId="o7" estado="por_recibir" />);
    expect(await screen.findByRole('heading', { name: 'Recibir con un video' })).toBeInTheDocument();
    expect(screen.getByText('Placa').closest('li')).toHaveTextContent('falta');
    expect(screen.queryByRole('button', { name: /Grabar/ })).not.toBeInTheDocument(); // jsdom no graba

    const fotos = [new File(['a'], 'frente.jpg', { type: 'image/jpeg' }), new File(['b'], 'placa.jpg', { type: 'image/jpeg' })];
    await userEvent.upload(screen.getByLabelText('Tomar fotos por lado'), fotos);

    await waitFor(() => expect(screen.getAllByText('Placa')[0]?.closest('li')).toHaveTextContent('visto'));
    const post = calls.find((c) => c.method === 'POST');
    expect(post?.path).toBe('/orders/o7/reception/video');
    expect(String(post?.body).match(/name="cuadros"/g)).toHaveLength(2);
    expect(String(post?.body)).not.toMatch(/name="video"/);
    expect(keys[0]).toMatch(/^[0-9a-f-]{36}$/);
    expect(screen.getByText('Atrás').closest('li')).toHaveTextContent('falta');
    expect(screen.getByText('del video')).toBeInTheDocument(); // el origen del kilometraje
    expect(screen.getByText('Video de recepción · 1:02 · 2 cuadros')).toBeInTheDocument();
  });

  it('graba con contador, toma un cuadro cada 10 s, corta a los 90 s y manda audio y 10 cuadros', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: false });
    const stop = vi.fn();
    const track = { stop, kind: 'audio' };
    vi.stubGlobal('MediaStream', class { constructor(public t: unknown[]) {} getTracks() { return this.t; } });
    class FakeRecorder {
      state = 'inactive';
      mimeType = 'audio/webm';
      ondataavailable: ((e: { data: Blob }) => void) | null = null;
      onstop: (() => void) | null = null;
      start() { this.state = 'recording'; }
      stop() { this.state = 'inactive'; this.ondataavailable?.({ data: new Blob(['audio'], { type: 'audio/webm' }) }); this.onstop?.(); }
    }
    vi.stubGlobal('MediaRecorder', FakeRecorder);
    Object.defineProperty(navigator, 'mediaDevices', {
      configurable: true,
      value: { getUserMedia: vi.fn().mockResolvedValue({ getTracks: () => [track], getAudioTracks: () => [track] }) },
    });
    vi.spyOn(HTMLVideoElement.prototype, 'videoWidth', 'get').mockReturnValue(1920);
    vi.spyOn(HTMLVideoElement.prototype, 'videoHeight', 'get').mockReturnValue(1080);
    vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined);
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({ drawImage: () => undefined } as unknown as CanvasRenderingContext2D);
    vi.spyOn(HTMLCanvasElement.prototype, 'toBlob').mockImplementation(function (cb: BlobCallback) {
      cb(new Blob(['jpg'], { type: 'image/jpeg' }));
    });
    const calls = mockApi({
      'GET /orders/{id}/assistant': { chat: null, disponible: true, voz: false },
      'POST /orders/{id}/reception/video': { chat: chat() },
    });
    renderPage(<OrderAssistantCard ordenId="o7" estado="por_recibir" />);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10);
    });
    fireEvent.click(screen.getByRole('button', { name: 'Grabar recepción' }));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10);
    });
    expect(screen.getByRole('timer')).toHaveTextContent('0:00 / 1:30');
    await act(async () => {
      await vi.advanceTimersByTimeAsync(30_000);
    });
    expect(screen.getByRole('timer')).toHaveTextContent('0:30 / 1:30');
    await act(async () => {
      await vi.advanceTimersByTimeAsync(61_000); // pasa de 1:30: se corta solo
    });
    vi.useRealTimers();
    await waitFor(() => expect(calls.some((c) => c.method === 'POST')).toBe(true));
    const body = String(calls.find((c) => c.method === 'POST')?.body);
    expect(body).toMatch(/name="segundos"\r\n\r\n90/);
    expect(body.match(/name="cuadros"/g)).toHaveLength(10);
    expect(body).toMatch(/name="audio"/);
    expect(stop).toHaveBeenCalled(); // la cámara y el micrófono se apagan
    expect(await screen.findByRole('button', { name: 'Grabar otro video' })).toBeInTheDocument();
  });

  it('solo se ofrece mientras la orden está por recibir', async () => {
    mockApi({ 'GET /orders/{id}/assistant': { chat: null, disponible: true, voz: false } });
    renderPage(<OrderAssistantCard ordenId="o7" estado="recibido" />);
    expect(await screen.findByText(/Cuéntame qué llegó/)).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Recibir con un video' })).not.toBeInTheDocument();
  });
});
