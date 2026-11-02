import { Link } from 'react-router';

import { formatoFechaHora } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { IconPlus } from '../../shared/ui/icons';
import { Plate } from '../../shared/ui/Plate';
import { StatePill } from '../../shared/ui/StatePill';
import { useRecentOrders } from './api';

export function OrdersPage() {
  const q = useRecentOrders();
  return (
    <>
      <header className="flex flex-wrap items-center justify-between gap-3.5">
        <div>
          <h1 className="text-[28px] font-extrabold tracking-[-.02em]">Órdenes</h1>
          <p className="text-ink-2">Las más recientes primero.</p>
        </div>
        <Link
          to="/panel/ordenes/nueva"
          className="inline-flex min-h-[44px] items-center gap-2 rounded-full bg-primary px-5 text-[15px] font-extrabold text-white hover:bg-primary-pressed"
        >
          <IconPlus size={18} />
          Nueva orden
        </Link>
      </header>
      <section className="rounded-card bg-white p-5" aria-labelledby="titulo-recientes">
        <h2 id="titulo-recientes" className="sr-only">
          Órdenes recientes
        </h2>
        {q.isPending ? (
          <p className="text-ink-2" role="status">
            Cargando órdenes…
          </p>
        ) : q.isError ? (
          <Alert title="No pudimos cargar las órdenes">
            {q.error.message}{' '}
            <button type="button" className="font-bold underline" onClick={() => void q.refetch()}>
              Intentar de nuevo
            </button>
          </Alert>
        ) : q.data.ordenes.length === 0 ? (
          <div className="flex flex-col items-start gap-3 py-6">
            <b className="text-lg">Todavía no hay órdenes</b>
            <p className="text-ink-2">Escribe una placa y lo que dice el cliente; la orden queda con número y hora.</p>
            <Link to="/panel/ordenes/nueva" className="font-bold text-primary">
              Abrir la primera orden
            </Link>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[760px] border-collapse text-sm">
              <thead>
                <tr className="text-left text-[13px] text-ink-2">
                  {['Orden', 'Placa', 'Cliente', 'Vehículo', 'Estado', 'Abierta'].map((h, i, all) => (
                    <th
                      key={h}
                      scope="col"
                      className={`bg-surface px-3 py-2.5 font-bold ${i === 0 ? 'rounded-l-xl' : ''} ${i === all.length - 1 ? 'rounded-r-xl' : ''}`}
                    >
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {q.data.ordenes.map((o) => (
                  <tr key={o.id} className="border-b border-[#E3EAE8]">
                    <td className="whitespace-nowrap px-3 py-3 font-mono">
                      <Link to={`/panel/ordenes/${o.id}`} className="font-bold text-primary">
                        {o.codigo}
                      </Link>
                    </td>
                    <td className="px-3 py-3">
                      <Plate value={o.placa} />
                    </td>
                    <td className="whitespace-nowrap px-3 py-3">{o.cliente?.nombre ?? '—'}</td>
                    <td className="whitespace-nowrap px-3 py-3">
                      {[o.vehiculo?.marca, o.vehiculo?.linea, o.vehiculo?.modeloAnio].filter(Boolean).join(' ') ||
                        (o.vehiculo?.tipo === 'moto' ? 'Moto' : 'Carro')}
                    </td>
                    <td className="px-3 py-3">
                      <StatePill estado={o.estado} />
                    </td>
                    <td className="whitespace-nowrap px-3 py-3 font-mono text-ink-2">{formatoFechaHora(o.creadoEn)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </>
  );
}
