import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router';

import { api, unwrap, type EstadoOrden } from '../../shared/api/client';
import { estados, flujo } from '../../shared/i18n/es-CO';
import { Alert } from '../../shared/ui/Alert';

const tonos = { err: 'bg-err-tint text-err-ink', warn: 'bg-warn-tint text-warn-ink', info: 'bg-primary-tint' } as const;

/** HU-68…73 y HU-76: los 6 números del taller, cada uno con "qué significa" (HU-75), y lo pendiente de hoy. */
export function DashboardPage() {
  const kpis = useQuery({ queryKey: ['tablero'], queryFn: async () => unwrap(await api.GET('/dashboard')), refetchInterval: 60_000 });
  const hoy = useQuery({ queryKey: ['hoy'], queryFn: async () => unwrap(await api.GET('/today')).pendientes, refetchInterval: 60_000 });
  return (
    <>
      <header>
        <h1 className="text-[28px] font-extrabold">Tablero</h1>
      </header>
      {hoy.data && hoy.data.length > 0 ? (
        <section className="flex flex-col gap-2 rounded-card bg-white p-5" aria-labelledby="hoy">
          <h2 id="hoy" className="text-[17px] font-extrabold">
            Pendiente hoy
          </h2>
          <ul className="flex flex-col gap-2">
            {hoy.data.map((p, i) => (
              <li key={`${p.tipo}-${i}`} className={`rounded-2xl p-3 text-sm ${tonos[p.tono]}`}>
                <b>{p.titulo}</b> {p.detalle}{' '}
                {p.ordenId ? (
                  <Link className="font-bold underline" to={`/panel/ordenes/${p.ordenId}`}>
                    Ver orden
                  </Link>
                ) : null}
                {p.tipo === 'inventario' ? (
                  <Link className="font-bold underline" to="/panel/repuestos?bajo=1">
                    Ver repuestos
                  </Link>
                ) : null}
              </li>
            ))}
          </ul>
        </section>
      ) : null}
      {kpis.isError ? <Alert>{kpis.error.message}</Alert> : null}
      {kpis.data ? (
        <>
          <ul className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
            {kpis.data.kpis.map((k) => (
              <li key={k.codigo} className="flex flex-col gap-1 rounded-card bg-white p-5">
                <span className="text-sm font-bold text-ink-2">{k.titulo}</span>
                <span className="text-[28px] font-extrabold">{k.texto}</span>
                <span className="text-sm">{k.detalle}</span>
                <details className="text-sm text-ink-2">
                  <summary className="min-h-[44px] cursor-pointer font-bold text-primary">¿Qué significa?</summary>
                  {k.queSignifica}
                </details>
              </li>
            ))}
          </ul>
          <section className="flex flex-col gap-2 rounded-card bg-white p-5" aria-labelledby="por-estado">
            <h2 id="por-estado" className="text-[17px] font-extrabold">
              Órdenes abiertas por estado
            </h2>
            <ul className="grid grid-cols-2 gap-2 sm:grid-cols-4 xl:grid-cols-6">
              {flujo
                .filter((e) => e !== 'entregado')
                .map((e: EstadoOrden) => (
                  <li key={e} className="rounded-2xl bg-surface p-3">
                    <span className="block text-sm">{estados[e]}</span>
                    <b className="text-xl">{kpis.data.porEstado[e] ?? 0}</b>
                  </li>
                ))}
            </ul>
          </section>
        </>
      ) : (
        <p role="status">Cargando…</p>
      )}
    </>
  );
}
