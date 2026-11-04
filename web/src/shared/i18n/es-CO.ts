import type { EstadoOrden } from '../api/client';

/** Textos de la interfaz en español de Colombia (docs/08 §3). */
export const estados: Record<EstadoOrden, string> = {
  por_recibir: 'Por recibir',
  recibido: 'Recibido',
  en_diagnostico: 'En diagnóstico',
  esperando_aprobacion: 'Esperando aprobación',
  en_reparacion: 'En reparación',
  esperando_repuesto: 'Esperando repuesto',
  control_calidad: 'Control de calidad',
  listo: 'Listo',
  entregado: 'Entregado',
  cancelada: 'Cancelada',
};

/** Los 7 estados del flujo, en orden (Q-04). Cancelada va fuera del flujo (Q-05). */
export const flujo: EstadoOrden[] = [
  'por_recibir',
  'recibido',
  'en_diagnostico',
  'esperando_aprobacion',
  'en_reparacion',
  'listo',
  'entregado',
];

/** El flujo del taller según sus parámetros: Control de calidad va entre En reparación y Listo si está activo.
 *  Esperando repuesto no es un paso: es un desvío desde En reparación (ver OrderActions). */
export function flujoFor(p?: { usarControlCalidad: boolean } | null): EstadoOrden[] {
  if (!p?.usarControlCalidad) return flujo;
  return flujo.flatMap((e) => (e === 'listo' ? (['control_calidad', 'listo'] as EstadoOrden[]) : [e]));
}

export const roles: Record<string, string> = {
  dueno: 'Dueño',
  recepcion: 'Recepción',
  mecanico: 'Mecánico',
};
