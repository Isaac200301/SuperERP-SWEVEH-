import { useQuery } from '@tanstack/react-query';
import { useEffect, useRef } from 'react';

import { api, unwrap } from '../../shared/api/client';

/** Cada cuánto el panel pregunta por tarjetas nuevas (HU-66). Avisar por WhatsApp gastaría mensajes (Q-18). */
export const PENDING_POLL_MS = 20_000;

/** Un pitido corto con Web Audio: sin archivo de sonido. Si el navegador no deja sonar (sin interacción), no pasa nada. */
export function beep() {
  try {
    const Ctx = window.AudioContext ?? (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
    if (!Ctx) return;
    const ctx = new Ctx();
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();
    osc.frequency.value = 880;
    gain.gain.setValueAtTime(0.15, ctx.currentTime);
    gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.35);
    osc.connect(gain).connect(ctx.destination);
    osc.start();
    osc.stop(ctx.currentTime + 0.35);
    osc.onended = () => void ctx.close();
  } catch {
    // sin audio: el contador basta
  }
}

/**
 * HU-66: tarjetas por revisar para quien atiende la bandeja (dueño o recepción con inbox:manage). Devuelve el número
 * para el contador del menú; cuando sube, suena un aviso y el título de la pestaña lo muestra.
 */
export function usePendingCards(enabled: boolean, onNew: () => void = beep): number {
  const q = useQuery({
    queryKey: ['bandeja', 'pendientes'],
    enabled,
    refetchInterval: PENDING_POLL_MS,
    refetchIntervalInBackground: true,
    queryFn: async () => unwrap(await api.GET('/inbox/stats')).pendientes,
  });
  const n = q.data ?? 0;
  const prev = useRef<number | undefined>(undefined);
  useEffect(() => {
    if (q.data === undefined) return;
    if (prev.current !== undefined && q.data > prev.current) onNew();
    prev.current = q.data;
  }, [q.data, onNew]);
  useEffect(() => {
    const base = document.title.replace(/^\(\d+\) /, '');
    document.title = n > 0 ? `(${n}) ${base}` : base;
  }, [n]);
  return n;
}
