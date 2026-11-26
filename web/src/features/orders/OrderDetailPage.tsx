import { useQuery } from '@tanstack/react-query';
import { Link, useParams, useSearchParams } from 'react-router';

import { api, unwrap, type EstadoOrden, type Orden } from '../../shared/api/client';
import { estados, flujoFor } from '../../shared/i18n/es-CO';
import { formatoFechaHora, formatoHora } from '../../shared/lib/format';
import { Alert } from '../../shared/ui/Alert';
import { IconBack, IconCheck, IconWarn } from '../../shared/ui/icons';
import { Plate } from '../../shared/ui/Plate';
import { StatePill } from '../../shared/ui/StatePill';
import { useIsMobile } from '../../shared/ui/useIsMobile';
import { useFeature, useSession } from '../auth/api';
import { Thread } from '../messaging/Thread';
import { useSettings } from '../workshop/SettingsForm';
import { useOrder } from './api';
import { DiagnosisPanel } from './DiagnosisPanel';
import { OrderAssistantCard } from '../assistant/OrderAssistantCard';
import { OrderDetailMobile } from './OrderDetailMobile';
import { OrderActions, QuotePanel, ReceptionPanel } from './OrderWork';
import { SpendPanel } from './SpendPanel';

/**
 * Los estados de la orden como fichas (Figma D2): lo hecho con ✓, el estado actual resaltado (limón con ▲ si es una espera,
 * lima si no) y lo que falta con borde. La hora de cada paso va en el título y para lectores de pantalla (DR-01).
 */
export function StateStepper({ orden }: Readonly<{ orden: Orden }>) {
  const flujo = flujoFor(useSettings().data);
  // Esperando repuesto es un desvío de En reparación: el stepper la muestra en ese paso.
  const actual = flujo.indexOf(orden.estado === 'esperando_repuesto' ? 'en_reparacion' : orden.estado);
  const horas = new Map<EstadoOrden, string>();
  for (const h of orden.historial ?? []) horas.set(h.hacia, h.ocurridoEn);
  const espera = orden.estado === 'esperando_aprobacion' || orden.estado === 'esperando_repuesto';
  return (
    <ol className="flex flex-wrap gap-2" aria-label="Estados de la orden">
      {flujo.map((e, i) => {
        const hecho = actual >= 0 && i < actual;
        const aqui = i === actual;
        const hora = horas.get(e);
        let tono = 'border border-line bg-white text-ink-2';
        if (hecho) tono = 'bg-primary-tint text-primary';
        else if (aqui) tono = espera ? 'bg-warn-tint text-warn-ink' : 'bg-nav text-ink';
        return (
          <li
            key={e}
            aria-current={aqui ? 'step' : undefined}
            className={`inline-flex min-h-[36px] items-center gap-1.5 rounded-full px-3.5 text-[13px] font-bold ${tono}`}
          >
            {hecho ? <IconCheck size={14} strokeWidth={2.4} /> : null}
            {aqui && espera ? <IconWarn size={14} strokeWidth={2.4} /> : null}
            {estados[e]}
            {/* HU-20: la hora de cada paso se ve, aunque Figma no la dibuje en la ficha. */}
            {hora ? <span className="font-mono text-[11px] font-normal">{formatoHora(hora)}</span> : null}
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
  const mobile = useIsMobile();
  const whatsapp = useFeature('whatsapp'); // V-01: sin WhatsApp no hay conversación en la orden
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
  if (mobile) return <OrderDetailMobile orden={o} />;
  const vehiculo =
    [o.vehiculo?.marca, o.vehiculo?.linea, o.vehiculo?.modeloAnio].filter(Boolean).join(' ') || (o.vehiculo?.tipo === 'moto' ? 'Moto' : 'Carro');
  const avanzada = !['por_recibir', 'recibido', 'cancelada'].includes(o.estado);
  const cotizacion = avanzada ? <QuotePanel orden={o} /> : null;
  const diagnostico = !['por_recibir', 'cancelada'].includes(o.estado) ? <DiagnosisPanel orden={o} /> : null;
  return (
    <>
      <header className="flex flex-wrap items-center gap-3">
        <Link to="/panel/ordenes" aria-label="Volver a órdenes" className="inline-flex h-11 w-11 items-center text-ink hover:text-primary">
          <IconBack size={22} />
        </Link>
        <h1 className="font-mono text-[17px] font-bold text-ink-2">{o.codigo}</h1>
        <Plate value={o.placa} />
        <p className="min-w-0 flex-1 truncate text-[24px] leading-tight font-extrabold tracking-[-.01em]">
          {vehiculo}
          {o.cliente ? ` · ${o.cliente.nombre}` : ''}
        </p>
        <StatePill estado={o.estado} />
      </header>
      <ReceiptAnswer orden={o} />
      {/* Vista previa del aviso de WhatsApp (HU-05): sale con V-01. */}
      {whatsapp && params.get('primera') === '1' ? <ReceivedPreview orden={o} /> : null}
      <StateStepper orden={o} />
      {/* Figma D2: la cotización a la izquierda; a la derecha, lo gastado y el asistente de la orden. */}
      <div className="grid items-start gap-5 xl:grid-cols-[minmax(0,1fr)_420px]">
        <div className="flex min-w-0 flex-col gap-5">
          <section className="flex flex-col gap-2 rounded-card bg-white p-5">
            <h2 className="text-[17px] font-extrabold">Lo que dice el cliente</h2>
            <p className="whitespace-pre-wrap">{o.sintoma}</p>
            <p className="text-[13px] text-ink-2">Abierta el {formatoFechaHora(o.creadoEn)}</p>
            {o.vehiculo?.motorPendiente ? (
              <p className="flex items-center gap-2 rounded-xl bg-warn-tint p-3 text-sm font-bold text-warn-ink">
                <IconWarn size={16} />
                Ficha pendiente: falta combustible, caja o datos del motor
              </p>
            ) : null}
          </section>
          {cotizacion}
          <OrderActions orden={o} />
          {diagnostico}
          <ReceptionPanel orden={o} />
        </div>
        <aside className="flex min-w-0 flex-col gap-5">
          {/* V-08: lo gastado contra lo cotizado, interno del taller. */}
          {avanzada ? <SpendPanel orden={o} /> : null}
          <OrderAssistantCard key={o.id} ordenId={o.id} estado={o.estado} />
          {whatsapp ? <OrderMessages orden={o} /> : null}
        </aside>
      </div>
    </>
  );
}
