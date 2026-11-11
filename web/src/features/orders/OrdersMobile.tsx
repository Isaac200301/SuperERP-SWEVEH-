import { useState } from 'react';
import { Link, useSearchParams } from 'react-router';

import type { EstadoOrden, Orden } from '../../shared/api/client';
import { estados } from '../../shared/i18n/es-CO';
import { formatoFechaHora } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { Chips } from '../../shared/ui/Chips';
import { IconCar, IconChevron, IconMoto } from '../../shared/ui/icons';
import { Plate } from '../../shared/ui/Plate';
import { StatePill } from '../../shared/ui/StatePill';
import { SearchBox } from '../panel/SearchBox';
import { useRecentOrders } from './api';

export function vehiculoDe(o: Orden): string {
  return [o.vehiculo?.marca, o.vehiculo?.linea, o.vehiculo?.modeloAnio].filter(Boolean).join(' ') || (o.vehiculo?.tipo === 'moto' ? 'Moto' : 'Carro');
}

/** Tarjeta de una orden en el celular. La que espera al cliente va resaltada en limón (DR-01: con su ícono y palabra). */
export function OrderCard({ orden: o }: { orden: Orden }) {
  const espera = o.estado === 'esperando_aprobacion';
  const Tipo = o.vehiculo?.tipo === 'moto' ? IconMoto : IconCar;
  return (
    <Link
      to={`/panel/ordenes/${o.id}`}
      className={`flex flex-col gap-2.5 rounded-[22px] p-3.5 text-ink ${espera ? 'bg-[#FFFBE6] shadow-[inset_0_0_0_2px_var(--color-warn)]' : 'bg-white'}`}
    >
      <span className="flex flex-wrap items-center gap-2">
        <Plate value={o.placa} />
        <span className="font-mono text-[13px] text-ink-2">{o.codigo}</span>
        <span className="ml-auto">
          <StatePill estado={o.estado} />
        </span>
      </span>
      <span className="flex items-center gap-3">
        <span className="grid h-10 w-10 flex-none place-items-center rounded-[13px] bg-surface text-ink-2">
          <Tipo size={20} />
        </span>
        <span className="flex min-w-0 flex-col">
          <b className="truncate text-[15px]">{vehiculoDe(o)}</b>
          <span className="truncate text-[13px] text-ink-2">{o.cliente?.nombre ?? 'Sin cliente'}</span>
        </span>
      </span>
      <span className="flex items-center justify-between text-[13px]">
        <span className={espera ? 'font-extrabold text-warn-ink' : 'font-semibold text-ink-2'}>
          {o.fechaPrometida ? `Prometido ${formatoFechaHora(o.fechaPrometida)}` : `Abierta ${formatoFechaHora(o.creadoEn)}`}
        </span>
        <span className="inline-flex items-center gap-0.5 font-extrabold text-primary">
          Abrir
          <IconChevron size={14} />
        </span>
      </span>
    </Link>
  );
}

const cerradas: EstadoOrden[] = ['entregado', 'cancelada'];

/** Órdenes en el celular (mockup Ordenes): buscador, chips por estado con su conteo y tarjetas (HU-20, HU-68). */
export function OrdersMobile() {
  const q = useRecentOrders();
  // Desde las barras de Taller llega ?estado=… para abrir ya filtrado.
  const [params] = useSearchParams();
  const [filtro, setFiltro] = useState<'abiertas' | EstadoOrden | 'todas'>((params.get('estado') as EstadoOrden | null) ?? 'abiertas');
  const todas = q.data?.ordenes ?? [];
  const abiertas = todas.filter((o) => !cerradas.includes(o.estado));
  const presentes = [...new Set([...todas.map((o) => o.estado), ...(filtro !== 'abiertas' && filtro !== 'todas' ? [filtro] : [])])];
  const lista = filtro === 'todas' ? todas : filtro === 'abiertas' ? abiertas : todas.filter((o) => o.estado === filtro);
  const espera = abiertas.filter((o) => o.estado === 'esperando_aprobacion').length;
  return (
    <>
      <header className="-mx-4 -mt-4 flex flex-col gap-3.5 rounded-b-[28px] bg-white px-[18px] pt-5 pb-4">
        <div>
          <h1 className="text-[26px] font-extrabold tracking-[-.02em]">Órdenes</h1>
          <p className="text-[13px] text-ink-2">
            {abiertas.length} {abiertas.length === 1 ? 'abierta' : 'abiertas'}
            {espera ? ` · ${espera} espera${espera === 1 ? '' : 'n'} al cliente` : ''}
          </p>
        </div>
        <SearchBox placeholder="Placa, cliente u OS" tone="surface" />
      </header>
      <Chips
        label="Filtrar por estado"
        value={filtro}
        onChange={setFiltro}
        options={[
          { value: 'abiertas', label: 'Abiertas', count: abiertas.length },
          ...presentes
            .filter((e) => !cerradas.includes(e))
            .map((e) => ({ value: e, label: estados[e], count: todas.filter((o) => o.estado === e).length })),
          { value: 'todas', label: 'Todas', count: todas.length },
        ]}
      />
      {q.isPending ? <p role="status" className="text-ink-2">Cargando órdenes…</p> : null}
      {q.isError ? <Alert title="No pudimos cargar las órdenes">{q.error.message}</Alert> : null}
      {q.isSuccess && lista.length === 0 ? <p className="rounded-[22px] bg-white p-4 text-sm text-ink-2">No hay órdenes aquí.</p> : null}
      <div className="flex flex-col gap-2.5">
        {lista.map((o) => (
          <OrderCard key={o.id} orden={o} />
        ))}
      </div>
      <p className="text-center text-xs text-ink-2">Entregado y Cancelada no cuentan como abiertas.</p>
    </>
  );
}
