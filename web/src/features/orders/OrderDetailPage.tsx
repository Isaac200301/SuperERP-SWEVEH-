import { useQuery } from '@tanstack/react-query';
import { Link, useParams, useSearchParams } from 'react-router';

import { api, unwrap, type EstadoOrden, type Orden } from '../../shared/api/client';
import { estados, flujoFor } from '../../shared/i18n/es-CO';
import { formatoFechaHora, formatoHora } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { IconBack, IconCheck, IconWarn } from '../../shared/ui/icons';
import { Plate } from '../../shared/ui/Plate';
import { StatePill } from '../../shared/ui/StatePill';
import { useSession } from '../auth/api';
import { Thread } from '../messaging/Thread';
import { useSettings } from '../workshop/SettingsForm';
import { useOrder } from './api';
import { DiagnosisPanel } from './DiagnosisPanel';
import { OrderActions, QuotePanel, ReceptionPanel } from './OrderWork';

/** Stepper de los 7 estados con la hora de cada paso (docs/10 §4.3). */
export function StateStepper({ orden }: { orden: Orden }) {
  const flujo = flujoFor(useSettings().data);
  // Esperando repuesto es un desvío de En reparación: el stepper la muestra en ese paso.
  const actual = flujo.indexOf(orden.estado === 'esperando_repuesto' ? 'en_reparacion' : orden.estado);
  const horas = new Map<EstadoOrden, string>();
  for (const h of orden.historial ?? []) horas.set(h.hacia, h.ocurridoEn);
  return (
    <ol className="grid grid-cols-2 gap-2 sm:grid-cols-4 xl:grid-cols-7" aria-label="Estados de la orden">
      {flujo.map((e, i) => {
        const hecho = actual >= 0 && i < actual;
        const aqui = i === actual;
        const hora = horas.get(e);
        return (
          <li
            key={e}
            aria-current={aqui ? 'step' : undefined}
            className={`flex flex-col gap-1 rounded-2xl p-3 text-sm ${aqui ? 'bg-nav font-extrabold' : hecho ? 'bg-primary-tint' : 'bg-surface text-ink-2'}`}
          >
            <span className="flex items-center gap-1.5">
              {hecho ? <IconCheck size={14} /> : null}
              {estados[e]}
            </span>
            <span className="font-mono text-xs">{hora ? formatoHora(hora) : '—'}</span>
          </li>
        );
      })}
    </ol>
  );
}

/** HU-05 criterio 3: cómo le llegaría al cliente el aviso de "vehículo recibido". No se envía nada. */
function ReceivedPreview({ orden }: { orden: Orden }) {
  const taller = useSession().data?.taller.nombre ?? 'tu taller';
  const cliente = orden.cliente?.nombre ?? 'cliente';
  const que = orden.vehiculo?.tipo === 'moto' ? 'tu moto' : 'tu carro';
  return (
    <section aria-label="Vista previa del aviso" className="flex flex-col gap-3 rounded-card bg-white p-5">
      <h2 className="text-[17px] font-extrabold">Así le llegaría el aviso a tu cliente</h2>
      <div className="max-w-[420px] rounded-2xl bg-[#DCF8C6] p-4 text-[15px] text-ink">
        <p>
          Hola {cliente}, en <b>{taller}</b> recibimos {que} de placa <b>{orden.placa}</b> (orden {orden.codigo}). Te avisamos por aquí
          cuando tengamos el diagnóstico.
        </p>
        <div className="mt-3 grid h-20 place-items-center rounded-xl border-2 border-dashed border-line text-sm text-ink-2">Aquí van las fotos de la recepción</div>
      </div>
      <p className="text-sm text-ink-2">Es solo una vista previa: no se envió nada. Los avisos se activan cuando conectes WhatsApp.</p>
      {orden.ejemplo ? <p className="text-sm font-bold text-warn-ink">Esta es una orden de ejemplo y no cuenta en tus números.</p> : null}
    </section>
  );
}

