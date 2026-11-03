import { useEffect, useState, type FormEvent } from 'react';

import { ApiError, type Orden } from '../../shared/api/client';
import { estados, flujo } from '../../shared/i18n/es-CO';
import { formatoCOP, formatoHora } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';
import {
  photoUrl,
  useAddEvidence,
  useCatalog,
  useChecklist,
  useEvidence,
  useQuote,
  useQuoteActions,
  useSaveChecklist,
  useTransition,
} from './work';
import { usePermission } from '../auth/api';

export const cotizarEvento = 'sweveh:cotizar';
const ladosCarro = ['frente', 'trasera', 'izquierdo', 'derecho'];
const ladosMoto = [...ladosCarro, 'tablero'];
const combustibles: Record<string, string> = {
  vacio: 'Vacío',
  '1/4': '1/4',
  '1/2': '1/2',
  '3/4': '3/4',
  lleno: 'Lleno',
  no_aplica: 'No aplica (moto)',
};

function errorText(e: unknown): string | undefined {
  if (!e) return undefined;
  if (e instanceof ApiError && e.status === 409) return 'Alguien cambió esta orden. Ya la actualizamos: revisa e intenta de nuevo.';
  return e instanceof Error ? e.message : 'No se pudo guardar.';
}

const card = 'flex flex-col gap-3 rounded-card bg-white p-5';
const select = 'min-h-[52px] rounded-btn border-[1.5px] border-line bg-white px-4 text-base';

/** HU-20 y HU-25: avanzar o devolverse un paso, con las puertas que pide cada estado. */
export function OrderActions({ orden }: { orden: Orden }) {
  const mover = useTransition(orden.id);
  const puedeAnular = usePermission('order:override_approval');
  const [motivo, setMotivo] = useState('');
  const [recogido, setRecogido] = useState('');
  const [accesorios, setAccesorios] = useState(false);
  const i = flujo.indexOf(orden.estado);
  if (i < 0 || orden.estado === 'entregado') return null;
  const siguiente = flujo[i + 1];
  const anterior = i > 0 ? flujo[i - 1] : undefined;
  const sinAprobacion = siguiente === 'en_reparacion';
  const entrega = siguiente === 'entregado';

  return (
    <section className={card} aria-labelledby="acciones">
      <h2 id="acciones" className="text-[17px] font-extrabold">
        Mover la orden
      </h2>
      {sinAprobacion && puedeAnular ? (
        <Field
          label="Motivo si el cliente aprobó por fuera (solo el dueño)"
          placeholder="Ej.: aprobó por teléfono"
          value={motivo}
          onChange={(e) => setMotivo(e.target.value)}
        />
      ) : null}
      {entrega ? (
        <>
          <Field label="¿Quién recoge?" value={recogido} onChange={(e) => setRecogido(e.target.value)} />
          <label className="flex min-h-[44px] items-center gap-2">
            <input type="checkbox" checked={accesorios} onChange={(e) => setAccesorios(e.target.checked)} />
            Revisé con el cliente los accesorios del checklist
          </label>
        </>
      ) : null}
      {mover.isError ? <Alert>{errorText(mover.error)}</Alert> : null}
      <div className="flex flex-wrap gap-3">
        {siguiente ? (
          <Button
            busy={mover.isPending}
            onClick={() =>
              mover.mutate({
                estado: siguiente,
                version: orden.version,
                ...(motivo ? { motivo } : {}),
                ...(entrega ? { recogidoPor: recogido, accesoriosConfirmados: accesorios } : {}),
              })
            }
          >
            Pasar a {estados[siguiente]}
          </Button>
        ) : null}
        {anterior ? (
          <Button variant="ghost" disabled={mover.isPending} onClick={() => mover.mutate({ estado: anterior, version: orden.version })}>
            Devolver a {estados[anterior]}
          </Button>
        ) : null}
      </div>
    </section>
  );
}

