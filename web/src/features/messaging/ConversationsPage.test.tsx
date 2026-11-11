import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Route, Routes } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { mockApi, renderPage } from '../../test/api';
import { ConversationPage, ConversationsPage } from './ConversationsPage';

afterEach(() => vi.unstubAllGlobals());

const conv = (over: object) => ({ id: 'c1', celular: '3001234567', modo: 'agente', pideHumano: false, clienteId: null, ventanaAbiertaHasta: null, ...over });
const enUnaHora = new Date(Date.now() + 3_600_000).toISOString();

describe('ConversationsPage (HU-59)', () => {
  it('pone primero a quien escribió de último y marca a quien pidió una persona', async () => {
    mockApi({
      'GET /conversations': {
        conversaciones: [
          conv({ id: 'c2', celular: '3002222222', modo: 'humano', pideHumano: true, ultimoMensaje: 'quiero hablar con alguien', ultimoEn: '2026-11-04T15:00:00Z' }),
          conv({ id: 'c1', celular: '3001111111', ultimoMensaje: 'gracias', ultimoEn: '2026-11-04T16:00:00Z', nombre: 'Laura Gómez' }),
          conv({ id: 'c3', celular: '3003333333', ultimoMensaje: 'hola', ultimoEn: '2026-11-04T14:00:00Z' }),
        ],
      },
    });
    renderPage(<ConversationsPage />);
    const links = await screen.findAllByRole('link');
    expect(links.map((l) => l.getAttribute('href'))).toEqual(['/panel/conversaciones/c1', '/panel/conversaciones/c2', '/panel/conversaciones/c3']);
    expect(links[0]).toHaveTextContent('300 111 1111');
    expect(links[0]).toHaveTextContent('Laura Gómez');
    expect(links[1]).toHaveTextContent('Pidió una persona');
  });

  it('si WhatsApp ocultó el número, muestra el nombre y no el id interno', async () => {
    mockApi({
      'GET /conversations': { conversaciones: [conv({ id: 'c4', celular: '123456789012345@lid', nombre: 'Belén', ultimoEn: '2026-11-04T16:00:00Z' })] },
    });
    renderPage(<ConversationsPage />);
    const [link] = await screen.findAllByRole('link');
    expect(link).toHaveTextContent('Belén');
    expect(link).toHaveTextContent('Número oculto por WhatsApp');
    expect(link).not.toHaveTextContent('@lid');
  });
});

describe('ConversationPage (HU-59)', () => {
  const page = () =>
    renderPage(
      <Routes>
        <Route path="/panel/conversaciones/:id" element={<ConversationPage />} />
      </Routes>,
      '/panel/conversaciones/c2',
    );

  it('responde como persona y devuelve la conversación al asistente', async () => {
    const calls = mockApi({
      'GET /conversations/{id}': {
        conversacion: conv({ id: 'c2', modo: 'humano', pideHumano: true, ventanaAbiertaHasta: enUnaHora }),
        mensajes: [{ id: 'm1', direccion: 'entrante', tipo: 'texto', cuerpo: 'quiero hablar con alguien', estado: 'recibido', origen: 'cliente', creadoEn: '2026-11-04T15:00:00Z' }],
      },
      'POST /conversations/{id}/messages': { status: 201, body: { id: 'm2' } },
      'PATCH /conversations/{id}': { status: 204 },
    });
    page();
    expect(await screen.findByText('quiero hablar con alguien')).toBeInTheDocument();
    expect(screen.getByText(/pidió hablar con una persona/)).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText('Tu respuesta'), 'Hola, soy Rosa. ¿En qué te ayudo?');
    await userEvent.click(screen.getByRole('button', { name: 'Enviar' }));
    await waitFor(() =>
      expect(calls.find((c) => c.method === 'POST')).toMatchObject({ path: '/conversations/c2/messages', body: { texto: 'Hola, soy Rosa. ¿En qué te ayudo?' } }),
    );

    await userEvent.click(screen.getByRole('button', { name: 'Devolver al asistente' }));
    await waitFor(() => expect(calls.find((c) => c.method === 'PATCH')).toMatchObject({ path: '/conversations/c2', body: { modo: 'agente' } }));
  });

  it('con la ventana de 24 h cerrada no deja enviar y dice por qué', async () => {
    mockApi({
      'GET /conversations/{id}': { conversacion: conv({ id: 'c2', modo: 'humano', ventanaAbiertaHasta: '2026-01-01T00:00:00Z' }), mensajes: [] },
    });
    page();
    expect(await screen.findByText(/no ha escrito en 24 horas/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Enviar' })).toBeDisabled();
  });
});
