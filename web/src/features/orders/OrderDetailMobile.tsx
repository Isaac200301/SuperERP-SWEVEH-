import { useQuery } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router';

import { api, unwrap, type EstadoOrden, type Orden } from '../../shared/api/client';
import { estados, flujoFor } from '../../shared/i18n/es-CO';
import { formatoFechaHora, formatoHora } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { IconBack, IconCheck, IconClock, IconWarn } from '../../shared/ui/icons';
import { Plate } from '../../shared/ui/Plate';
import { StatePill } from '../../shared/ui/StatePill';
import { Tabs } from '../../shared/ui/Tabs';
import { useSettings } from '../workshop/SettingsForm';
import { DiagnosisPanel } from './DiagnosisPanel';
import { OrderMessages } from './OrderDetailPage';
import { OrderActions, QuotePanel, ReceptionPanel } from './OrderWork';
import { vehiculoDe } from './OrdersMobile';

type Pestana = 'estado' | 'cotizacion' | 'whatsapp' | 'historial';

/** Los estados en vertical, con la hora de cada paso (mockup Orden). */
function VerticalStepper({ orden }: { orden: Orden }) {
  const flujo = flujoFor(useSettings().data);
  const actual = flujo.indexOf(orden.estado === 'esperando_repuesto' ? 'en_reparacion' : orden.estado);
  const horas = new Map<EstadoOrden, string>();
  for (const h of orden.historial ?? []) horas.set(h.hacia, h.ocurridoEn);
  return (
    <ol aria-label="Estados de la orden" className="flex flex-col rounded-[22px] bg-white p-4">
      {flujo.map((e, i) => {
        const hecho = actual >= 0 && i < actual;
        const aqui = i === actual;
        const hora = horas.get(e);
        return (
          <li key={e} aria-current={aqui ? 'step' : undefined} className="flex gap-3">
            <span className="flex flex-col items-center">
              <span
                className={`grid h-6 w-6 flex-none place-items-center rounded-full ${
                  hecho ? 'bg-primary text-white' : aqui ? 'bg-nav text-ink shadow-[0_0_0_3px_var(--color-primary)]' : 'bg-surface shadow-[inset_0_0_0_2px_var(--color-line)]'
                }`}
              >
                {hecho ? <IconCheck size={14} strokeWidth={3} /> : null}
              </span>
              {i < flujo.length - 1 ? <span className={`min-h-3 w-0.5 flex-1 ${hecho ? 'bg-primary' : 'bg-line'}`} /> : null}
            </span>
            <span className="flex flex-1 items-start justify-between gap-2 pb-3">
              <b className={`text-sm ${aqui ? 'font-extrabold' : hecho ? 'font-semibold' : 'font-semibold text-ink-2'}`}>
                {estados[e]}
                {aqui && orden.estado === 'esperando_repuesto' ? ' · esperando repuesto' : ''}
              </b>
              <span className="font-mono text-xs text-ink-2">{hora ? formatoHora(hora) : '—'}</span>
            </span>
          </li>
        );
      })}
    </ol>
  );
}

/** HU-13: lo que se le ha hecho al vehículo, y los cambios de estado de esta orden con su hora. */
function Historial({ orden }: { orden: Orden }) {
  const id = orden.vehiculo?.id ?? '';
  const q = useQuery({
    queryKey: ['vehiculos', id, 'historial'],
    enabled: id !== '',
    queryFn: async () => unwrap(await api.GET('/vehicles/{id}/history', { params: { path: { id } } })).ordenes,
  });
  const cambios = [...(orden.historial ?? [])].reverse();
  return (
    <>
      <section aria-labelledby="h-cambios" className="flex flex-col gap-2 rounded-[22px] bg-white p-4">
        <h3 id="h-cambios" className="text-[15px] font-extrabold">
          Cambios de estado
        </h3>
        {cambios.length === 0 ? <p className="text-sm text-ink-2">Todavía no hay cambios.</p> : null}
        <ul className="flex flex-col gap-2 text-sm">
          {cambios.map((h, i) => (
            <li key={`${h.hacia}-${i}`} className="flex justify-between gap-2">
              <span>
                {h.desde ? `${estados[h.desde]} → ` : 'Abierta en '}
                <b>{estados[h.hacia]}</b>
              </span>
              <span className="font-mono text-xs text-ink-2">{formatoFechaHora(h.ocurridoEn)}</span>
            </li>
          ))}
        </ul>
      </section>
      <section aria-labelledby="h-veh" className="flex flex-col gap-2 rounded-[22px] bg-white p-4">
        <h3 id="h-veh" className="text-[15px] font-extrabold">
          Historial de {orden.placa}
        </h3>
        {q.isPending && id ? <p role="status" className="text-sm text-ink-2">Cargando historial…</p> : null}
        {q.isError ? <Alert>{q.error.message}</Alert> : null}
        <ul className="flex flex-col gap-1.5">
          {(q.data ?? [])
            .filter(({ orden: o }) => o.id !== orden.id)
            .map(({ orden: o }) => (
              <li key={o.id}>
                <Link to={`/panel/ordenes/${o.id}`} className="flex min-h-[44px] items-center justify-between gap-2 rounded-xl bg-surface px-3 py-2 text-sm">
                  <span className={o.estado === 'cancelada' ? 'line-through' : ''}>
                    <b className="font-mono">{o.codigo}</b> · {estados[o.estado]}
                    <br />
                    <span className="text-xs text-ink-2">{formatoFechaHora(o.creadoEn)}</span>
                  </span>
                </Link>
              </li>
            ))}
        </ul>
        {q.isSuccess && q.data.filter(({ orden: o }) => o.id !== orden.id).length === 0 ? (
          <p className="text-sm text-ink-2">Es la primera vez que viene.</p>
        ) : null}
      </section>
    </>
  );
}

