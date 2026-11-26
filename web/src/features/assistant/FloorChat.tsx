import { useEffect, useRef, useState, type FormEvent } from 'react';
import { Link } from 'react-router';

import { formatoCOP, formatoHora, pesos } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { IconCamera, IconX } from '../../shared/ui/icons';
import { Plate } from '../../shared/ui/Plate';
import { useConfirmFloor, useEditFloor, useSendFloor, type ChatPiso, type MensajePiso, type PropuestaPiso } from './api';

/** De dónde salió cada dato (HU-64), en palabras del taller. */
const ORIGEN: Record<string, string> = { dijiste: 'lo dijiste', foto: 'de la foto', video: 'del video', lista: 'de tu lista', pusiste: 'lo pusiste', taller: 'ya estaba' };

const LADO: Record<string, string> = { frente: 'Frente', trasera: 'Atrás', izquierdo: 'Lado izq.', derecho: 'Lado der.', tablero: 'Tablero' };

const ESTADO: Record<string, string> = {
  recibido: 'Recibido', en_diagnostico: 'En diagnóstico', esperando_aprobacion: 'Esperando aprobación', en_reparacion: 'En reparación',
  esperando_repuesto: 'Esperando repuesto', control_calidad: 'Control de calidad', listo: 'Listo',
};

const FICHA: Record<string, string> = {
  marca: 'Marca', linea: 'Línea', anio: 'Año', color: 'Color', combustible: 'Combustible', caja: 'Caja', cilindraje: 'Cilindraje (cc)',
  aspiracion: 'Aspiración', traccion: 'Tracción', ciclo: 'Ciclo', refrigeracion: 'Refrigeración',
};

function Origen({ o }: { o: string | undefined }) {
  if (!o) return null;
  return <span className="rounded-full bg-kpi-lilac px-2 py-0.5 text-[11px] font-bold text-lilac-ink">{ORIGEN[o] ?? o}</span>;
}

type Props = Readonly<{
  chat: ChatPiso | null;
  /** Sin chat: el primer mensaje empieza la conversación de esta orden. */
  ordenId?: string;
  disponible: boolean;
  voz: boolean;
  onChat?: (c: ChatPiso) => void;
  /** En el chat general, enlace a la orden cuando la conversación ya es de una. */
  conEnlace?: boolean;
}>;

/**
 * El chat del asistente del piso (V-11, HU-107, ADR-0041; Figma 06, 07 y D2) con la ficha en vivo arriba. El asistente
 * propone; nada queda en la orden hasta tocar Confirmar (R-08).
 */
