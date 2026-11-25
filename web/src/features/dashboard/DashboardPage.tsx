import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { Link } from 'react-router';

import { api, unwrap, type EstadoOrden, type Schemas } from '../../shared/api/client';
import { estados, flujoFor } from '../../shared/i18n/es-CO';
import { Alert } from '../../shared/ui/Alert';
import { IconStop, IconWarn } from '../../shared/ui/icons';
import { Plate } from '../../shared/ui/Plate';
import { Segmented } from '../../shared/ui/Segmented';
import { useIsMobile } from '../../shared/ui/useIsMobile';
import { useSession } from '../auth/api';
import { DesktopHeader } from '../panel/DesktopHeader';
import { useSettings } from '../workshop/SettingsForm';
import { DashboardMobile } from './DashboardMobile';

type Pendiente = Schemas['Pendiente'];

/** Fondos pastel de las tarjetas, en el orden de Figma D1: aqua, durazno, lila, aqua, durazno (decorativos, DR-11). */
const fondos = ['bg-kpi-aqua', 'bg-kpi-peach', 'bg-kpi-lilac'];

/** HU-68…73 y HU-76: los números del taller, cada uno con "qué significa" (HU-75), y lo pendiente de hoy. */
export function DashboardPage() {
  if (useIsMobile()) return <DashboardMobile />;
  return <DashboardPageDesktop />;
}

/** "Buenos días" / "Buenas tardes" / "Buenas noches" y la fecha larga, con la hora de Colombia (R-16). */
function saludo(now = new Date()): { hola: string; fecha: string } {
  const hora = Number(new Intl.DateTimeFormat('es-CO', { hour: 'numeric', hourCycle: 'h23', timeZone: 'America/Bogota' }).format(now));
  const hola = hora < 12 ? 'Buenos días' : hora < 19 ? 'Buenas tardes' : 'Buenas noches';
  const larga = new Intl.DateTimeFormat('es-CO', { weekday: 'long', day: 'numeric', month: 'long', timeZone: 'America/Bogota' })
    .format(now)
    .replace(',', '');
  return { hola, fecha: larga.charAt(0).toUpperCase() + larga.slice(1) };
}

const placaEn = /\b[A-Z]{3}\d{2}[A-Z\d]?\b/;

/** El pendiente trae la placa dentro del título ("Mazda 2 JNX907 · 4 días sin recoger"): se saca para mostrarla como placa (DR-09). */
export function conPlaca(p: Pendiente): { placa: string | null; titulo: string } {
  const placa = placaEn.exec(p.titulo)?.[0] ?? null;
  if (!placa) return { placa, titulo: p.titulo };
  return { placa, titulo: p.titulo.replace(placa, '').replace(/\s{2,}/g, ' ').replace(/^\s*·\s*|\s*·\s*$/g, '').trim() };
}

/** A dónde lleva cada pendiente (HU-76). */
function accion(p: Pendiente): { to: string; label: string } | null {
  if (p.ordenId) return { to: `/panel/ordenes/${p.ordenId}`, label: 'Ver orden' };
  if (p.conversacionId) return { to: `/panel/conversaciones/${p.conversacionId}`, label: 'Atender' };
  if (p.tipo === 'borradores') return { to: '/panel/por-revisar', label: 'Revisar' };
  if (p.tipo === 'inventario') return { to: '/panel/repuestos?bajo=1', label: 'Ver repuestos' };
  return null;
}

/** DR-01: la urgencia lleva ícono y palabra, no solo color. */
const urgencia = {
  err: { texto: 'Urgente', cls: 'bg-err-tint text-err-ink', Icono: IconStop },
  warn: { texto: 'Atención', cls: 'bg-warn-tint text-warn-ink', Icono: IconWarn },
  info: null,
} as const;

function PendienteFila({ p }: Readonly<{ p: Pendiente }>) {
  const { placa, titulo } = conPlaca(p);
  const a = accion(p);
  const u = urgencia[p.tono];
  return (
    <li className="flex items-center gap-3 py-2">
      {placa ? <Plate value={placa} /> : null}
      <div className="flex min-w-0 flex-1 flex-col">
        <span className="flex flex-wrap items-center gap-2 text-[15px] font-bold">
          {titulo}
          {u ? (
            <span className={`inline-flex items-center gap-1 rounded-full px-2 py-px text-xs font-bold ${u.cls}`}>
              <u.Icono size={12} strokeWidth={2.4} />
              {u.texto}
            </span>
          ) : null}
        </span>
        <span className="text-[13px] text-ink-2">{p.detalle}</span>
      </div>
      {a ? (
        <Link
          to={a.to}
          className="inline-flex min-h-[44px] flex-none items-center rounded-btn border-[1.5px] border-line bg-white px-[18px] text-[15px] font-extrabold hover:border-primary"
        >
          {a.label}
        </Link>
      ) : null}
    </li>
  );
}

