import { useQuery } from '@tanstack/react-query';
import { useState, type ComponentType } from 'react';
import { Link } from 'react-router';

import { api, unwrap, type Schemas } from '../../shared/api/client';
import { Alert } from '../../shared/ui/Alert';
import { BottomSheet } from '../../shared/ui/BottomSheet';
import { Button } from '../../shared/ui/Button';
import { Chips } from '../../shared/ui/Chips';
import {
  IconBolt,
  IconBox,
  IconCar,
  IconChart,
  IconChevron,
  IconInbox,
  IconOrders,
  IconStop,
  IconWarn,
} from '../../shared/ui/icons';
import { Logo } from '../../shared/ui/Logo';
import { can, useSession } from '../auth/api';
import { usePendingCards } from '../inbox/usePendingCards';
import { useRecentOrders } from '../orders/api';
import { OrderCard } from '../orders/OrdersMobile';
import { SearchBox } from '../panel/SearchBox';

type Pendiente = Schemas['Pendiente'];
type Kpi = Schemas['Kpi'];

/** A dónde lleva cada pendiente (las mismas acciones del tablero de escritorio, HU-76). */
export function pendingAction(p: Pendiente): { to: string; label: string } | null {
  if (p.ordenId) return { to: `/panel/ordenes/${p.ordenId}`, label: 'Ver la orden' };
  if (p.conversacionId) return { to: `/panel/conversaciones/${p.conversacionId}`, label: 'Atender' };
  if (p.tipo === 'borradores') return { to: '/panel/por-revisar', label: 'Revisar ahora' };
  if (p.tipo === 'inventario') return { to: '/panel/repuestos?bajo=1', label: 'Ver inventario' };
  return null;
}

const tonos = {
  err: { tile: 'bg-err-tint text-err-ink', tag: 'Urgente', icon: IconStop },
  warn: { tile: 'bg-warn-tint text-warn-ink', tag: 'Atención', icon: IconWarn },
  info: { tile: 'bg-primary-tint text-primary', tag: 'Para ti', icon: IconInbox },
} as const;
const sinSonido = () => undefined;
const kpiFondos = ['bg-kpi-peach', 'bg-kpi-lilac', 'bg-kpi-aqua'];

/** "Buen día" / "Buenas tardes" / "Buenas noches" y la fecha, con la hora de Colombia (R-16). */
function saludo(now = new Date()): { hola: string; fecha: string } {
  const hora = Number(new Intl.DateTimeFormat('es-CO', { hour: 'numeric', hourCycle: 'h23', timeZone: 'America/Bogota' }).format(now));
  const hola = hora < 12 ? 'Buen día' : hora < 19 ? 'Buenas tardes' : 'Buenas noches';
  const fecha = new Intl.DateTimeFormat('es-CO', { weekday: 'short', day: 'numeric', month: 'short', timeZone: 'America/Bogota' })
    .format(now)
    .replace(/\./g, '')
    .replace(',', '');
  return { hola, fecha };
}

function QuickLink({ to, label, icon: Icon, tone, badge }: { to: string; label: string; icon: ComponentType<{ size?: number }>; tone: string; badge?: number | undefined }) {
  return (
    <Link to={to} className="flex w-[72px] flex-none flex-col items-center gap-1.5 text-center text-xs font-bold text-ink">
      <span className={`relative grid h-14 w-14 place-items-center rounded-[18px] ${tone}`}>
        <Icon size={24} />
        {badge ? (
          <span className="absolute -top-1 -right-1 grid h-5 min-w-5 place-items-center rounded-full border-2 border-white bg-primary px-1 text-[11px] font-extrabold text-white">
            {badge}
          </span>
        ) : null}
      </span>
      {label}
    </Link>
  );
}

