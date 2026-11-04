import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { Link } from 'react-router';

import { api, unwrap, type EstadoOrden } from '../../shared/api/client';
import { estados, flujoFor } from '../../shared/i18n/es-CO';
import { useSettings } from '../workshop/SettingsForm';
import { Alert } from '../../shared/ui/Alert';
import { Segmented } from '../../shared/ui/Segmented';

const tonos = { err: 'bg-err-tint text-err-ink', warn: 'bg-warn-tint text-warn-ink', info: 'bg-primary-tint' } as const;

/** HU-68…73 y HU-76: los 6 números del taller, cada uno con "qué significa" (HU-75), y lo pendiente de hoy. */
export function DashboardPage() {
  const [tipo, setTipo] = useState<'' | 'carro' | 'moto'>('');
  const kpis = useQuery({
    queryKey: ['tablero', tipo],
    queryFn: async () => unwrap(await api.GET('/dashboard', { params: { query: tipo ? { tipo } : {} } })),
    refetchInterval: 60_000,
  });
  const taller = useQuery({ queryKey: ['taller'], queryFn: async () => unwrap(await api.GET('/workshop')) });
  const settings = useSettings().data;
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
                {p.conversacionId ? (
                  <Link className="font-bold underline" to={`/panel/conversaciones/${p.conversacionId}`}>
                    Atender
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
      {taller.data?.atiende === 'ambos' ? ( // HU-68 y HU-70 (Should): mezclar carros y motos falsea los promedios
        <Segmented
          label="Ver"
          value={tipo}
          onChange={setTipo}
          options={[
            { value: '', label: 'Todo' },
            { value: 'carro', label: 'Carros' },
            { value: 'moto', label: 'Motos' },
          ]}
        />
      ) : null}
      {kpis.data?.ejemplo ? (
        <p role="note" className="rounded-2xl bg-primary-tint p-3 text-sm">
          Los números marcados <b>ejemplo</b> incluyen órdenes de ejemplo para que el tablero no salga vacío.
        </p>
      ) : null}
      {kpis.data ? (
        <>
          <ul className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
            {kpis.data.kpis.map((k) => (
              <li key={k.codigo} className="flex flex-col gap-1 rounded-card bg-white p-5">
                <span className="text-sm font-bold text-ink-2">
                  {k.titulo}
                  {k.ejemplo ? <span className="ml-2 rounded-full bg-warn-tint px-2 py-0.5 text-xs text-warn-ink">ejemplo</span> : null}
                </span>
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
              {[...flujoFor(settings), ...(settings?.usarEsperandoRepuesto ? (['esperando_repuesto'] as EstadoOrden[]) : [])]
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
