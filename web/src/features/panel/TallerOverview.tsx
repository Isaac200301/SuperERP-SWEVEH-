import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { Link } from 'react-router';

import { api, unwrap, type EstadoOrden } from '../../shared/api/client';
import { estados, flujoFor } from '../../shared/i18n/es-CO';
import { Alert } from '../../shared/ui/Alert';
import { IconCheck, IconSearch, IconStop, IconWarn } from '../../shared/ui/icons';
import { Tabs } from '../../shared/ui/Tabs';
import { can, useSession } from '../auth/api';
import { KpiCards } from '../dashboard/KpiCards';
import { useParts } from '../inventory/api';
import { useSettings } from '../workshop/SettingsForm';

/** HU-68 a HU-75 en el celular: los números con "¿Qué significa?" y las órdenes abiertas por estado, en barras. */
function Numeros() {
  const kpis = useQuery({ queryKey: ['tablero', ''], queryFn: async () => unwrap(await api.GET('/dashboard', { params: { query: {} } })), refetchInterval: 60_000 });
  const settings = useSettings().data;
  if (kpis.isPending) return <p role="status">Cargando…</p>;
  if (kpis.isError) return <Alert>{kpis.error.message}</Alert>;
  const pasos: EstadoOrden[] = [...flujoFor(settings), ...(settings?.usarEsperandoRepuesto ? (['esperando_repuesto'] as EstadoOrden[]) : [])].filter(
    (e) => e !== 'entregado',
  );
  const max = Math.max(1, ...pasos.map((e) => kpis.data.porEstado[e] ?? 0));
  return (
    <>
      {kpis.data.ejemplo ? <p className="text-xs text-ink-2">Incluye órdenes de ejemplo.</p> : null}
      <KpiCards kpis={kpis.data.kpis} layout="grid" />
      <section aria-labelledby="h-barras" className="flex flex-col gap-3 rounded-[22px] bg-white p-4">
        <div>
          <h2 id="h-barras" className="text-[15px] font-extrabold">
            Órdenes abiertas por estado
          </h2>
          <p className="text-xs text-ink-2">Toca un estado para ver sus órdenes</p>
        </div>
        <ul className="flex flex-col gap-1.5">
          {pasos.map((e) => {
            const n = kpis.data.porEstado[e] ?? 0;
            const espera = e === 'esperando_aprobacion' || e === 'esperando_repuesto';
            return (
              <li key={e}>
                <Link to={`/panel/ordenes?estado=${e}`} aria-label={`${estados[e]}: ${n}`} className="flex min-h-[44px] items-center gap-3">
                  <span className="w-[118px] flex-none text-[13px] text-ink-2">{estados[e]}</span>
                  <span className="h-3 flex-1 overflow-hidden rounded-full bg-surface">
                    <span className={`block h-full rounded-full ${espera ? 'bg-warn' : 'bg-primary'}`} style={{ width: `${(n / max) * 100}%` }} />
                  </span>
                  <b className="w-6 text-right font-mono">{n}</b>
                </Link>
              </li>
            );
          })}
        </ul>
      </section>
    </>
  );
}

/** HU-36 en el celular: existencia contra el mínimo, con ícono y palabra (DR-01). */
function Inventario() {
  const [q, setQ] = useState('');
  const parts = useParts({ q, categoria: '', bajo: false, inactivos: false });
  return (
    <>
      <label className="flex min-h-[50px] items-center gap-2.5 rounded-2xl bg-white px-3.5 text-ink-2">
        <IconSearch size={20} />
        <input
          type="search"
          aria-label="Buscar repuesto"
          placeholder="Buscar repuesto o código"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          className="min-w-0 flex-1 bg-transparent text-[15px] text-ink outline-none"
        />
      </label>
      {parts.isError ? <Alert>{parts.error.message}</Alert> : null}
      {parts.isSuccess && parts.data.length === 0 ? <p className="rounded-[22px] bg-white p-4 text-sm text-ink-2">No hay repuestos con ese nombre.</p> : null}
      <ul className="flex flex-col gap-2">
        {(parts.data ?? []).map((p) => {
          const st =
            p.existencia <= 0
              ? { t: 'Agotado', c: 'bg-err-tint text-err-ink', I: IconStop }
              : p.bajoMinimo
                ? { t: 'Bajo el mínimo', c: 'bg-warn-tint text-warn-ink', I: IconWarn }
                : { t: 'Bien', c: 'bg-ok-tint text-ok-ink', I: IconCheck };
          return (
            <li key={p.id} className="flex items-center gap-3 rounded-[18px] bg-white p-3.5">
              <span className="flex min-w-0 flex-1 flex-col">
                <b className="truncate text-[15px]">{p.nombre}</b>
                <span className="font-mono text-xs text-ink-2">
                  {p.referencia} · mín. {p.minimo}
                </span>
              </span>
              <span className="flex flex-col items-end gap-1">
                <b className="font-mono text-lg">{p.existencia}</b>
                <span className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-bold ${st.c}`}>
                  <st.I size={12} strokeWidth={2.4} />
                  {st.t}
                </span>
              </span>
            </li>
          );
        })}
      </ul>
      <Link to="/panel/repuestos" className="flex min-h-[52px] items-center justify-center rounded-btn bg-white font-extrabold text-primary">
        Registrar entrada de mercancía
      </Link>
    </>
  );
}

/** Mockup Taller: Números e Inventario del dueño, arriba de las demás secciones. */
export function TallerOverview() {
  const session = useSession().data;
  const inventario = can(session, 'part:manage');
  const [tab, setTab] = useState<'numeros' | 'inventario'>('numeros');
  if (!inventario) return <Numeros />;
  return (
    <Tabs
      label="Vista"
      value={tab}
      onChange={setTab}
      tabs={[
        { value: 'numeros', label: 'Números' },
        { value: 'inventario', label: 'Inventario' },
      ]}
    >
      {tab === 'numeros' ? <Numeros /> : <Inventario />}
    </Tabs>
  );
}
