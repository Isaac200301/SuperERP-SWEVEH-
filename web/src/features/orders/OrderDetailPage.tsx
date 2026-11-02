import { Link, useParams } from 'react-router';

import type { EstadoOrden, Orden } from '../../shared/api/client';
import { estados, flujo } from '../../shared/i18n/es-CO';
import { formatoFechaHora, formatoHora } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { IconBack, IconCheck, IconWarn } from '../../shared/ui/icons';
import { Plate } from '../../shared/ui/Plate';
import { StatePill } from '../../shared/ui/StatePill';
import { useOrder } from './api';

/** Stepper de los 7 estados con la hora de cada paso (docs/10 §4.3). */
export function StateStepper({ orden }: { orden: Orden }) {
  const actual = flujo.indexOf(orden.estado);
  const horas = new Map<EstadoOrden, string>();
  for (const h of orden.historial ?? []) horas.set(h.hacia, h.ocurridoEn);
  return (
    <ol className="grid grid-cols-2 gap-2 sm:grid-cols-4 xl:grid-cols-7" aria-label="Estados de la orden">
      {flujo.map((e, i) => {
        const hecho = actual >= 0 && i < actual;
        const aqui = i === actual;
        const hora = horas.get(e);
        return (
          <li
            key={e}
            aria-current={aqui ? 'step' : undefined}
            className={`flex flex-col gap-1 rounded-2xl p-3 text-sm ${aqui ? 'bg-nav font-extrabold' : hecho ? 'bg-primary-tint' : 'bg-surface text-ink-2'}`}
          >
            <span className="flex items-center gap-1.5">
              {hecho ? <IconCheck size={14} /> : null}
              {estados[e]}
            </span>
            <span className="font-mono text-xs">{hora ? formatoHora(hora) : '—'}</span>
          </li>
        );
      })}
    </ol>
  );
}

export function OrderDetailPage() {
  const { id = '' } = useParams();
  const q = useOrder(id);
  if (q.isPending) return <p role="status">Cargando la orden…</p>;
  if (q.isError) {
    return (
      <Alert title="No pudimos abrir la orden">
        {q.error.message}{' '}
        <Link to="/panel/ordenes" className="font-bold underline">
          Volver a órdenes
        </Link>
      </Alert>
    );
  }
  const o = q.data;
  return (
    <>
      <header className="flex flex-col gap-1">
        <Link to="/panel/ordenes" className="inline-flex min-h-[44px] items-center gap-1 self-start font-bold text-primary">
          <IconBack size={18} />
          Órdenes
        </Link>
        <div className="flex flex-wrap items-center gap-3">
          <h1 className="font-mono text-[28px] font-bold">{o.codigo}</h1>
          <Plate value={o.placa} size="lg" />
          <StatePill estado={o.estado} />
        </div>
        <p className="text-ink-2">Abierta el {formatoFechaHora(o.creadoEn)}</p>
      </header>
      <section className="rounded-card bg-white p-5">
        <StateStepper orden={o} />
      </section>
      <div className="grid gap-5 lg:grid-cols-2">
        <section className="flex flex-col gap-2 rounded-card bg-white p-5">
          <h2 className="text-[17px] font-extrabold">Lo que dice el cliente</h2>
          <p className="whitespace-pre-wrap">{o.sintoma}</p>
        </section>
        <section className="flex flex-col gap-2 rounded-card bg-white p-5">
          <h2 className="text-[17px] font-extrabold">Vehículo y cliente</h2>
          <p>
            {[o.vehiculo?.marca, o.vehiculo?.linea, o.vehiculo?.modeloAnio].filter(Boolean).join(' ') ||
              (o.vehiculo?.tipo === 'moto' ? 'Moto' : 'Carro')}
          </p>
          {o.cliente ? <p className="text-ink-2">{o.cliente.nombre}</p> : null}
          {o.vehiculo?.motorPendiente ? (
            <p className="flex items-center gap-2 rounded-xl bg-warn-tint p-3 text-sm font-bold text-warn-ink">
              <IconWarn size={16} />
              Motor pendiente: falta cilindraje o turbo
            </p>
          ) : null}
        </section>
      </div>
    </>
  );
}