/** Hoy en el celular (mockup Main): pendientes del día, números con "¿Qué significa?" y órdenes en curso. */
export function DashboardMobile() {
  const session = useSession().data;
  const [seg, setSeg] = useState<'todo' | 'clientes' | 'taller'>('todo');
  const [kpi, setKpi] = useState<Kpi | null>(null);
  const kpis = useQuery({ queryKey: ['tablero', ''], queryFn: async () => unwrap(await api.GET('/dashboard', { params: { query: {} } })), refetchInterval: 60_000 });
  const hoy = useQuery({ queryKey: ['hoy'], queryFn: async () => unwrap(await api.GET('/today')).pendientes, refetchInterval: 60_000 });
  const ordenes = useRecentOrders();
  const pending = usePendingCards(false, sinSonido); // el sonido y el título ya los maneja la barra
  const abiertas = (ordenes.data?.ordenes ?? []).filter((o) => o.estado !== 'entregado' && o.estado !== 'cancelada');
  const todos = hoy.data ?? [];
  const deTaller = (p: Pendiente) => p.tipo === 'inventario';
  const lista = todos.filter((p) => seg === 'todo' || (seg === 'taller') === deTaller(p));
  const { hola, fecha } = saludo();
  const nombre = session?.usuario.nombre.split(' ')[0] ?? '';

  return (
    <>
      <header className="-mx-4 -mt-4 flex flex-col gap-4 rounded-b-[28px] bg-primary px-[18px] pt-5 pb-[22px] text-white">
        <div className="flex items-center gap-3">
          <Logo size={40} light />
          <div className="min-w-0 flex-1">
            <h1 className="text-[21px] font-extrabold tracking-[-.02em]">
              {hola}
              {nombre ? `, ${nombre}` : ''}
            </h1>
            <p className="truncate text-[13px] text-[#CFE9E4]">
              {session?.taller.nombre} · {fecha}
            </p>
          </div>
        </div>
        <SearchBox />
      </header>

      <section aria-labelledby="h-qa" className="flex flex-col gap-1.5">
        <h2 id="h-qa" className="text-[17px] font-extrabold">
          Accesos rápidos
        </h2>
        <nav aria-label="Accesos rápidos" className="-mx-4 flex gap-2.5 overflow-x-auto px-4 pb-1 [scrollbar-width:none]">
          <QuickLink to="/panel/ordenes" label="Órdenes" icon={IconOrders} tone="bg-primary-tint text-primary" badge={abiertas.length || undefined} />
          {can(session, 'inbox:manage') ? (
            <QuickLink to="/panel/por-revisar" label="Por revisar" icon={IconInbox} tone="bg-kpi-lilac text-[#3E2E8C]" badge={pending || undefined} />
          ) : null}
          {can(session, 'customer:read_all') ? <QuickLink to="/panel/clientes" label="Clientes" icon={IconCar} tone="bg-kpi-peach text-[#8A3E00]" /> : null}
          {can(session, 'part:manage') ? <QuickLink to="/panel/repuestos" label="Inventario" icon={IconBox} tone="bg-kpi-aqua text-primary" /> : null}
          {can(session, 'automation:manage') ? (
            <QuickLink to="/panel/automatizaciones" label="Automati­zaciones" icon={IconBolt} tone="bg-primary-tint text-primary" />
          ) : null}
          <QuickLink to="/panel/mas" label="Números" icon={IconChart} tone="bg-kpi-lilac text-[#3E2E8C]" />
        </nav>
      </section>

      <section aria-labelledby="h-pend" className="flex flex-col gap-2.5">
        <div className="flex items-center justify-between">
          <h2 id="h-pend" className="text-[17px] font-extrabold">
            Hoy tienes pendiente
          </h2>
          <span className="text-[13px] font-bold text-ink-2">
            {lista.length} {lista.length === 1 ? 'pendiente' : 'pendientes'}
          </span>
        </div>
        <Chips
          label="Filtrar pendientes"
          value={seg}
          onChange={setSeg}
          options={[
            { value: 'todo', label: 'Todo', count: todos.length },
            { value: 'clientes', label: 'Clientes', count: todos.filter((p) => !deTaller(p)).length },
            { value: 'taller', label: 'Taller', count: todos.filter(deTaller).length },
          ]}
        />
        {hoy.isError ? <Alert>{hoy.error.message}</Alert> : null}
        {hoy.isSuccess && lista.length === 0 ? <p className="rounded-[22px] bg-white p-4 text-sm text-ink-2">Nada pendiente por aquí.</p> : null}
        <div className="-mx-4 flex gap-3 overflow-x-auto px-4 pb-1 [scrollbar-width:none]">
          {lista.map((p, i) => {
            const t = tonos[p.tono];
            const a = pendingAction(p);
            return (
              <article key={`${p.tipo}-${i}`} className="flex w-[250px] flex-none flex-col gap-2.5 rounded-[22px] bg-white p-3.5">
                <div className="flex items-center gap-2.5">
                  <span className={`grid h-10 w-10 place-items-center rounded-[13px] ${t.tile}`}>
                    <t.icon size={20} />
                  </span>
                  <span className={`rounded-full px-2.5 py-0.5 text-xs font-extrabold ${t.tile}`}>{t.tag}</span>
                </div>
                <div className="flex-1">
                  <b className="block text-[15px] leading-tight">{p.titulo}</b>
                  <span className="text-[13px] text-ink-2">{p.detalle}</span>
                </div>
                {a ? (
                  <Link to={a.to} className="flex min-h-[44px] items-center justify-center rounded-[14px] bg-surface text-sm font-extrabold text-primary">
                    {a.label}
                  </Link>
                ) : null}
              </article>
            );
          })}
        </div>
      </section>

      {kpis.data ? (
        <section aria-labelledby="h-num" className="flex flex-col gap-2.5">
          <div className="flex items-center justify-between">
            <h2 id="h-num" className="text-[17px] font-extrabold">
              Taller en números
            </h2>
            <Link to="/panel/mas" className="flex min-h-[44px] items-center text-[13px] font-bold text-primary">
              Ver todo
            </Link>
          </div>
          {kpis.data.ejemplo ? <p className="text-xs text-ink-2">Incluye órdenes de ejemplo.</p> : null}
          <div className="-mx-4 flex gap-2.5 overflow-x-auto px-4 pb-1 [scrollbar-width:none]">
            {kpis.data.kpis.map((k, i) => (
              <div key={k.codigo} className={`flex w-[150px] flex-none flex-col gap-1 rounded-[20px] p-3.5 ${kpiFondos[i % kpiFondos.length]}`}>
                <span className={`font-mono font-bold break-words ${k.texto.length > 9 ? 'text-[17px]' : 'text-[22px]'}`}>{k.texto}</span>
                <span className="text-[13px] font-bold text-[#33443F]">{k.titulo}</span>
                <button type="button" onClick={() => setKpi(k)} className="mt-1 min-h-[32px] self-start rounded-full bg-white/75 px-2.5 text-xs font-bold text-primary">
                  ¿Qué significa?
                </button>
              </div>
            ))}
          </div>
        </section>
      ) : null}

      <section aria-labelledby="h-curso" className="flex flex-col gap-2">
        <div className="flex items-center justify-between">
          <h2 id="h-curso" className="text-[17px] font-extrabold">
            Órdenes en curso
          </h2>
          <Link to="/panel/ordenes" className="flex min-h-[44px] items-center gap-1 text-[13px] font-bold text-primary">
            Ver las {abiertas.length}
            <IconChevron size={14} />
          </Link>
        </div>
        {abiertas.slice(0, 3).map((o) => (
          <OrderCard key={o.id} orden={o} />
        ))}
        {ordenes.isSuccess && abiertas.length === 0 ? <p className="rounded-[22px] bg-white p-4 text-sm text-ink-2">No hay órdenes abiertas.</p> : null}
      </section>

      <BottomSheet open={kpi !== null} onClose={() => setKpi(null)} title={kpi?.titulo ?? ''} eyebrow="¿QUÉ SIGNIFICA?">
        {kpi ? (
          <>
            <div className="flex flex-col gap-0.5 rounded-[20px] bg-kpi-aqua p-3.5">
              <span className="font-mono text-[30px] font-bold">{kpi.texto}</span>
              <span className="text-sm">{kpi.detalle}</span>
            </div>
            <p className="text-[15px] leading-normal">{kpi.queSignifica}</p>
            <Button onClick={() => setKpi(null)}>Entendido</Button>
          </>
        ) : null}
      </BottomSheet>
    </>
  );
}