/** HU-24: la conversación de WhatsApp de esta orden, con hora y quién envió cada mensaje. */
export function OrderMessages({ orden }: { orden: Orden }) {
  const q = useQuery({
    queryKey: ['orden-mensajes', orden.id],
    queryFn: async () => unwrap(await api.GET('/orders/{id}/messages', { params: { path: { id: orden.id } } })),
    refetchInterval: 15_000,
  });
  let body = <p role="status">Cargando mensajes…</p>;
  if (q.isError) body = <Alert>{q.error.message}</Alert>;
  else if (q.isSuccess) body = <Thread mensajes={q.data.mensajes} />;
  return (
    <section aria-labelledby="whatsapp" className="flex flex-col gap-3 rounded-card bg-white p-5">
      <h2 id="whatsapp" className="text-[17px] font-extrabold">
        WhatsApp
      </h2>
      {body}
    </section>
  );
}

/** HU-19: lo que respondió el cliente a la constancia de recibo. */
function ReceiptAnswer({ orden }: { orden: Orden }) {
  if (!orden.constancia || !orden.constanciaEn) return null;
  return orden.constancia === 'observado' ? (
    <Alert title="El cliente dice que algo no cuadra en el recibo">
      Respondió a las {formatoHora(orden.constanciaEn)}. Revisa la recepción con él antes de seguir.
    </Alert>
  ) : (
    <p className="flex items-center gap-2 text-sm font-bold text-ok-ink">
      <IconCheck size={16} />
      El cliente confirmó el recibo a las {formatoHora(orden.constanciaEn)}
    </p>
  );
}

export function OrderDetailPage() {
  const { id = '' } = useParams();
  const [params] = useSearchParams();
  const q = useOrder(id);
  if (q.isPending) return <p role="status">Cargando la orden…</p>;
  if (q.isError) {
    return (
      <Alert title="No pudimos abrir la orden">
        {q.error.message}{' '}
        <Link to="/panel/ordenes" className="font-bold underline">
          Volver a órdenes
        </Link>
      </Alert>
    );
  }
  const o = q.data;
  return (
    <>
      <header className="flex flex-col gap-1">
        <Link to="/panel/ordenes" className="inline-flex min-h-[44px] items-center gap-1 self-start font-bold text-primary">
          <IconBack size={18} />
          Órdenes
        </Link>
        <div className="flex flex-wrap items-center gap-3">
          <h1 className="font-mono text-[28px] font-bold">{o.codigo}</h1>
          <Plate value={o.placa} size="lg" />
          <StatePill estado={o.estado} />
        </div>
        <p className="text-ink-2">Abierta el {formatoFechaHora(o.creadoEn)}</p>
        <ReceiptAnswer orden={o} />
      </header>
      {params.get('primera') === '1' ? <ReceivedPreview orden={o} /> : null}
      <section className="rounded-card bg-white p-5">
        <StateStepper orden={o} />
      </section>
      <div className="grid gap-5 lg:grid-cols-2">
        <section className="flex flex-col gap-2 rounded-card bg-white p-5">
          <h2 className="text-[17px] font-extrabold">Lo que dice el cliente</h2>
          <p className="whitespace-pre-wrap">{o.sintoma}</p>
        </section>
        <section className="flex flex-col gap-2 rounded-card bg-white p-5">
          <h2 className="text-[17px] font-extrabold">Vehículo y cliente</h2>
          <p>
            {[o.vehiculo?.marca, o.vehiculo?.linea, o.vehiculo?.modeloAnio].filter(Boolean).join(' ') ||
              (o.vehiculo?.tipo === 'moto' ? 'Moto' : 'Carro')}
          </p>
          {o.cliente ? <p className="text-ink-2">{o.cliente.nombre}</p> : null}
          {o.vehiculo?.motorPendiente ? (
            <p className="flex items-center gap-2 rounded-xl bg-warn-tint p-3 text-sm font-bold text-warn-ink">
              <IconWarn size={16} />
              Ficha pendiente: falta combustible, caja o datos del motor
            </p>
          ) : null}
        </section>
      </div>
      <OrderActions orden={o} />
      <ReceptionPanel orden={o} />
      {!['por_recibir', 'cancelada'].includes(o.estado) ? <DiagnosisPanel orden={o} /> : null}
      {/* Se cotiza desde el diagnóstico; en reparación sale la adicional (HU-31). */}
      {!['por_recibir', 'recibido', 'cancelada'].includes(o.estado) ? <QuotePanel orden={o} /> : null}
      <OrderMessages orden={o} />
    </>
  );
}