/** HU-17 y HU-18: checklist de recepción con fotos por lado; dice qué falta (DR-06). */
export function ReceptionPanel({ orden }: { orden: Orden }) {
  const cl = useChecklist(orden.id);
  const guardar = useSaveChecklist(orden.id);
  const ev = useEvidence(orden.id);
  const subir = useAddEvidence(orden.id);
  const [km, setKm] = useState('');
  const [fuel, setFuel] = useState('');
  const [acc, setAcc] = useState('');
  const [nota, setNota] = useState('');

  if (cl.isPending) return <p role="status">Cargando checklist…</p>;
  if (cl.isError) return <Alert>{cl.error.message}</Alert>;
  const c = cl.data;
  const lados = c.tipo === 'moto' ? ladosMoto : ladosCarro;
  const abierta = orden.estado !== 'entregado' && orden.estado !== 'cancelada';

  const onSave = (e: FormEvent) => {
    e.preventDefault();
    guardar.mutate({
      kilometraje: km === '' ? (c.kilometraje ?? null) : Number(km),
      combustible: fuel === '' ? (c.combustible ?? null) : fuel,
      accesorios: acc === '' ? c.accesorios : acc.split(',').map((s) => s.trim()).filter(Boolean),
    });
  };

  return (
    <section className={card} aria-labelledby="recepcion">
      <h2 id="recepcion" className="text-[17px] font-extrabold">
        Recepción {c.completadoEn ? '· completa' : ''}
      </h2>
      {c.falta.length > 0 ? (
        <Alert kind="warn" title="Para pasar a Recibido falta">
          {c.falta.join('; ')}
        </Alert>
      ) : null}
      {abierta ? (
        <form onSubmit={onSave} className="grid gap-3 sm:grid-cols-3">
          <Field
            label="Kilometraje"
            inputMode="numeric"
            placeholder={c.kilometraje?.toString() ?? '45000'}
            value={km}
            onChange={(e) => setKm(e.target.value.replace(/\D/g, ''))}
          />
          <div className="flex flex-col gap-1.5">
            <label htmlFor="combustible" className="text-[15px] font-extrabold">
              Combustible
            </label>
            <select id="combustible" className={select} value={fuel || c.combustible || ''} onChange={(e) => setFuel(e.target.value)}>
              <option value="">Elige…</option>
              {Object.entries(combustibles).map(([v, t]) => (
                <option key={v} value={v}>
                  {t}
                </option>
              ))}
            </select>
          </div>
          <Field
            label="Accesorios (separados por coma)"
            placeholder={c.accesorios.join(', ') || 'gato, llanta de repuesto'}
            value={acc}
            onChange={(e) => setAcc(e.target.value)}
          />
          <div className="sm:col-span-3">
            <Button type="submit" variant="secondary" busy={guardar.isPending}>
              Guardar checklist
            </Button>
          </div>
          {guardar.isError ? <Alert>{errorText(guardar.error)}</Alert> : null}
        </form>
      ) : null}

      <h3 className="font-extrabold">Fotos y notas</h3>
      <ul className="grid grid-cols-2 gap-3 sm:grid-cols-5">
        {lados.map((lado) => {
          const tiene = c.ladosConFoto.includes(lado);
          return (
            <li key={lado} className={`flex flex-col gap-2 rounded-2xl p-3 text-sm ${tiene ? 'bg-primary-tint' : 'bg-surface'}`}>
              <span className="font-bold capitalize">{lado}</span>
              {abierta ? (
                <label className="cursor-pointer font-bold text-primary">
                  {tiene ? 'Cambiar foto' : 'Tomar foto'}
                  <input
                    type="file"
                    accept="image/jpeg,image/png,image/webp"
                    capture="environment"
                    className="sr-only"
                    onChange={(e) => {
                      const foto = e.target.files?.[0];
                      if (foto) subir.mutate({ lado, foto, mostrarCliente: true });
                      e.target.value = '';
                    }}
                  />
                </label>
              ) : null}
            </li>
          );
        })}
      </ul>
      {subir.isPending ? <p role="status">Subiendo…</p> : null}
      {subir.isError ? <Alert>{errorText(subir.error)}</Alert> : null}
      {abierta ? (
        <form
          className="flex flex-wrap items-end gap-3"
          onSubmit={(e) => {
            e.preventDefault();
            if (nota.trim()) subir.mutate({ nota: nota.trim() }, { onSuccess: () => setNota('') });
          }}
        >
          <div className="min-w-[240px] flex-1">
            <Field label="Nota (lo que encontraste o cambiaste)" value={nota} onChange={(e) => setNota(e.target.value)} />
          </div>
          <Button type="submit" variant="ghost">
            Agregar nota
          </Button>
        </form>
      ) : null}
      <ul className="flex flex-col gap-2">
        {(ev.data ?? []).map((x) => (
          <li key={x.id} className="flex items-center gap-3 rounded-2xl bg-surface p-3 text-sm">
            {x.tieneFoto ? (
              <img src={photoUrl(orden.id, x.id)} alt={`Foto ${x.lado ?? ''}`} className="h-14 w-14 rounded-xl object-cover" loading="lazy" />
            ) : null}
            <span className="flex-1">{x.nota ?? (x.lado ? `Foto: ${x.lado}` : 'Foto')}</span>
            <span className="font-mono text-xs text-ink-2">
              {estados[x.etapa as keyof typeof estados] ?? x.etapa} · {formatoHora(x.creadoEn)}
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}

/** HU-27, HU-29 y HU-34: cotización con servicios y repuestos del taller; precios solo de la lista. */
export function QuotePanel({ orden }: { orden: Orden }) {
  const q = useQuote(orden.id);
  const act = useQuoteActions(orden.id);
  const puedeRegistrar = usePermission('quote:register_response');
  const [buscar, setBuscar] = useState('');
  const cat = useCatalog(buscar);
  // "Cotizar esto" desde un hallazgo del diagnóstico: busca en el catálogo con su texto (DiagnosisPanel).
  useEffect(() => {
    const onCotizar = (e: Event) => setBuscar((e as CustomEvent<string>).detail);
    window.addEventListener(cotizarEvento, onCotizar);
    return () => window.removeEventListener(cotizarEvento, onCotizar);
  }, []);
  if (q.isPending) return <p role="status">Cargando cotización…</p>;
  if (q.isError) return <Alert>{q.error.message}</Alert>;
  // Sin cotización todavía: se arma una vacía; el primer ítem crea el borrador en el servidor.
  const cot = q.data ?? { numero: 1, estado: 'borrador' as const, total: 0, lineas: [] };
  const editable = cot.estado === 'borrador';
  const err = act.add.error ?? act.remove.error ?? act.send.error ?? act.respond.error;

  return (
    <section className={card} aria-labelledby="cotizacion">
      <h2 id="cotizacion" className="text-[17px] font-extrabold">
        Cotización #{cot.numero} · {cot.estado}
      </h2>
      <table className="w-full text-left text-sm">
        <thead>
          <tr className="text-ink-2">
            <th className="py-2">Ítem</th>
            <th>Cant.</th>
            <th className="text-right">Subtotal</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {cot.lineas.map((l) => (
            <tr key={l.id} className="border-t border-line">
              <td className="py-2">
                {l.descripcion}
                {l.sinStock ? <span className="ml-2 font-bold text-warn-ink">sin existencia</span> : null}
              </td>
              <td>{l.cantidad}</td>
              <td className="text-right font-mono">{formatoCOP(l.subtotal)}</td>
              <td className="text-right">
                {editable ? (
                  <button type="button" className="min-h-[44px] px-2 font-bold text-err-ink" onClick={() => act.remove.mutate(l.id)}>
                    Quitar
                  </button>
                ) : null}
              </td>
            </tr>
          ))}
          {cot.lineas.length === 0 ? (
            <tr>
              <td colSpan={4} className="py-3 text-ink-2">
                Aún no hay ítems.
              </td>
            </tr>
          ) : null}
        </tbody>
        <tfoot>
          <tr className="border-t-2 border-ink font-extrabold">
            <td className="py-2" colSpan={2}>
              Total
            </td>
            <td className="text-right font-mono">{formatoCOP(cot.total)}</td>
            <td />
          </tr>
        </tfoot>
      </table>

      {editable ? (
        <>
          <Field label="Buscar servicio o repuesto" value={buscar} onChange={(e) => setBuscar(e.target.value)} />
          <ul className="flex max-h-64 flex-col gap-1 overflow-auto">
            {cat.data?.servicios.map((s) => (
              <li key={s.id}>
                <button
                  type="button"
                  className="flex min-h-[44px] w-full justify-between rounded-xl px-3 text-left hover:bg-primary-tint"
                  onClick={() => act.add.mutate({ tipo: 'servicio', refId: s.id, cantidad: 1 })}
                >
                  <span>Servicio · {s.nombre}</span>
                  <span className="font-mono">{s.precio == null ? 'precio por confirmar' : formatoCOP(s.precio)}</span>
                </button>
              </li>
            ))}
            {cat.data?.repuestos.map((p) => (
              <li key={p.id}>
                <button
                  type="button"
                  className="flex min-h-[44px] w-full justify-between rounded-xl px-3 text-left hover:bg-primary-tint"
                  onClick={() => act.add.mutate({ tipo: 'repuesto', refId: p.id, cantidad: 1 })}
                >
                  <span>
                    Repuesto · {p.nombre} <span className="text-ink-2">({p.existencia} en bodega)</span>
                  </span>
                  <span className="font-mono">{formatoCOP(p.precioVenta)}</span>
                </button>
              </li>
            ))}
          </ul>
          <Button
            missing={cot.lineas.length === 0 ? 'Agrega al menos un ítem para enviarla.' : undefined}
            busy={act.send.isPending}
            onClick={() => act.send.mutate()}
          >
            Enviar cotización al cliente
          </Button>
        </>
      ) : null}
      {puedeRegistrar && (cot.estado === 'enviada' || cot.estado === 'dudas') ? (
        <div className="flex flex-wrap gap-3">
          <Button variant="secondary" onClick={() => act.respond.mutate({ decision: 'aprobar', canal: 'telefono' })}>
            Aprobó por teléfono
          </Button>
          <Button variant="secondary" onClick={() => act.respond.mutate({ decision: 'aprobar', canal: 'mostrador' })}>
            Aprobó en el mostrador
          </Button>
          <Button variant="ghost" onClick={() => act.respond.mutate({ decision: 'rechazar', canal: 'telefono' })}>
            No aprobó
          </Button>
        </div>
      ) : null}
      {err ? <Alert>{errorText(err)}</Alert> : null}
    </section>
  );
}
