import { Link } from 'react-router';

import { formatoFechaHora } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { IconClock } from '../../shared/ui/icons';
import { Plate } from '../../shared/ui/Plate';
import { StatePill } from '../../shared/ui/StatePill';
import { useMyOrders } from './work';

/** HU-23: la lista del piso, pensada para el celular del mecánico (tarjetas grandes, un toque). */
export function FloorPage() {
  const q = useMyOrders();
  return (
    <>
      <header>
        <h1 className="text-[28px] font-extrabold">Piso</h1>
        <p className="text-ink-2">Tus órdenes abiertas, primero las que tienen fecha prometida.</p>
      </header>
      {q.isPending ? <p role="status">Cargando…</p> : null}
      {q.isError ? <Alert>{q.error.message}</Alert> : null}
      {q.data?.length === 0 ? <p className="rounded-card bg-white p-5 text-ink-2">No hay órdenes abiertas. ¡Buen trabajo!</p> : null}
      <ul className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        {(q.data ?? []).map((o) => (
          <li key={o.id}>
            <Link to={`/panel/ordenes/${o.id}`} className="flex min-h-[120px] flex-col gap-2 rounded-card bg-white p-5 hover:outline hover:outline-2 hover:outline-primary">
              <span className="flex flex-wrap items-center gap-3">
                <Plate value={o.placa} />
                <StatePill estado={o.estado} />
              </span>
              <span className="font-bold">
                {[o.vehiculo?.marca, o.vehiculo?.linea].filter(Boolean).join(' ') || (o.vehiculo?.tipo === 'moto' ? 'Moto' : 'Carro')}
                <span className="ml-2 font-mono text-sm text-ink-2">{o.codigo}</span>
              </span>
              <span className="line-clamp-2 text-ink-2">{o.sintoma}</span>
              {o.fechaPrometida ? (
                <span className="flex items-center gap-1.5 text-sm font-bold">
                  <IconClock size={16} />
                  Prometida: {formatoFechaHora(o.fechaPrometida)}
                </span>
              ) : null}
            </Link>
          </li>
        ))}
      </ul>
    </>
  );
}
