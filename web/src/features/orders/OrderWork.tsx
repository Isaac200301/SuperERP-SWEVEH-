import { useEffect, useState, type FormEvent } from 'react';

import { ApiError, type EstadoOrden, type Orden, type Schemas } from '../../shared/api/client';
import { estados, flujoFor } from '../../shared/i18n/es-CO';
import { formatoCOP, formatoFechaHora, formatoHora } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Button } from '../../shared/ui/Button';
import { Field } from '../../shared/ui/Field';
import {
  photoUrl,
  useAddEvidence,
  useCatalog,
  useOrderParts,
  useChecklist,
  maxFotosPorNota,
  useEvidence,
  useQuote,
  useQuoteActions,
  useSaveChecklist,
  useTransition,
  type ItemRevisado,
} from './work';
import { usePermission } from '../auth/api';
import { useSettings } from '../workshop/SettingsForm';

export const cotizarEvento = 'sweveh:cotizar';

type Evidencia = Schemas['Evidencia'];
/** Junta las fotos que se subieron con la misma nota (grupoId); la primera trae la nota. */
export function agrupar(list: Evidencia[]): Evidencia[][] {
  const grupos = new Map<string, Evidencia[]>();
  for (const x of list) {
    const k = x.grupoId ?? x.id;
    grupos.set(k, [...(grupos.get(k) ?? []), x]);
  }
  return [...grupos.values()];
}
const estadosItem: [ItemRevisado['estado'], string][] = [['bien', 'Bien'], ['mal', 'Mal'], ['no_tiene', 'No tiene']];
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
  const [devueltos, setDevueltos] = useState<string[]>([]);
  const [sinAccesorios, setSinAccesorios] = useState(false);
  const [falta, setFalta] = useState('');
  const settings = useSettings().data;
  // Lo que el cliente dejó al recibir es lo que se devuelve al entregar (HU-25; casco en motos, HU-18).
  const cl = useChecklist(orden.id, orden.estado === 'listo');
  const flujo = flujoFor(settings);
  const i = flujo.indexOf(orden.estado);
  if (orden.estado === 'entregado' || orden.estado === 'cancelada') return null;
  // Esperando repuesto solo vuelve a En reparación; Control de calidad apagado con la orden adentro sale a Listo.
  let siguiente: EstadoOrden | undefined = flujo[i + 1];
  let anterior: EstadoOrden | undefined = i > 0 ? flujo[i - 1] : undefined;
  if (orden.estado === 'esperando_repuesto') [siguiente, anterior] = ['en_reparacion', undefined];
  else if (i < 0 && orden.estado === 'control_calidad') [siguiente, anterior] = ['listo', 'en_reparacion'];
  else if (i < 0) return null;
  const sinAprobacion = siguiente === 'en_reparacion' && orden.estado !== 'esperando_repuesto';
  const entrega = siguiente === 'entregado';
  const puedeEsperar = orden.estado === 'en_reparacion' && settings?.usarEsperandoRepuesto === true;
  const dejados = cl.data?.accesorios ?? [];
  const accesoriosOK = dejados.length > 0 ? dejados.every((a) => devueltos.includes(a)) : sinAccesorios;
  let faltaEntrega: string | undefined;
  if (entrega) {
    if (cl.isPending) faltaEntrega = 'Cargando los accesorios del checklist…';
    else if (!recogido.trim()) faltaEntrega = 'Escribe quién recoge.';
    else if (!accesoriosOK) faltaEntrega = dejados.length > 0 ? 'Marca cada accesorio que devolviste.' : 'Confirma que no dejó accesorios.';
  }
  const marcarDevuelto = (a: string, si: boolean) => setDevueltos(si ? [...devueltos, a] : devueltos.filter((x) => x !== a));

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
          {dejados.length > 0 ? (
            <fieldset className="flex flex-col gap-1">
              <legend className="font-bold">Devuelve lo que dejó al recibir</legend>
              {dejados.map((a) => (
                <label key={a} className="flex min-h-[44px] items-center gap-2">
                  <input type="checkbox" checked={devueltos.includes(a)} onChange={(e) => marcarDevuelto(a, e.target.checked)} />
                  {a.charAt(0).toUpperCase() + a.slice(1)}
                </label>
              ))}
            </fieldset>
          ) : cl.isSuccess ? (
            <label className="flex min-h-[44px] items-center gap-2">
              <input type="checkbox" checked={sinAccesorios} onChange={(e) => setSinAccesorios(e.target.checked)} />
              No dejó accesorios
            </label>
          ) : null}
        </>
      ) : null}
      {mover.isError ? <Alert>{errorText(mover.error)}</Alert> : null}
      <div className="flex flex-wrap gap-3">
        {siguiente ? (
          <Button
            busy={mover.isPending}
            missing={faltaEntrega}
            onClick={() =>
              mover.mutate({
                estado: siguiente,
                version: orden.version,
                ...(motivo ? { motivo } : {}),
                ...(entrega ? { recogidoPor: recogido.trim(), accesoriosConfirmados: accesoriosOK } : {}),
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
      {puedeEsperar ? (
        <div className="flex flex-wrap items-end gap-3 border-t border-line pt-3">
          <div className="min-w-[220px] flex-1">
            <Field label="¿Qué repuesto falta?" value={falta} onChange={(e) => setFalta(e.target.value)} placeholder="Kit de embrague, llega el jueves" />
          </div>
          <Button
            variant="secondary"
            busy={mover.isPending}
            missing={falta.trim() ? undefined : 'Escribe qué repuesto falta.'}
            onClick={() => mover.mutate({ estado: 'esperando_repuesto', version: orden.version, motivo: falta.trim() }, { onSuccess: () => setFalta('') })}
          >
            Poner en Esperando repuesto
          </Button>
        </div>
      ) : null}
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
  const [fotos, setFotos] = useState<File[]>([]);
  const [items, setItems] = useState<ItemRevisado[] | null>(null);

  if (cl.isPending) return <p role="status">Cargando checklist…</p>;
  if (cl.isError) return <Alert>{cl.error.message}</Alert>;
  const c = cl.data;
  const lados = c.tipo === 'moto' ? ladosMoto : ladosCarro;
  const revisados = items ?? c.items ?? [];
  const marcar = (clave: string, cambio: Partial<ItemRevisado>) =>
    setItems(revisados.map((it) => (it.clave === clave ? { ...it, ...cambio } : it)));
  const abierta = orden.estado !== 'entregado' && orden.estado !== 'cancelada';

  const onSave = (e: FormEvent) => {
    e.preventDefault();
    guardar.mutate({
      kilometraje: km === '' ? (c.kilometraje ?? null) : Number(km),
      combustible: fuel === '' ? (c.combustible ?? null) : fuel,
      accesorios: acc === '' ? c.accesorios : acc.split(',').map((s) => s.trim()).filter(Boolean),
      items: revisados,
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
          {revisados.length ? (
            <fieldset className="flex flex-col gap-2 sm:col-span-3">
              <legend className="mb-1 text-[15px] font-extrabold">Revisión al recibir</legend>
              {revisados.map((it) => (
                <div key={it.clave} className="flex flex-wrap items-center gap-2">
                  <span className="w-44 font-bold">{it.nombre}</span>
                  <div role="radiogroup" aria-label={it.nombre} className="flex gap-1">
                    {estadosItem.map(([v, t]) => (
                      <button
                        key={v}
                        type="button"
                        role="radio"
                        aria-checked={it.estado === v}
                        className={`min-h-[40px] rounded-full px-3 text-sm font-bold ${it.estado === v ? 'bg-ink text-white' : 'bg-surface'}`}
                        onClick={() => marcar(it.clave, { estado: v })}
                      >
                        {t}
                      </button>
                    ))}
                  </div>
                  {it.estado === 'mal' ? (
                    <div className="min-w-[200px] flex-1">
                      <Field label={`Qué tiene ${it.nombre}`} value={it.nota ?? ''} onChange={(e) => marcar(it.clave, { nota: e.target.value })} />
                    </div>
                  ) : null}
                </div>
              ))}
            </fieldset>
          ) : null}
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
            if (!nota.trim() && fotos.length === 0) return;
            subir.mutate(
              { ...(nota.trim() ? { nota: nota.trim() } : {}), fotos },
              { onSuccess: () => { setNota(''); setFotos([]); } },
            );
          }}
        >
          <div className="min-w-[240px] flex-1">
            <Field label="Nota (lo que encontraste o cambiaste)" value={nota} onChange={(e) => setNota(e.target.value)} placeholder="Rayón en la puerta trasera" />
          </div>
          <label className="inline-flex min-h-[52px] cursor-pointer items-center rounded-btn border-[1.5px] border-line px-4 font-bold text-primary">
            {fotos.length ? `${fotos.length} foto${fotos.length > 1 ? 's' : ''}` : 'Agregar fotos'}
            <input
              type="file"
              accept="image/*"
              capture="environment"
              multiple
              aria-label="Fotos de la nota"
              className="sr-only"
              onChange={(e) => setFotos([...(e.target.files ?? [])].slice(0, maxFotosPorNota))}
            />
          </label>
          <Button type="submit" variant="ghost" missing={nota.trim() || fotos.length ? undefined : 'Escribe la nota o agrega fotos.'}>
            Guardar nota
          </Button>
        </form>
      ) : null}
      <ul className="flex flex-col gap-2">
        {agrupar((ev.data ?? []).filter((x) => !x.hallazgoId)).map((g) => (
          <li key={g[0]?.id} className="flex flex-wrap items-center gap-3 rounded-2xl bg-surface p-3 text-sm">
            {g.filter((x) => x.tieneFoto).map((x) => (
              <a key={x.id} href={photoUrl(orden.id, x.id)} target="_blank" rel="noopener noreferrer">
                <img src={photoUrl(orden.id, x.id)} alt={`Foto ${x.lado ?? x.nota ?? ''}`} className="h-14 w-14 rounded-xl object-cover" loading="lazy" />
              </a>
            ))}
            <span className="min-w-[160px] flex-1">{g[0]?.nota ?? (g[0]?.lado ? `Foto: ${g[0].lado}` : 'Fotos')}</span>
            <span className="font-mono text-xs text-ink-2">
              {estados[g[0]?.etapa as keyof typeof estados] ?? g[0]?.etapa} · {g[0] ? formatoHora(g[0].creadoEn) : ''}
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}

const pesos = (s: string) => (s.trim() === '' ? undefined : Number(s.replace(/\D/g, '')));

/** Un ítem del catálogo: los servicios sin precio piden el valor y los repuestos la cantidad antes de agregar. */
function CatalogItem({ label, detalle, precio, pidePrecio, pideCantidad, busy, onAdd }: {
  label: string; detalle?: string; precio: number | null; pidePrecio: boolean; pideCantidad: boolean; busy: boolean;
  onAdd: (v: { cantidad: number; precioUnitario?: number }) => void;
}) {
  const [cant, setCant] = useState('1');
  const [valor, setValor] = useState('');
  const n = pesos(cant) ?? 0;
  const v = pesos(valor);
  let falta: string | undefined;
  if (n < 1) falta = 'Escribe la cantidad.';
  else if (pidePrecio && v === undefined) falta = 'Escribe el precio.';
  return (
    <li className="flex flex-wrap items-end gap-2 rounded-xl px-3 py-2 hover:bg-primary-tint">
      <span className="min-w-[180px] flex-1">
        {label} {detalle ? <span className="text-ink-2">{detalle}</span> : null}
        <span className="ml-2 font-mono">{precio == null ? 'sin precio de lista' : formatoCOP(precio)}</span>
      </span>
      {pidePrecio ? <Field label={`Precio de ${label}`} inputMode="numeric" value={valor} onChange={(e) => setValor(e.target.value)} className="w-32" /> : null}
      {pideCantidad ? <Field label={`Cantidad de ${label}`} inputMode="numeric" value={cant} onChange={(e) => setCant(e.target.value)} className="w-20" /> : null}
      <Button variant="secondary" busy={busy} missing={falta} onClick={() => onAdd({ cantidad: n, ...(pidePrecio && v !== undefined ? { precioUnitario: v } : {}) })}>
        Agregar
      </Button>
    </li>
  );
}

/** Mano de obra u otro ítem libre: el precio lo pone la persona (RF-17) y se puede corregir mientras es borrador. */
function FreeItem({ busy, tarifa, onAdd }: {
  busy: boolean; tarifa: number | null | undefined;
  onAdd: (v: { descripcion: string; cantidad: number } & ({ precioUnitario: number } | { horas: number })) => void;
}) {
  const [desc, setDesc] = useState('');
  const [valor, setValor] = useState('');
  const [cant, setCant] = useState('1');
  const [horas, setHoras] = useState('');
  const porHoras = tarifa != null;
  const v = pesos(valor);
  const n = pesos(cant) ?? 0;
  const h = horas.trim() === '' ? undefined : Number(horas.replace(',', '.'));
  let falta: string | undefined;
  if (!desc.trim()) falta = 'Describe el trabajo.';
  else if (porHoras && h === undefined && v === undefined) falta = 'Escribe las horas o el valor.';
  else if (!porHoras && v === undefined) falta = 'Escribe el valor.';
  else if (n < 1) falta = 'Escribe la cantidad.';
  return (
    <form
      className="grid gap-3 rounded-2xl bg-surface p-3 sm:grid-cols-4"
      onSubmit={(e) => {
        e.preventDefault();
        if (falta) return;
        if (porHoras && h !== undefined) onAdd({ descripcion: desc.trim(), horas: h, cantidad: 1 });
        else if (v !== undefined) onAdd({ descripcion: desc.trim(), precioUnitario: v, cantidad: n });
        setDesc('');
        setValor('');
        setCant('1');
        setHoras('');
      }}
    >
      <div className="sm:col-span-2">
        <Field label="Mano de obra u otro ítem" value={desc} onChange={(e) => setDesc(e.target.value)} placeholder="Mano de obra cambio de pastillas" />
      </div>
      {porHoras ? (
        <Field label="Horas" inputMode="decimal" value={horas} onChange={(e) => setHoras(e.target.value)} hint={`× ${formatoCOP(tarifa)} la hora`} placeholder="1,5" />
      ) : null}
      <Field label="Valor (COP)" inputMode="numeric" value={valor} onChange={(e) => setValor(e.target.value)} hint={porHoras ? 'O un valor fijo en vez de horas' : undefined} />
      <Field label="Cantidad del ítem" inputMode="numeric" value={cant} onChange={(e) => setCant(e.target.value)} />
      <div className="sm:col-span-4">
        <Button type="submit" variant="secondary" busy={busy} missing={falta}>
          Agregar ítem
        </Button>
      </div>
    </form>
  );
}

/** HU-27, HU-29, HU-34 y HU-38: cotización con servicios, repuestos y mano de obra; precios de la lista o puestos por la persona. */
export function QuotePanel({ orden }: { orden: Orden }) {
  const q = useQuote(orden.id);
  const act = useQuoteActions(orden.id);
  const puedeRegistrar = usePermission('quote:register_response');
  const permisoDevolver = usePermission('part:return');
  const puedeDescontar = usePermission('quote:discount');
  const puedeEnviar = usePermission('quote:send');
  const params = useSettings().data;
  const [buscar, setBuscar] = useState('');
  const cat = useCatalog(buscar);
  const aprobada = q.data?.estado === 'aprobada';
  const movidos = useOrderParts(orden.id, aprobada);
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
  const err = act.add.error ?? act.remove.error ?? act.send.error ?? act.respond.error ?? act.giveBack.error ?? act.update.error ?? act.renew.error;
  const t = { subtotal: 0, descuento: 0, iva: 0, ivaPct: 0, preciosConIva: true, venceEn: null, vencida: false, ...cot };
  const devuelto = act.giveBack.data;
  const movPorRepuesto = new Map((movidos.data ?? []).map((m) => [m.repuestoId, m]));

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
          {cot.lineas.map((l) => {
            const mov = l.refId ? movPorRepuesto.get(l.refId) : undefined;
            return (
              <tr key={l.id} className="border-t border-line align-top">
                <td className="py-2">
                  {l.descripcion}
                  {l.horas != null ? <span className="ml-2 text-ink-2">({l.horas} h)</span> : null}
                  {l.sinStock ? <span className="ml-2 font-bold text-warn-ink">sin existencia</span> : null}
                  {editable && puedeDescontar ? (
                    <label className="mt-1 flex items-center gap-2 text-xs text-ink-2">
                      Descuento %
                      <input
                        aria-label={`Descuento de ${l.descripcion}`}
                        inputMode="numeric"
                        className="w-14 rounded border border-line px-1"
                        defaultValue={l.descuentoPct || ''}
                        onBlur={(e) => {
                          const v = pesos(e.target.value) ?? 0;
                          if (v !== (l.descuentoPct ?? 0)) act.update.mutate({ lineId: l.id, descuentoPct: v });
                        }}
                      />
                    </label>
                  ) : l.descuentoPct ? (
                    <span className="block text-xs text-ink-2">Descuento {l.descuentoPct} %</span>
                  ) : null}
                  {mov ? (
                    <span className="block text-xs text-ink-2">
                      Descontado {mov.descontado} · Devuelto {mov.devuelto}
                      {mov.pendiente ? ` · Pendiente ${mov.pendiente} (no había existencia)` : ''}
                    </span>
                  ) : null}
                </td>
                <td>
                  {editable ? (
                    <span className="inline-flex items-center gap-1">
                      <button
                        type="button"
                        aria-label={`Menos ${l.descripcion}`}
                        className="h-9 w-9 rounded-full bg-surface font-bold"
                        disabled={l.cantidad <= 1}
                        onClick={() => act.update.mutate({ lineId: l.id, cantidad: l.cantidad - 1 })}
                      >
                        −
                      </button>
                      <span className="w-6 text-center">{l.cantidad}</span>
                      <button
                        type="button"
                        aria-label={`Más ${l.descripcion}`}
                        className="h-9 w-9 rounded-full bg-surface font-bold"
                        onClick={() => act.update.mutate({ lineId: l.id, cantidad: l.cantidad + 1 })}
                      >
                        +
                      </button>
                    </span>
                  ) : (
                    l.cantidad
                  )}
                </td>
                <td className="text-right font-mono">{formatoCOP(l.subtotal)}</td>
                <td className="text-right">
                  {editable ? (
                    <button type="button" className="min-h-[44px] px-2 font-bold text-err-ink" onClick={() => act.remove.mutate(l.id)}>
                      Quitar
                    </button>
                  ) : null}
                  {permisoDevolver && aprobada && l.tipo === 'repuesto' && l.refId && mov && mov.porDevolver > 0 ? (
                    <button
                      type="button"
                      className="min-h-[44px] px-2 font-bold text-primary"
                      onClick={() => act.giveBack.mutate({ partId: l.refId ?? '', nombre: l.descripcion })}
                    >
                      Devolver 1 al inventario
                    </button>
                  ) : null}
                </td>
              </tr>
            );
          })}
          {cot.lineas.length === 0 ? (
            <tr>
              <td colSpan={4} className="py-3 text-ink-2">
                Aún no hay ítems.
              </td>
            </tr>
          ) : null}
        </tbody>
        <tfoot>
          {t.descuento || t.iva ? (
            <>
              <tr className="border-t border-line text-ink-2">
                <td className="py-1" colSpan={2}>Subtotal</td>
                <td className="text-right font-mono">{formatoCOP(t.subtotal)}</td>
                <td />
              </tr>
              {t.descuento ? (
                <tr className="text-ink-2">
                  <td className="py-1" colSpan={2}>Descuentos</td>
                  <td className="text-right font-mono">−{formatoCOP(t.descuento)}</td>
                  <td />
                </tr>
              ) : null}
              <tr className="text-ink-2">
                <td className="py-1" colSpan={2}>IVA {t.ivaPct} %{t.preciosConIva ? ' (incluido)' : ''}</td>
                <td className="text-right font-mono">{formatoCOP(t.iva)}</td>
                <td />
              </tr>
            </>
          ) : null}
          <tr className="border-t-2 border-ink font-extrabold">
            <td className="py-2" colSpan={2}>
              Total
            </td>
            <td className="text-right font-mono">{formatoCOP(cot.total)}</td>
            <td />
          </tr>
        </tfoot>
      </table>
      {t.venceEn && (cot.estado === 'enviada' || cot.estado === 'dudas') ? (
        <p className={`flex flex-wrap items-center gap-3 text-sm ${t.vencida ? 'font-bold text-err-ink' : 'text-ink-2'}`}>
          {t.vencida ? `Venció el ${formatoFechaHora(t.venceEn)}: renueva la vigencia para registrar la respuesta.` : `Vence el ${formatoFechaHora(t.venceEn)}`}
          {t.vencida && puedeEnviar ? (
            <Button variant="secondary" busy={act.renew.isPending} onClick={() => act.renew.mutate()}>
              Renovar vigencia
            </Button>
          ) : null}
        </p>
      ) : null}
      {devuelto ? (
        <p role="status" className="font-bold text-ok">
          Se devolvió {devuelto.devueltas} de {devuelto.nombre}. Quedan {devuelto.quedanPorDevolver} por devolver · {devuelto.existencia} en bodega.
        </p>
      ) : null}

      {editable ? (
        <>
          <Field label="Buscar servicio o repuesto" value={buscar} onChange={(e) => setBuscar(e.target.value)} />
          <ul className="flex max-h-80 flex-col gap-1 overflow-auto">
            {cat.data?.servicios.map((s) => (
              <CatalogItem
                key={s.id}
                label={`Servicio · ${s.nombre}`}
                precio={s.precio}
                pidePrecio={s.precio == null}
                pideCantidad={false}
                busy={act.add.isPending}
                onAdd={(v) => act.add.mutate({ tipo: 'servicio', refId: s.id, ...v })}
              />
            ))}
            {cat.data?.repuestos.map((p) => (
              <CatalogItem
                key={p.id}
                label={`Repuesto · ${p.nombre}`}
                detalle={`(${p.existencia} en bodega)`}
                precio={p.precioVenta}
                pidePrecio={false}
                pideCantidad
                busy={act.add.isPending}
                onAdd={(v) => act.add.mutate({ tipo: 'repuesto', refId: p.id, ...v })}
              />
            ))}
          </ul>
          <FreeItem busy={act.add.isPending} tarifa={params?.tarifaHora} onAdd={(v) => act.add.mutate({ tipo: 'servicio', ...v })} />
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
