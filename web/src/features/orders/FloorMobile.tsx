import { useState, type ReactNode } from 'react';
import { Link, Navigate, useParams } from 'react-router';

import { ApiError, type EstadoOrden, type Orden } from '../../shared/api/client';
import { estados, flujoFor } from '../../shared/i18n/es-CO';
import { formatoFechaHora } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Chips } from '../../shared/ui/Chips';
import { IconBack, IconCamera, IconClock, IconWarn } from '../../shared/ui/icons';
import { Plate } from '../../shared/ui/Plate';
import { StatePill } from '../../shared/ui/StatePill';
import { useSession } from '../auth/api';
import { SearchBox } from '../panel/SearchBox';
import { useSettings } from '../workshop/SettingsForm';
import { useOrder } from './api';
import { OrderActions, ReceptionPanel } from './OrderWork';
import { vehiculoDe } from './OrdersMobile';
import { useMyOrders, useTransition } from './work';

const btn = 'flex min-h-[52px] flex-1 items-center justify-center gap-2 rounded-btn text-[15px] font-extrabold';

/**
 * El paso que el mecánico puede dar desde la tarjeta (DR-04: 52 px). Lo que tiene puertas propias no va aquí:
 * recibir lleva a la recepción, la aprobación es del cliente y la entrega se hace en la orden.
 */
function NextStep({ o }: { o: Orden }) {
  const mover = useTransition(o.id);
  const flujo = flujoFor(useSettings().data);
  if (o.estado === 'por_recibir') {
    return (
      <Link to={`/panel/piso/recibir/${o.id}`} className={`${btn} bg-primary text-white`}>
        Recibir vehículo
      </Link>
    );
  }
  if (o.estado === 'esperando_aprobacion') {
    return <span className="flex items-center gap-1.5 text-[13px] font-bold text-warn-ink"><IconWarn size={16} />Espera la aprobación del cliente</span>;
  }
  if (o.estado === 'listo') return <span className="text-[13px] font-bold text-ok-ink">Listo para entregar en el mostrador</span>;
  const siguiente: EstadoOrden | undefined = o.estado === 'esperando_repuesto' ? 'en_reparacion' : flujo[flujo.indexOf(o.estado) + 1];
  if (!siguiente || siguiente === 'entregado') return null;
  return (
    <div className="flex flex-1 flex-col gap-2">
      <button
        type="button"
        disabled={mover.isPending}
        onClick={() => mover.mutate({ estado: siguiente, version: o.version })}
        className={`${btn} bg-primary text-white disabled:opacity-60`}
      >
        {mover.isPending ? 'Un momento…' : `Pasar a ${estados[siguiente]}`}
      </button>
      {mover.isError ? (
        <Alert>{mover.error instanceof ApiError && mover.error.status === 409 ? 'Alguien cambió esta orden. Ya la actualizamos.' : mover.error.message}</Alert>
      ) : null}
    </div>
  );
}

function FloorCard({ o }: { o: Orden }) {
  return (
    <article className="flex flex-col gap-3 rounded-[22px] bg-white p-4">
      <Link to={`/panel/ordenes/${o.id}`} className="flex flex-col gap-2 text-ink">
        <span className="flex flex-wrap items-center gap-2">
          <Plate value={o.placa} />
          <span className="ml-auto">
            <StatePill estado={o.estado} />
          </span>
        </span>
        <b className="text-[16px]">{vehiculoDe(o)}</b>
        <span className="line-clamp-2 text-[13px] text-ink-2">{o.sintoma}</span>
        {o.fechaPrometida ? (
          <span className="flex items-center gap-1.5 text-[13px] font-bold">
            <IconClock size={15} />
            Prometido {formatoFechaHora(o.fechaPrometida)}
          </span>
        ) : null}
      </Link>
      <div className="flex items-center gap-2.5">
        <NextStep o={o} />
        <Link to={`/panel/ordenes/${o.id}`} aria-label={`Agregar foto o nota a ${o.placa}`} className="grid h-[52px] w-[52px] flex-none place-items-center rounded-btn bg-surface text-ink">
          <IconCamera size={22} />
        </Link>
      </div>
    </article>
  );
}

type Filtro = 'todas' | 'recibir' | 'curso' | 'listas';
const enCurso = (e: EstadoOrden) => e !== 'por_recibir' && e !== 'listo';