/** La orden en el celular (mockup Orden): encabezado compacto, pestañas y la acción principal fija abajo. */
export function OrderDetailMobile({ orden: o }: { orden: Orden }) {
  const [params] = useSearchParams();
  const [tab, setTab] = useState<Pestana>('estado');
  const recibir = params.get('recibir') === '1';
  useEffect(() => {
    if (recibir) document.getElementById('recepcion')?.scrollIntoView({ block: 'start' });
  }, [recibir]);
  const cotizable = !['por_recibir', 'recibido', 'cancelada'].includes(o.estado);
  const conAcciones = o.estado !== 'entregado' && o.estado !== 'cancelada';
  return (
    <div className={`flex flex-col gap-4 ${conAcciones ? 'pb-[calc(96px+env(safe-area-inset-bottom))]' : ''}`}>
      <header className="-mx-4 -mt-4 flex flex-col gap-3 rounded-b-[28px] bg-white px-4 pt-3 pb-4">
        <div className="flex items-center gap-2">
          <Link to="/panel/ordenes" aria-label="Volver a órdenes" className="grid h-11 w-11 place-items-center rounded-[14px] bg-surface">
            <IconBack size={20} />
          </Link>
          <h1 className="flex-1 text-center font-mono text-[17px] font-bold">{o.codigo}</h1>
          <span className="h-11 w-11" aria-hidden="true" />
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Plate value={o.placa} size="lg" />
          <StatePill estado={o.estado} />
        </div>
        <div>
          <b className="text-[19px]">{vehiculoDe(o)}</b>
          <p className="text-sm text-ink-2">{o.cliente?.nombre ?? 'Sin cliente'}</p>
        </div>
        <div className="flex flex-wrap gap-2 text-[13px]">
          <span className="inline-flex items-center gap-1.5 rounded-xl bg-surface px-3 py-2">
            <IconClock size={16} />
            {o.fechaPrometida ? `Prometido ${formatoFechaHora(o.fechaPrometida)}` : 'Sin fecha prometida'}
          </span>
          {o.vehiculo?.motorPendiente ? (
            <span className="inline-flex items-center gap-1.5 rounded-xl bg-warn-tint px-3 py-2 font-bold text-warn-ink">
              <IconWarn size={16} />
              Ficha pendiente
            </span>
          ) : null}
        </div>
      </header>

      <Tabs
        label="Secciones de la orden"
        value={tab}
        onChange={setTab}
        tabs={[
          { value: 'estado', label: 'Estado' },
          { value: 'cotizacion', label: 'Cotización' },
          { value: 'whatsapp', label: 'WhatsApp' },
          { value: 'historial', label: 'Historial' },
        ]}
      >
        {tab === 'estado' ? (
          <>
            <VerticalStepper orden={o} />
            <section className="flex flex-col gap-1 rounded-[22px] bg-white p-4">
              <h3 className="text-[15px] font-extrabold">Lo que dice el cliente</h3>
              <p className="whitespace-pre-wrap text-sm">{o.sintoma}</p>
            </section>
            <ReceptionPanel orden={o} />
            {!['por_recibir', 'cancelada'].includes(o.estado) ? <DiagnosisPanel orden={o} /> : null}
          </>
        ) : null}
        {tab === 'cotizacion' ? (
          cotizable ? (
            <QuotePanel orden={o} />
          ) : (
            <p className="rounded-[22px] bg-white p-4 text-sm text-ink-2">La cotización se arma después del diagnóstico.</p>
          )
        ) : null}
        {tab === 'whatsapp' ? <OrderMessages orden={o} /> : null}
        {tab === 'historial' ? <Historial orden={o} /> : null}
      </Tabs>
      {conAcciones ? <OrderActions orden={o} bar /> : null}
    </div>
  );
}