function DashboardPageDesktop() {
  const [tipo, setTipo] = useState<'' | 'carro' | 'moto'>('');
  const kpis = useQuery({
    queryKey: ['tablero', tipo],
    queryFn: async () => unwrap(await api.GET('/dashboard', { params: { query: tipo ? { tipo } : {} } })),
    refetchInterval: 60_000,
  });
  const taller = useQuery({ queryKey: ['taller'], queryFn: async () => unwrap(await api.GET('/workshop')) });
  const settings = useSettings().data;
  const hoy = useQuery({ queryKey: ['hoy'], queryFn: async () => unwrap(await api.GET('/today')).pendientes, refetchInterval: 60_000 });
  const nombre = useSession().data?.usuario.nombre.split(' ')[0] ?? '';
  const { hola, fecha } = saludo();
  const pasos: EstadoOrden[] = [...flujoFor(settings), ...(settings?.usarEsperandoRepuesto ? (['esperando_repuesto'] as EstadoOrden[]) : [])].filter(
    (e) => e !== 'entregado',
  );
  const max = Math.max(1, ...pasos.map((e) => kpis.data?.porEstado[e] ?? 0));
  return (
    <>
      <DesktopHeader title={`${hola}${nombre ? `, ${nombre}` : ''}`} subtitle={fecha} />
      {kpis.isError ? <Alert>{kpis.error.message}</Alert> : null}
      {kpis.data?.ejemplo ? (
        <p role="note" className="rounded-2xl bg-primary-tint p-3 text-sm">
          Los números marcados <b>ejemplo</b> incluyen órdenes de ejemplo para que el tablero no salga vacío.
        </p>
      ) : null}
      {kpis.data ? (
        <ul className="grid items-start gap-3.5 [grid-template-columns:repeat(auto-fit,minmax(190px,1fr))]">
          {kpis.data.kpis.map((k, i) => (
            <li key={k.codigo} className={`flex flex-col gap-1 rounded-[20px] p-[18px] ${fondos[i % fondos.length]}`}>
              <span className="text-[13px] font-bold text-ink-2">
                {k.titulo}
                {k.ejemplo ? <span className="ml-2 rounded-full bg-warn-tint px-2 py-0.5 text-xs text-warn-ink">ejemplo</span> : null}
              </span>
              <span className="font-mono text-[26px] leading-tight font-bold break-words">{k.texto}</span>
              <span className="text-[13px] text-ink-2">{k.detalle}</span>
              <details className="text-[13px] text-ink-2">
                <summary className="flex min-h-[44px] cursor-pointer items-center font-bold text-primary">¿Qué significa?</summary>
                {k.queSignifica}
              </details>
            </li>
          ))}
        </ul>
      ) : (
        <p role="status">Cargando…</p>
      )}
      <div className="grid items-start gap-5 lg:grid-cols-2">
        <section className="flex flex-col gap-2 rounded-card bg-white p-5" aria-labelledby="hoy">
          <h2 id="hoy" className="text-[18px] font-extrabold">
            Hoy tienes pendiente
          </h2>
          {hoy.data && hoy.data.length > 0 ? (
            <ul className="flex flex-col divide-y divide-line">
              {hoy.data.map((p, i) => (
                <PendienteFila key={`${p.tipo}-${i}`} p={p} />
              ))}
            </ul>
          ) : (
            <p className="text-[15px] text-ink-2">{hoy.isPending ? 'Cargando…' : 'No tienes nada pendiente por ahora.'}</p>
          )}
        </section>
        {kpis.data ? (
          <section className="flex flex-col gap-3 rounded-card bg-white p-5" aria-labelledby="por-estado">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <h2 id="por-estado" className="text-[18px] font-extrabold">
                Órdenes por estado
              </h2>
              {taller.data?.atiende === 'ambos' ? ( // HU-68 y HU-70 (Should): mezclar carros y motos falsea los promedios
                <Segmented
                  label="Ver"
                  hideLabel
                  value={tipo}
                  onChange={setTipo}
                  options={[
                    { value: '', label: 'Todo' },
                    { value: 'carro', label: 'Carros' },
                    { value: 'moto', label: 'Motos' },
                  ]}
                />
              ) : null}
            </div>
            <ul className="flex flex-col gap-2.5">
              {pasos.map((e) => {
                const n = kpis.data.porEstado[e] ?? 0;
                const espera = e === 'esperando_aprobacion' || e === 'esperando_repuesto';
                return (
                  <li key={e} className="flex items-center gap-3 text-[14px]">
                    <span className="w-[170px] flex-none text-ink-2">{estados[e]}</span>
                    <span className="h-3.5 max-w-[260px] flex-1">
                      <span
                        className={`block h-full rounded-full ${espera ? 'bg-warn' : 'bg-primary'}`}
                        style={{ width: n === 0 ? 0 : `${Math.max(6, (n / max) * 100)}%` }}
                      />
                    </span>
                    <b className="w-6 font-mono">{n}</b>
                  </li>
                );
              })}
            </ul>
          </section>
        ) : null}
      </div>
    </>
  );
}
