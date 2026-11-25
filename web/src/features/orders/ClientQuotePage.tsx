import { useEffect, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router';

import { ApiError, type Schemas } from '../../shared/api/client';
import { formatoCOP, formatoFechaHora } from '../../shared/lib/format';
import { sharePdf } from '../../shared/lib/sharePdf';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';
import { IconCheck, IconClock, IconStop, IconWarn } from '../../shared/ui/icons';
import { usePermission } from '../auth/api';
import { fetchQuotePdf, useClientView, useCounterConsent, useQuoteActions } from './work';

type Documento = Schemas['DocumentoCliente'];
type Paso = 'ver' | 'autorizar' | 'motivo';

const HOLD_MS = 700;

function errorText(e: unknown): string {
  if (e instanceof ApiError && e.status === 409) return 'Alguien cambió esta cotización. Ya la actualizamos: revisa e intenta de nuevo.';
  return e instanceof Error ? e.message : 'No se pudo guardar.';
}

/**
 * V-10 (ADR-0043, HU-109): la cotización en pantalla completa para mostrársela al cliente, con letra grande y sin el
 * menú del panel. Para volver al panel hay que mantener presionado "Volver", así el cliente no navega el panel
 * por error. "Compartir PDF" abre el menú de compartir del sistema; no es una integración con WhatsApp.
 */
export function ClientQuotePage() {
  const { id = '' } = useParams();
  const nav = useNavigate();
  const vista = useClientView(id);
  const puedeRegistrar = usePermission('quote:register_response');
  const act = useQuoteActions(id);
  const consent = useCounterConsent(id);
  const [paso, setPaso] = useState<Paso>('ver');
  const [consentido, setConsentido] = useState(false); // la autorización ya se guardó: no se repite si falla la aprobación
  const [motivo, setMotivo] = useState('');
  const [pdf, setPdf] = useState<{ busy: boolean; aviso: string; error: string }>({ busy: false, aviso: '', error: '' });

  const volver = () => nav(`/panel/ordenes/${id}`);

  if (vista.isPending) {
    return (
      <Pantalla>
        <p role="status" className="p-8 text-xl text-ink-2">Cargando la cotización…</p>
      </Pantalla>
    );
  }
  if (vista.isError) {
    return (
      <Pantalla>
        <div className="flex flex-col gap-4 p-6">
          <Alert>{vista.error.message}</Alert>
          <HoldToLeave onLeave={volver} />
        </div>
      </Pantalla>
    );
  }

  const { documento: doc, autorizacion } = vista.data;
  const q = doc.cotizacion;
  const espera = (q.estado === 'enviada' || q.estado === 'dudas') && !q.vencida;
  const error = act.respond.error ?? consent.error;

  const aprobar = async () => {
    await act.respond.mutateAsync({ decision: 'aprobar', canal: 'mostrador' });
    setPaso('ver');
  };
  // HU-86: antes de aprobar en el mostrador se le pide al cliente la autorización de sus datos, en la misma pantalla.
  const decidirAutorizacion = async (acepta: boolean) => {
    await consent.mutateAsync({ clienteId: autorizacion.clienteId, acepta });
    setConsentido(true);
    setPaso('ver');
    await aprobar();
  };
  const noAprobo = async () => {
    await act.respond.mutateAsync({ decision: 'rechazar', canal: 'mostrador', ...(motivo.trim() ? { motivo: motivo.trim() } : {}) });
    setPaso('ver');
  };
  const compartir = async () => {
    setPdf({ busy: true, aviso: '', error: '' });
    try {
      const { blob, nombre } = await fetchQuotePdf(id);
      const r = await sharePdf(blob, nombre, `Cotización ${doc.orden}`);
      setPdf({ busy: false, aviso: r === 'descargado' ? 'Se descargó el PDF.' : '', error: '' });
    } catch (e) {
      setPdf({ busy: false, aviso: '', error: errorText(e) });
    }
  };

  return (
    <Pantalla>
      <article className="mx-auto flex w-full max-w-[640px] flex-col gap-6 px-5 pb-6 pt-8">
        <Encabezado doc={doc} />
        <Estado estado={q.estado} vencida={q.vencida} venceEn={q.venceEn} />
        <Lineas doc={doc} />
        <Totales doc={doc} />
        {q.venceEn && !q.vencida && espera ? (
          <p className="text-lg text-ink-2">Válida hasta el {formatoFechaHora(q.venceEn)}</p>
        ) : null}
      </article>

      <footer className="mt-auto border-t border-line bg-white px-5 pb-[max(env(safe-area-inset-bottom),16px)] pt-4">
        <div className="mx-auto flex w-full max-w-[640px] flex-col gap-3">
          {error ? <Alert>{errorText(error)}</Alert> : null}
          {pdf.error ? <Alert>{pdf.error}</Alert> : null}
          {pdf.aviso ? <p role="status" className="text-center text-base font-bold text-ok-ink">{pdf.aviso}</p> : null}

          {paso === 'ver' && puedeRegistrar && espera ? (
            <>
              <Button busy={act.respond.isPending} onClick={() => (autorizacion.estado === 'pendiente' && !consentido ? setPaso('autorizar') : void aprobar().catch(() => undefined))}>
                El cliente aprobó
              </Button>
              <Button variant="secondary" onClick={() => setPaso('motivo')}>
                No aprobó
              </Button>
            </>
          ) : null}

          {paso === 'autorizar' ? (
            <section aria-labelledby="autoriza" className="flex flex-col gap-3 rounded-card bg-surface p-4">
              <h2 id="autoriza" className="text-lg font-extrabold">Antes de aprobar: autorización de datos</h2>
              <p className="text-base">{autorizacion.texto}</p>
              <Button busy={consent.isPending || act.respond.isPending} onClick={() => void decidirAutorizacion(true).catch(() => undefined)}>
                El cliente autoriza y aprueba
              </Button>
              <Button variant="secondary" busy={consent.isPending || act.respond.isPending} onClick={() => void decidirAutorizacion(false).catch(() => undefined)}>
                No autoriza, aprueba igual
              </Button>
              <Button variant="ghost" onClick={() => setPaso('ver')}>Atrás</Button>
            </section>
          ) : null}

          {paso === 'motivo' ? (
            <section aria-labelledby="motivo" className="flex flex-col gap-3 rounded-card bg-surface p-4">
              <h2 id="motivo" className="text-lg font-extrabold">¿Por qué no aprobó?</h2>
              <Field label="Motivo (opcional)" maxLength={200} value={motivo} onChange={(e) => setMotivo(e.target.value)} />
              <Button busy={act.respond.isPending} onClick={() => void noAprobo().catch(() => undefined)}>
                Confirmar: no aprobó
              </Button>
              <Button variant="ghost" onClick={() => setPaso('ver')}>Atrás</Button>
            </section>
          ) : null}

          <Button variant="secondary" busy={pdf.busy} onClick={() => void compartir()}>
            Compartir PDF
          </Button>
          <HoldToLeave onLeave={volver} />
        </div>
      </footer>
    </Pantalla>
  );
}

function Pantalla({ children }: { children: React.ReactNode }) {
  return <main className="flex min-h-dvh flex-col bg-white text-[20px] text-ink">{children}</main>;
}

function Encabezado({ doc }: { doc: Documento }) {
  return (
    <header className="flex flex-col gap-1">
      <h1 className="text-[30px] font-extrabold leading-tight">{doc.taller}</h1>
      <p className="text-lg text-ink-2">
        Cotización {doc.cotizacion.numero} · Orden {doc.orden}
      </p>
      <p className="text-xl font-bold">
        {doc.vehiculo} · <span className="font-mono">{doc.placa}</span>
      </p>
      {doc.cliente ? <p className="text-lg text-ink-2">Para {doc.cliente}</p> : null}
    </header>
  );
}

/** Cada estado lleva ícono y palabra (DR-01). */
function Estado({ estado, vencida, venceEn }: { estado: string; vencida: boolean; venceEn: string | null }) {
  if (vencida) {
    return (
      <Alert kind="warn" title="Esta cotización venció">
        {venceEn ? `Venció el ${formatoFechaHora(venceEn)}. ` : ''}Renueva la vigencia desde la orden para registrar la respuesta.
      </Alert>
    );
  }
  const filas: Record<string, { icono: React.ReactNode; texto: string; clase: string } | undefined> = {
    borrador: { icono: <IconWarn size={22} />, texto: 'Borrador: todavía no se ha enviado. Envíala desde la orden para registrar la respuesta.', clase: 'bg-warn-tint text-warn-ink' },
    aprobada: { icono: <IconCheck size={22} />, texto: 'Aprobada. El trabajo puede empezar.', clase: 'bg-ok-tint text-ok-ink' },
    rechazada: { icono: <IconStop size={22} />, texto: 'No aprobada.', clase: 'bg-err-tint text-err-ink' },
    dudas: { icono: <IconClock size={22} />, texto: 'El cliente tiene dudas.', clase: 'bg-surface text-ink' },
  };
  const f = filas[estado];
  if (!f) return null;
  return (
    <p role="status" className={`flex items-start gap-3 rounded-2xl p-4 text-lg font-bold ${f.clase}`}>
      <span className="mt-0.5 flex-none">{f.icono}</span>
      {f.texto}
    </p>
  );
}

function Lineas({ doc }: { doc: Documento }) {
  return (
    <ul className="flex flex-col divide-y divide-line" aria-label="Ítems de la cotización">
      {doc.cotizacion.lineas.map((l, i) => (
        <li key={`${l.descripcion}-${i}`} className="flex flex-col gap-1 py-4">
          <div className="flex items-start justify-between gap-4">
            <span className={l.tipo === 'partida' ? 'text-[22px] font-extrabold' : 'text-[20px] font-bold'}>
              {l.descripcion}
              {l.aprobada === false ? <span className="ml-2 text-base font-bold text-err-ink">no aprobado</span> : null}
            </span>
            <span className="flex-none font-mono text-[20px] font-bold">{formatoCOP(l.subtotal)}</span>
          </div>
          {l.cantidad > 1 && l.tipo !== 'partida' ? (
            <span className="text-base text-ink-2">
              {l.cantidad} × {formatoCOP(l.precioUnitario)}
            </span>
          ) : null}
          {l.detalle?.length ? (
            <ul className="ml-4 list-disc text-lg text-ink-2" aria-label={`Detalle de ${l.descripcion}`}>
              {l.detalle.map((d, j) => (
                <li key={`${d.descripcion}-${j}`}>
                  {d.descripcion}
                  {d.cantidad > 1 ? ` × ${d.cantidad}` : ''}
                </li>
              ))}
            </ul>
          ) : null}
          {l.descuentoPct > 0 ? (
            <span className="text-base text-ink-2">
              Descuento {l.descuentoPct} %: −{formatoCOP(l.descuento)}
            </span>
          ) : null}
        </li>
      ))}
    </ul>
  );
}

function Totales({ doc }: { doc: Documento }) {
  const q = doc.cotizacion;
  const detalle = q.descuento > 0 || q.ivaPct > 0;
  return (
    <dl className="flex flex-col gap-2 border-t-2 border-ink pt-4">
      {detalle ? (
        <>
          <Fila etiqueta="Subtotal" valor={formatoCOP(q.subtotal)} />
          {q.descuento > 0 ? <Fila etiqueta="Descuentos" valor={`−${formatoCOP(q.descuento)}`} /> : null}
          {q.ivaPct > 0 ? <Fila etiqueta={`IVA ${q.ivaPct} %${q.preciosConIva ? ' (incluido)' : ''}`} valor={formatoCOP(q.iva)} /> : null}
        </>
      ) : null}
      <div className="flex items-baseline justify-between gap-4 text-[28px] font-extrabold">
        <dt>Total{q.preciosConIva && q.ivaPct > 0 ? ' con IVA incluido' : ''}</dt>
        <dd className="font-mono">{formatoCOP(q.total)}</dd>
      </div>
    </dl>
  );
}

function Fila({ etiqueta, valor }: { etiqueta: string; valor: string }) {
  return (
    <div className="flex items-baseline justify-between gap-4 text-lg text-ink-2">
      <dt>{etiqueta}</dt>
      <dd className="font-mono">{valor}</dd>
    </div>
  );
}

/**
 * Mantener presionado ~0,7 s para salir al panel. Un toque corto solo avisa cómo se sale. También funciona con el
 * teclado (Enter o espacio sostenidos).
 */
export function HoldToLeave({ onLeave }: { onLeave: () => void }) {
  const timer = useRef<number | undefined>(undefined);
  const [pista, setPista] = useState(false);
  const stop = (): boolean => {
    const corriendo = timer.current !== undefined;
    window.clearTimeout(timer.current);
    timer.current = undefined;
    return corriendo;
  };
  const start = () => {
    stop();
    setPista(false);
    timer.current = window.setTimeout(() => {
      timer.current = undefined;
      onLeave();
    }, HOLD_MS);
  };
  const release = () => {
    if (stop()) setPista(true); // lo soltó antes de tiempo
  };
  useEffect(() => () => void stop(), []);

  return (
    <div className="flex flex-col items-center gap-1">
      <button
        type="button"
        className="min-h-[52px] w-full select-none rounded-btn px-5 text-base font-bold text-ink-2 hover:bg-surface"
        onPointerDown={start}
        onPointerUp={release}
        onPointerLeave={() => void stop()}
        onPointerCancel={() => void stop()}
        onContextMenu={(e) => e.preventDefault()}
        onKeyDown={(e) => {
          if ((e.key === 'Enter' || e.key === ' ') && !e.repeat) start();
        }}
        onKeyUp={(e) => {
          if (e.key === 'Enter' || e.key === ' ') release();
        }}
      >
        Volver
      </button>
      <span role="status" className="min-h-5 text-sm text-ink-2">
        {pista ? 'Mantén presionado «Volver» para salir.' : ''}
      </span>
    </div>
  );
}
