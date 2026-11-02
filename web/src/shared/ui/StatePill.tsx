import type { EstadoOrden } from '../api/client';
import { estados } from '../i18n/es-CO';
import { IconCheck, IconClock, IconStop, IconWarn } from './icons';

/** DR-01: todo estado lleva ícono + palabra, nunca solo color. */
const tone: Record<EstadoOrden, { cls: string; icon: 'check' | 'warn' | 'stop' | 'clock' }> = {
  por_recibir: { cls: 'bg-primary-tint text-primary', icon: 'clock' },
  recibido: { cls: 'bg-primary-tint text-primary', icon: 'clock' },
  en_diagnostico: { cls: 'bg-primary-tint text-primary', icon: 'clock' },
  esperando_aprobacion: { cls: 'bg-warn-tint text-warn-ink', icon: 'warn' },
  en_reparacion: { cls: 'bg-primary-tint text-primary', icon: 'clock' },
  listo: { cls: 'bg-ok-tint text-ok-ink', icon: 'check' },
  entregado: { cls: 'bg-ok-tint text-ok-ink', icon: 'check' },
  cancelada: { cls: 'bg-err-tint text-err-ink', icon: 'stop' },
};

export function StatePill({ estado }: { estado: EstadoOrden }) {
  const t = tone[estado];
  const Icon = { check: IconCheck, warn: IconWarn, stop: IconStop, clock: IconClock }[t.icon];
  return (
    <span className={`inline-flex items-center gap-1.5 whitespace-nowrap rounded-full px-2.5 py-0.5 text-[13px] font-bold ${t.cls}`}>
      <Icon size={13} strokeWidth={2.4} />
      {estados[estado]}
    </span>
  );
}