export function FloorChat({ chat, ordenId, disponible, voz, onChat, conEnlace = false }: Props) {
  const send = useSendFloor();
  const [texto, setTexto] = useState('');
  const [fotos, setFotos] = useState<File[]>([]);
  const fin = useRef<HTMLDivElement>(null);
  const mensajes = chat?.mensajes ?? [];
  useEffect(() => {
    fin.current?.scrollIntoView?.({ block: 'end' });
  }, [mensajes.length]);

  async function enviar(m: { texto: string; audio?: Blob; fotos?: File[] }) {
    try {
      const c = await send.mutateAsync({ ...m, chatId: chat?.id, ordenId: chat ? undefined : ordenId });
      onChat?.(c);
    } catch {
      // el error ya se muestra con send.isError; el texto queda escrito para reintentar
      return false;
    }
    return true;
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!texto.trim() && fotos.length === 0) return;
    if (!(await enviar({ texto, fotos }))) return;
    setTexto('');
    setFotos([]);
  }

  return (
    <div className="flex flex-col gap-3">
      {!disponible ? (
        <Alert kind="warn" title="El asistente no está disponible">
          Sigue con el formulario: recepción, diagnóstico y cotización funcionan igual.{' '}
          {ordenId ? null : (
            <>
              <Link to="/panel/ordenes/nueva" className="font-bold text-primary underline">
                Nueva orden
              </Link>{' '}
              ·{' '}
              <Link to="/panel/ordenes" className="font-bold text-primary underline">
                Órdenes
              </Link>
            </>
          )}
        </Alert>
      ) : null}
      {chat && conEnlace && chat.ordenId ? (
        <Link to={`/panel/ordenes/${chat.ordenId}`} className="self-start rounded-xl bg-primary-tint px-3 py-2 text-sm font-bold text-primary">
          Seguimos en la orden {chat.propuesta.ordenCodigo ?? ''} · abrirla
        </Link>
      ) : null}
      {chat ? <LiveCard chat={chat} onChat={onChat} /> : null}

      <ol aria-label="Conversación con el asistente" className="flex flex-col gap-2">
        {mensajes.length === 0 ? (
          <li className="rounded-2xl bg-surface p-3.5 text-[14px] text-ink-2">
            Cuéntame qué llegó: “llegó el Mazda ABC123 de Carlos, le suenan los frenos”. También puedes mandar fotos o una nota de voz.
          </li>
        ) : null}
        {mensajes.map((m) => (
          <Burbuja key={m.id} m={m} onOpcion={(o) => void enviar({ texto: o })} ocupado={send.isPending} />
        ))}
        {send.isPending ? (
          <li role="status" className="self-start rounded-2xl bg-white px-3.5 py-2 text-sm text-ink-2">
            El asistente está pensando…
          </li>
        ) : null}
      </ol>
      <div ref={fin} />
      {send.isError ? <Alert>{send.error.message}</Alert> : null}

      <form onSubmit={(e) => void submit(e)} className="flex flex-col gap-2 rounded-[22px] bg-white p-3">
        {fotos.length > 0 ? (
          <ul aria-label="Fotos por enviar" className="flex flex-wrap gap-2">
            {fotos.map((f, i) => (
              <li key={`${f.name}-${i}`} className="flex items-center gap-1 rounded-xl bg-surface px-2 py-1 text-xs">
                {f.name}
                <button type="button" aria-label={`Quitar ${f.name}`} className="grid h-8 w-8 place-items-center" onClick={() => setFotos(fotos.filter((_, j) => j !== i))}>
                  <IconX size={14} />
                </button>
              </li>
            ))}
          </ul>
        ) : null}
        <label className="sr-only" htmlFor={`msg-${chat?.id ?? ordenId ?? 'nuevo'}`}>
          Mensaje para el asistente
        </label>
        <textarea
          id={`msg-${chat?.id ?? ordenId ?? 'nuevo'}`}
          value={texto}
          onChange={(e) => setTexto(e.target.value)}
          rows={2}
          maxLength={2000}
          placeholder="Escríbele al asistente…"
          className="w-full resize-none rounded-xl border-[1.5px] border-line px-3 py-2 text-base outline-none focus:border-primary"
        />
        <div className="flex items-center gap-2">
          <label className="inline-flex min-h-[44px] cursor-pointer items-center gap-1.5 rounded-btn bg-surface px-3 text-sm font-bold text-primary">
            <IconCamera size={18} />
            Fotos
            <input
              type="file"
              accept="image/jpeg,image/png,image/webp"
              capture="environment"
              multiple
              className="sr-only"
              onChange={(e) => {
                setFotos([...fotos, ...Array.from(e.target.files ?? [])].slice(0, 4));
                e.target.value = '';
              }}
            />
          </label>
          {voz ? <Microfono onAudio={(a) => void enviar({ texto: '', audio: a })} ocupado={send.isPending} /> : null}
          <span className="flex-1" />
          <button
            type="submit"
            disabled={send.isPending || (!texto.trim() && fotos.length === 0)}
            className="min-h-[44px] rounded-btn bg-primary px-5 font-extrabold text-white disabled:bg-line disabled:text-ink-2"
          >
            Enviar
          </button>
        </div>
      </form>
    </div>
  );
}

