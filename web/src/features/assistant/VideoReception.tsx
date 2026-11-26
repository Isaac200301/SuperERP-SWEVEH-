import { useEffect, useRef, useState } from 'react';

import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { IconCamera, IconCheck, IconVideo } from '../../shared/ui/icons';
import { reducir, useSendVideo, type ChatPiso } from './api';

/** Lo que dura el video como máximo, cada cuánto se toma un cuadro y cuántos cuadros van (V-12, ADR-0042). */
export const VIDEO_MAX = 90;
const CADA = 10;
const MAX_CUADROS = 10;
const LADO_CUADRO = 1280;

type Paso = NonNullable<ChatPiso['progreso']['guia']>[number];

/** La guía de lados antes de grabar (Figma 08): placa, tablero y la vuelta al vehículo. */
export const GUIA_BASE: Paso[] = [
  { clave: 'placa', nombre: 'Placa', visto: false },
  { clave: 'tablero', nombre: 'Tablero', visto: false },
  { clave: 'frente', nombre: 'Frente', visto: false },
  { clave: 'izquierdo', nombre: 'Lado izq.', visto: false },
  { clave: 'trasera', nombre: 'Atrás', visto: false },
  { clave: 'derecho', nombre: 'Lado der.', visto: false },
];

/** ¿Este navegador puede grabar con la cámara? Si no, se usa el recorte previsto: fotos por lado. */
export function puedeGrabar() {
  return typeof MediaRecorder !== 'undefined' && typeof navigator !== 'undefined' && typeof navigator.mediaDevices?.getUserMedia === 'function';
}