/** Piso en el celular (mockup PisoHoy, HU-23 y HU-81): lo siguiente arriba y cada orden con su próximo paso. Sin costos. */
export function FloorMobile() {
  const q = useMyOrders();
  const nombre = useSession().data?.usuario.nombre.split(' ')[0] ?? '';
  const [filtro, setFiltro] = useState<Filtro>('todas');
  const lista = q.data ?? [];
  const siguiente = lista.find((o) => o.estado === 'por_recibir');
  const visibles = lista.filter(
    (o) =>
      o.id !== siguiente?.id &&
      (filtro === 'todas' || (filtro === 'recibir' ? o.estado === 'por_recibir' : filtro === 'listas' ? o.estado === 'listo' : enCurso(o.estado))),
  );
  return (
    <>
      <header className="-mx-4 -mt-4 flex flex-col gap-4 rounded-b-[28px] bg-primary px-[18px] pt-5 pb-5 text-white">
        <div>
          <h1 className="text-[22px] font-extrabold tracking-[-.02em]">Hola{nombre ? `, ${nombre}` : ''}</h1>
          <p className="text-[13px] text-[#CFE9E4]">
            {lista.length} {lista.length === 1 ? 'orden abierta' : 'órdenes abiertas'}
          </p>
        </div>
        <SearchBox placeholder="Escribe una placa" />
      </header>
      {q.isPending ? <p role="status">Cargando…</p> : null}
      {q.isError ? <Alert>{q.error.message}</Alert> : null}
      {siguiente ? (
        <section aria-labelledby="h-sig" className="flex flex-col gap-2">
          <span id="h-sig" className="text-xs font-extrabold tracking-[.06em] text-primary">
            LO SIGUIENTE
          </span>
          <FloorCard o={siguiente} />
        </section>
      ) : null}
      <Chips
        label="Filtrar mis órdenes"
        value={filtro}
        onChange={setFiltro}
        options={[
          { value: 'todas', label: 'Todas' },
          { value: 'recibir', label: 'Por recibir' },
          { value: 'curso', label: 'En curso' },
          { value: 'listas', label: 'Listas' },
        ]}
      />
      {q.isSuccess && lista.length === 0 ? <p className="rounded-[22px] bg-white p-4 text-ink-2">No hay órdenes abiertas. ¡Buen trabajo!</p> : null}
      <div className="flex flex-col gap-3">
        {visibles.map((o) => (
          <FloorCard key={o.id} o={o} />
        ))}
      </div>
    </>
  );
}

/**
 * Recibir desde el piso (mockup PisoChecklist, HU-17 y HU-18): la recepción de la orden a pantalla completa con
 * "Pasar a Recibido" fijo abajo; si falta algo, el panel lo dice (DR-06). Sin id: la próxima por recibir.
 */
export function ReceivePage() {
  const { id } = useParams();
  const mias = useMyOrders();
  if (!id) {
    if (mias.isPending) return <p role="status">Cargando…</p>;
    const proxima = mias.data?.find((o) => o.estado === 'por_recibir');
    if (proxima) return <Navigate to={`/panel/piso/recibir/${proxima.id}`} replace />;
    return (
      <>
        <BackHeader titulo="Recibir" />
        <p className="rounded-[22px] bg-white p-4 text-ink-2">No tienes vehículos por recibir.</p>
      </>
    );
  }
  return <ReceiveOrder id={id} />;
}

function BackHeader({ titulo, children }: { titulo: string; children?: ReactNode }) {
  return (
    <header className="-mx-4 -mt-4 flex items-center gap-3 rounded-b-[28px] bg-white px-4 pt-3 pb-4">
      <Link to="/panel/piso" aria-label="Volver a mis órdenes" className="grid h-11 w-11 flex-none place-items-center rounded-[14px] bg-surface">
        <IconBack size={20} />
      </Link>
      <div className="flex min-w-0 flex-1 flex-col">
        <h1 className="flex items-center gap-2 text-[19px] font-extrabold">{titulo}</h1>
        {children}
      </div>
    </header>
  );
}

function ReceiveOrder({ id }: { id: string }) {
  const q = useOrder(id);
  if (q.isPending) return <p role="status">Cargando la orden…</p>;
  if (q.isError) return <Alert title="No pudimos abrir la orden">{q.error.message}</Alert>;
  const o = q.data;
  return (
    <div className="flex flex-col gap-4 pb-[calc(110px+env(safe-area-inset-bottom))]">
      <BackHeader titulo="Recibir">
        <span className="flex items-center gap-2">
          <Plate value={o.placa} />
          <span className="truncate text-[13px] text-ink-2">
            {vehiculoDe(o)} · {o.cliente?.nombre ?? 'Sin cliente'}
          </span>
        </span>
      </BackHeader>
      <ReceptionPanel orden={o} />
      {o.estado === 'por_recibir' ? (
        <OrderActions orden={o} bar />
      ) : (
        <Link to={`/panel/ordenes/${o.id}`} className="flex min-h-[52px] items-center justify-center rounded-btn bg-white font-extrabold text-primary">
          Ya está {estados[o.estado]} · ver la orden
        </Link>
      )}
    </div>
  );
}