function Burbuja({ m, onOpcion, ocupado }: { m: MensajePiso; onOpcion: (o: string) => void; ocupado: boolean }) {
  const mio = m.rol === 'usuario';
  return (
    <li className={`flex max-w-[88%] flex-col gap-1.5 ${mio ? 'self-end' : 'self-start'}`}>
      <div className={`rounded-2xl px-3.5 py-2.5 text-[15px] whitespace-pre-wrap ${mio ? 'bg-primary text-white' : 'bg-white'}`}>
        {m.adjuntos.some((a) => a.tipo === 'voz') && !m.adjuntos.some((a) => a.tipo === 'video') ? (
          <span className="mb-1 block text-xs font-bold opacity-80">Nota de voz</span>
        ) : null}
        <VideoTag m={m} />
        {m.texto}
        {m.conPropuesta ? (
          <span className="mt-2 block w-fit rounded-full bg-kpi-lilac px-2 py-0.5 text-[11px] font-extrabold text-lilac-ink">Propuesta · revísala arriba</span>
        ) : null}
      </div>
      {m.adjuntos.some((a) => a.tipo === 'foto') ? (
        <div className="flex flex-wrap gap-1.5">
          {m.adjuntos
            .filter((a) => a.tipo === 'foto' && a.url)
            .map((a) => (
              <img key={a.url} src={a.url} alt="Foto enviada al asistente" className="h-20 w-20 rounded-xl object-cover" />
            ))}
        </div>
      ) : null}
      {m.opciones.length > 0 ? (
        <div className="flex flex-wrap gap-2">
          {m.opciones.map((o) => (
            <button key={o} type="button" disabled={ocupado} onClick={() => onOpcion(o)} className="min-h-[44px] rounded-btn border-[1.5px] border-primary bg-white px-4 font-bold text-primary">
              Seguir con {o}
            </button>
          ))}
        </div>
      ) : null}
      <span className={`text-[11px] text-ink-3 ${mio ? 'self-end' : ''}`}>{formatoHora(m.creadoEn)}</span>
    </li>
  );
}

/** "Video de recepción · 1:02 · 8 cuadros": el video no se guarda, solo su audio en texto y los cuadros. */
function VideoTag({ m }: { m: MensajePiso }) {
  const v = m.adjuntos.find((a) => a.tipo === 'video');
  if (!v) return null;
  const s = v.segundos ?? 0;
  const n = m.adjuntos.filter((a) => a.tipo === 'foto').length;
  const dur = s > 0 ? ` · ${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}` : '';
  return <span className="mb-1 block text-xs font-bold opacity-80">{`${s > 0 ? 'Video de recepción' : 'Fotos de recepción'}${dur} · ${n} ${n === 1 ? 'cuadro' : 'cuadros'}`}</span>;
}

/** Nota de voz con MediaRecorder: un toque empieza, otro la manda. */
function Microfono({ onAudio, ocupado }: { onAudio: (a: Blob) => void; ocupado: boolean }) {
  const [grabando, setGrabando] = useState(false);
  const [error, setError] = useState('');
  const rec = useRef<MediaRecorder | null>(null);
  async function empezar() {
    setError('');
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      const r = new MediaRecorder(stream);
      const partes: Blob[] = [];
      r.ondataavailable = (e) => partes.push(e.data);
      r.onstop = () => {
        stream.getTracks().forEach((t) => t.stop());
        onAudio(new Blob(partes, { type: r.mimeType || 'audio/webm' }));
      };
      r.start();
      rec.current = r;
      setGrabando(true);
    } catch {
      setError('No pudimos usar el micrófono. Revisa el permiso del navegador.');
    }
  }
  return (
    <>
      <button
        type="button"
        disabled={ocupado}
        aria-pressed={grabando}
        onClick={() => {
          if (grabando) {
            rec.current?.stop();
            setGrabando(false);
          } else void empezar();
        }}
        className={`min-h-[44px] rounded-btn px-3 text-sm font-bold ${grabando ? 'bg-err text-white' : 'bg-surface text-primary'}`}
      >
        {grabando ? 'Parar y enviar' : 'Hablar'}
      </button>
      {error ? <span className="text-xs text-err-ink">{error}</span> : null}
    </>
  );
}