const mmss = (s: number) => `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;

/** Toma el cuadro que se ve en el video, de 1280 px como máximo, en JPEG. */
function cuadro(v: HTMLVideoElement): Promise<Blob | null> {
  const w = v.videoWidth;
  const h = v.videoHeight;
  if (!w || !h) return Promise.resolve(null);
  const escala = Math.min(1, LADO_CUADRO / Math.max(w, h));
  const c = document.createElement('canvas');
  c.width = Math.round(w * escala);
  c.height = Math.round(h * escala);
  const ctx = c.getContext('2d');
  if (!ctx) return Promise.resolve(null);
  ctx.drawImage(v, 0, 0, c.width, c.height);
  return new Promise((ok) => c.toBlob((b) => ok(b), 'image/jpeg', 0.8));
}

type Envio = { segundos: number; audio?: Blob | undefined; cuadros: Blob[]; clave: string };

type Props = Readonly<{ ordenId: string; chat: ChatPiso | null; onChat?: ((c: ChatPiso) => void) | undefined }>;

/**
 * Recibir con un video (R9, V-12, HU-108; Figma 08): hasta 90 s con contador y guía de lados. Mientras graba, se queda
 * con el audio y un cuadro cada 10 s; el video completo nunca sale del celular (Ley 1581). El asistente arma la
 * propuesta en la ficha en vivo y la guía marca lo que vio. Si el navegador no graba, se toman fotos por lado.
 */
export function VideoReception({ ordenId, chat, onChat }: Props) {
  const send = useSendVideo();
  const [grabando, setGrabando] = useState(false);
  const [segundos, setSegundos] = useState(0);
  const [error, setError] = useState('');
  const video = useRef<HTMLVideoElement>(null);
  const stream = useRef<MediaStream | null>(null);
  const rec = useRef<MediaRecorder | null>(null);
  const cuadros = useRef<Blob[]>([]);
  const reloj = useRef<number | null>(null);
  const ultimo = useRef<Envio | null>(null); // para reintentar con la misma clave
  const guia = chat?.progreso.guia ?? GUIA_BASE;
  const yaGrabo = (chat?.mensajes ?? []).some((m) => m.adjuntos.some((a) => a.tipo === 'video'));

  const tick = useRef(0); // segundos grabados (el estado es para pintar; el ref, para leerlo al cerrar)

  useEffect(
    () => () => {
      if (reloj.current !== null) window.clearInterval(reloj.current);
      stream.current?.getTracks().forEach((t) => t.stop());
    },
    [],
  );

  function apagar() {
    if (reloj.current !== null) window.clearInterval(reloj.current);
    reloj.current = null;
    stream.current?.getTracks().forEach((t) => t.stop());
    stream.current = null;
  }

  async function enviar(e: Envio) {
    ultimo.current = e;
    try {
      onChat?.(await send.mutateAsync({ ordenId, ...e }));
      ultimo.current = null;
    } catch {
      // el error se muestra con send.isError y "Reintentar" manda lo mismo con la misma clave
    }
  }

  async function tomar() {
    const v = video.current;
    if (!v || cuadros.current.length >= MAX_CUADROS) return;
    const b = await cuadro(v);
    if (b) cuadros.current.push(b);
  }

  async function empezar() {
    setError('');
    try {
      const s = await navigator.mediaDevices.getUserMedia({ video: { facingMode: 'environment', width: { ideal: LADO_CUADRO } }, audio: true });
      stream.current = s;
      cuadros.current = [];
      if (video.current) {
        video.current.srcObject = s;
        void video.current.play?.();
      }
      const partes: Blob[] = [];
      const audio = s.getAudioTracks();
      const r = audio.length > 0 ? new MediaRecorder(new MediaStream(audio)) : null;
      if (r) {
        r.ondataavailable = (ev) => partes.push(ev.data);
        r.onstop = () => void cerrar(r.mimeType, partes);
        r.start();
      }
      rec.current = r;
      tick.current = 0;
      setSegundos(0);
      setGrabando(true);
      reloj.current = window.setInterval(() => {
        tick.current += 1;
        setSegundos(tick.current);
        if (tick.current % CADA === 1) void tomar();
        if (tick.current >= VIDEO_MAX) terminar();
      }, 1000);
    } catch {
      apagar();
      setError('No pudimos usar la cámara. Revisa el permiso del navegador o toma las fotos por lado.');
    }
  }

  function terminar() {
    if (reloj.current !== null) window.clearInterval(reloj.current);
    reloj.current = null;
    setGrabando(false);
    if (rec.current && rec.current.state !== 'inactive') rec.current.stop(); // onstop cierra y manda
    else void cerrar('', []);
  }

  async function cerrar(mime: string, partes: Blob[]) {
    await tomar(); // el último cuadro, por si el recorrido terminó en un lado
    const dur = Math.min(tick.current, VIDEO_MAX);
    apagar();
    const audio = partes.length > 0 ? new Blob(partes, { type: mime || 'audio/webm' }) : undefined;
    if (cuadros.current.length === 0) {
      setError('No se alcanzó a tomar ningún cuadro. Graba otra vez apuntando al vehículo.');
      return;
    }
    await enviar({ segundos: dur, audio, cuadros: cuadros.current, clave: crypto.randomUUID() });
  }

  async function fotos(files: File[]) {
    if (files.length === 0) return;
    const reducidas = await Promise.all(files.slice(0, MAX_CUADROS).map((f) => reducir(f, LADO_CUADRO)));
    await enviar({ segundos: 0, cuadros: reducidas, clave: crypto.randomUUID() });
  }

  return (
    <section aria-labelledby={`video-${ordenId}`} className="flex flex-col gap-3 rounded-[22px] bg-white p-4">
      <div className="flex flex-col">
        <h2 id={`video-${ordenId}`} className="text-[16px] font-extrabold">
          Recibir con un video
        </h2>
        <span className="text-[13px] text-ink-2">
          Hasta 1:30. Muestra la placa y el tablero, y da la vuelta al vehículo contando lo que ves. El video no se guarda: solo el audio y algunos cuadros.
        </span>
      </div>

      <ol aria-label="Guía de lados" className="flex flex-wrap gap-1.5">
        {guia.map((g) => (
          <li
            key={g.clave}
            className={`inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-[13px] font-bold ${g.visto ? 'bg-ok-tint text-ok-ink' : 'bg-surface text-ink-2'}`}
          >
            {g.visto ? <IconCheck size={14} /> : <span aria-hidden="true">○</span>}
            {g.nombre}
            <span className="font-semibold">{g.visto ? '· visto' : '· falta'}</span>
          </li>
        ))}
      </ol>

      <video
        ref={video}
        muted
        playsInline
        aria-label="Lo que ve la cámara"
        className={grabando ? 'aspect-video w-full rounded-xl bg-ink object-cover' : 'hidden'}
      />

      {grabando ? (
        <div className="flex items-center gap-3">
          <span role="timer" aria-live="off" className="font-mono text-lg font-extrabold text-err-ink">
            ● {mmss(segundos)} / {mmss(VIDEO_MAX)}
          </span>
          <span className="flex-1" />
          <Button variant="secondary" onClick={terminar}>
            Terminar
          </Button>
        </div>
      ) : (
        <div className="flex flex-wrap items-center gap-2">
          {puedeGrabar() ? (
            <Button variant="secondary" icon={<IconVideo size={18} />} busy={send.isPending} onClick={() => void empezar()}>
              {yaGrabo ? 'Grabar otro video' : 'Grabar recepción'}
            </Button>
          ) : null}
          <label className="inline-flex min-h-[44px] cursor-pointer items-center gap-1.5 rounded-btn bg-surface px-3 text-sm font-bold text-primary">
            <IconCamera size={18} />
            {puedeGrabar() ? 'O toma fotos por lado' : 'Tomar fotos por lado'}
            <input
              type="file"
              accept="image/jpeg,image/png,image/webp"
              capture="environment"
              multiple
              disabled={send.isPending}
              className="sr-only"
              onChange={(e) => {
                const list = Array.from(e.target.files ?? []);
                e.target.value = '';
                void fotos(list);
              }}
            />
          </label>
        </div>
      )}

      {send.isPending ? (
        <p role="status" className="text-sm text-ink-2">
          El asistente está revisando el video…
        </p>
      ) : null}
      {error ? <Alert>{error}</Alert> : null}
      {send.isError ? (
        <Alert>
          {send.error.message}{' '}
          {ultimo.current ? (
            <button type="button" className="min-h-[44px] font-bold text-primary underline" onClick={() => ultimo.current && void enviar(ultimo.current)}>
              Reintentar
            </button>
          ) : null}
        </Alert>
      ) : null}
    </section>
  );
}