/** Cuántos ítems de la cotización siguen sin precio. */
function sinPrecio(p: PropuestaPiso) {
  return (p.cotizacion ?? []).filter((l) => l.precio == null || l.precio <= 0).length;
}

/**
 * Ficha en vivo (Figma 07): "7 de 8 datos · falta…", lo propuesto con su origen, quitar o poner un precio, y Confirmar,
 * que guarda todo en la orden.
 */
export function LiveCard({ chat, onChat }: { chat: ChatPiso; onChat?: ((c: ChatPiso) => void) | undefined }) {
  const [abierta, setAbierta] = useState(true);
  const edit = useEditFloor();
  const confirm = useConfirmFloor();
  const [clave, setClave] = useState(() => crypto.randomUUID());
  const [listo, setListo] = useState<string[] | null>(null);
  const p = chat.propuesta;
  const pr = chat.progreso;
  const guardar = async (np: PropuestaPiso) => onChat?.(await edit.mutateAsync({ chat, propuesta: np }));
  if (!chat.pendiente && !listo) {
    return pr.total > 0 ? <Resumen pr={pr} /> : null;
  }
  const faltan = sinPrecio(p);
  let falta: string | undefined;
  if (faltan === 1) falta = 'Falta el precio de 1 ítem de la cotización.';
  else if (faltan > 1) falta = `Falta el precio de ${faltan} ítems de la cotización.`;

  return (
    <section aria-labelledby={`ficha-${chat.id}`} className="flex flex-col gap-3 rounded-[22px] border-[1.5px] border-kpi-lilac bg-white p-4">
      <div className="flex items-start justify-between gap-2">
        <div className="flex min-w-0 flex-col">
          <h2 id={`ficha-${chat.id}`} className="text-[16px] font-extrabold">
            Ficha en vivo
          </h2>
          <span className="text-[13px] text-ink-2">
            {pr.total > 0 ? `${pr.tiene} de ${pr.total} datos` : 'Por confirmar'}
            {pr.falta.length > 0 ? ` · falta ${pr.falta.join(', ')}` : ''}
          </span>
        </div>
        <button type="button" aria-expanded={abierta} onClick={() => setAbierta(!abierta)} className="min-h-[44px] shrink-0 px-2 text-sm font-bold text-primary">
          {abierta ? 'Plegar' : 'Ver todo'}
        </button>
      </div>
      {listo ? (
        <p role="status" className="rounded-xl bg-ok-tint p-3 text-sm font-bold text-ok-ink">
          Quedó guardado: {listo.join(', ')}.
        </p>
      ) : null}
      {abierta && chat.pendiente ? <Detalle p={p} onChange={(np) => void guardar(np)} ocupado={edit.isPending} /> : null}
      {edit.isError ? <Alert>{edit.error.message}</Alert> : null}
      {confirm.isError ? <Alert>{confirm.error.message}</Alert> : null}
      {chat.pendiente ? (
        <Button
          busy={confirm.isPending}
          missing={falta}
          onClick={() =>
            void confirm
              .mutateAsync({ chat, key: clave })
              .then((r) => {
                setListo(r.resultado.hecho);
                setClave(crypto.randomUUID());
                onChat?.(r.chat);
              })
              .catch(() => undefined) // el error ya se muestra con confirm.isError
          }
        >
          Confirmar
        </Button>
      ) : null}
    </section>
  );
}

function Resumen({ pr }: { pr: ChatPiso['progreso'] }) {
  return (
    <p className="rounded-2xl bg-white px-4 py-3 text-[13px] text-ink-2">
      Recepción: {pr.tiene} de {pr.total} datos{pr.falta.length > 0 ? ` · falta ${pr.falta.join(', ')}` : ''}
    </p>
  );
}

function Fila({ label, valor, origen, onQuitar }: { label: string; valor: string; origen?: string | undefined; onQuitar?: () => void }) {
  return (
    <li className="flex items-center gap-2 py-1.5 text-sm">
      <span className="w-28 shrink-0 text-ink-2">{label}</span>
      <b className="min-w-0 flex-1 break-words">{valor}</b>
      <Origen o={origen} />
      {onQuitar ? (
        <button type="button" aria-label={`Quitar ${label}`} onClick={onQuitar} className="grid h-11 w-11 shrink-0 place-items-center rounded-xl text-ink-2 hover:bg-surface">
          <IconX size={16} />
        </button>
      ) : null}
    </li>
  );
}

function Detalle({ p, onChange, ocupado }: { p: PropuestaPiso; onChange: (np: PropuestaPiso) => void; ocupado: boolean }) {
  const set = (patch: Partial<PropuestaPiso>) => onChange({ ...p, ...patch });
  const quitar = (k: keyof PropuestaPiso) => onChange(Object.fromEntries(Object.entries(p).filter(([key]) => key !== k)) as PropuestaPiso);
  return (
    <div className="flex flex-col gap-3" aria-busy={ocupado || undefined}>
      <ul className="divide-y divide-surface-2">
        {p.placa ? (
          <li className="flex items-center gap-2 py-1.5 text-sm">
            <span className="w-28 shrink-0 text-ink-2">Placa</span>
            <Plate value={p.placa.valor} />
            <span className="flex-1 text-ink-2">{p.ordenCodigo ? `orden ${p.ordenCodigo}` : p.nuevaOrden ? 'orden nueva' : ''}</span>
            <Origen o={p.placa.origen} />
          </li>
        ) : null}
        {p.cliente?.nombre ? <Fila label="Cliente" valor={p.cliente.nombre.valor} origen={p.cliente.nombre.origen} /> : null}
        {p.cliente?.celular ? <Fila label="Celular" valor={p.cliente.celular.valor} origen={p.cliente.celular.origen} /> : null}
        {p.sintoma ? <Fila label="Qué trae" valor={p.sintoma.valor} origen={p.sintoma.origen} onQuitar={() => quitar('sintoma')} /> : null}
        {Object.entries(p.ficha ?? {}).map(([k, v]) => (
          <Fila
            key={k}
            label={FICHA[k] ?? k}
            valor={v.valor}
            origen={v.origen}
            onQuitar={() => set({ ficha: Object.fromEntries(Object.entries(p.ficha ?? {}).filter(([key]) => key !== k)) })}
          />
        ))}
        {p.kilometraje ? <Fila label="Kilometraje" valor={`${p.kilometraje.valor} km`} origen={p.kilometraje.origen} onQuitar={() => quitar('kilometraje')} /> : null}
        {p.combustible ? <Fila label="Combustible" valor={p.combustible.valor} origen={p.combustible.origen} onQuitar={() => quitar('combustible')} /> : null}
        {(p.items ?? []).map((it) => (
          <Fila
            key={it.clave}
            label={it.nombre}
            valor={it.estado === 'no_tiene' ? 'No tiene' : it.estado === 'mal' ? `Mal${it.nota ? ` · ${it.nota}` : ''}` : 'Bien'}
            origen={it.origen}
            onQuitar={() => set({ items: (p.items ?? []).filter((x) => x.clave !== it.clave) })}
          />
        ))}
        {(p.notas ?? []).map((n, i) => (
          <Fila key={`n-${i}`} label="Nota" valor={n.valor} origen={n.origen} onQuitar={() => set({ notas: (p.notas ?? []).filter((_, j) => j !== i) })} />
        ))}
        {(p.fotos ?? []).length > 0 ? (
          <Fila
            label="Fotos"
            valor={(p.fotos ?? []).map((f) => (f.lado ? LADO[f.lado] : 'sin lado')).join(' · ')}
            onQuitar={() => quitar('fotos')}
          />
        ) : null}
        {(p.hallazgos ?? []).map((h) => (
          <Fila
            key={h.id}
            label={`Hallazgo · ${h.sistema}`}
            valor={h.descripcion}
            origen={h.origen}
            onQuitar={() => set({ hallazgos: (p.hallazgos ?? []).filter((x) => x.id !== h.id) })}
          />
        ))}
        {(p.gastos ?? []).map((g) => (
          <Fila
            key={g.id}
            label="Gasto"
            valor={`${g.descripcion} · ${formatoCOP(g.valor)}`}
            origen={g.origen}
            onQuitar={() => set({ gastos: (p.gastos ?? []).filter((x) => x.id !== g.id) })}
          />
        ))}
        {p.estado ? <Fila label="Pasar a" valor={ESTADO[p.estado.valor] ?? p.estado.valor} origen={p.estado.origen} onQuitar={() => quitar('estado')} /> : null}
      </ul>
      {(p.cotizacion ?? []).length > 0 ? (
        <div className="flex flex-col gap-1">
          <h3 className="text-[14px] font-extrabold">Cotización</h3>
          <ul className="flex flex-col gap-1.5">
            {(p.cotizacion ?? []).map((l) => (
              <LineaPrecio
                key={l.id}
                linea={l}
                onPrecio={(precio) => set({ cotizacion: (p.cotizacion ?? []).map((x) => (x.id === l.id ? { ...x, precio } : x)) })}
                onQuitar={() => set({ cotizacion: (p.cotizacion ?? []).filter((x) => x.id !== l.id) })}
              />
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  );
}

type Linea = NonNullable<PropuestaPiso['cotizacion']>[number];

/** Un ítem de la cotización. Sin precio: "Sin precio · ponlo" con el campo para escribirlo (nunca se sugiere uno). */
function LineaPrecio({ linea: l, onPrecio, onQuitar }: { linea: Linea; onPrecio: (p: number) => void; onQuitar: () => void }) {
  const [valor, setValor] = useState('');
  const v = pesos(valor);
  return (
    <li className="flex flex-wrap items-center gap-2 rounded-xl bg-surface px-3 py-2 text-sm">
      <span className="min-w-0 flex-1">
        <b>{l.nombre}</b>
        {l.cantidad > 1 ? ` × ${l.cantidad}` : ''}
      </span>
      {l.precio != null ? (
        <>
          <b className="font-mono">{formatoCOP(l.precio * l.cantidad)}</b>
          <Origen o={l.origenPrecio} />
        </>
      ) : (
        <form
          className="flex items-center gap-1.5"
          onSubmit={(e) => {
            e.preventDefault();
            if (v && v > 0) onPrecio(v);
          }}
        >
          <label className="text-xs font-bold text-warn-ink" htmlFor={`precio-${l.id}`}>
            Sin precio · ponlo
          </label>
          <input
            id={`precio-${l.id}`}
            inputMode="numeric"
            value={valor}
            onChange={(e) => setValor(e.target.value)}
            className="w-28 rounded-lg border-[1.5px] border-line bg-white px-2 py-1.5 font-mono"
          />
          <button type="submit" disabled={!v || v <= 0} className="min-h-[44px] rounded-lg bg-primary px-3 text-xs font-extrabold text-white disabled:bg-line disabled:text-ink-2">
            Poner
          </button>
        </form>
      )}
      <button type="button" aria-label={`Quitar ${l.nombre}`} onClick={onQuitar} className="grid h-11 w-11 place-items-center rounded-xl text-ink-2 hover:bg-white">
        <IconX size={16} />
      </button>
    </li>
  );
}
