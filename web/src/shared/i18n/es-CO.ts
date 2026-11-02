import type { EstadoOrden } from '../api/client';

/** Textos de la interfaz en español de Colombia (docs/08 §3). */
export const estados: Record<EstadoOrden, string> = {
  por_recibir: 'Por recibir',
  recibido: 'Recibido',
  en_diagnostico: 'En diagnóstico',
  esperando_aprobacion: 'Esperando aprobación',
  en_reparacion: 'En reparación',
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

export const roles: Record<string, string> = {
  dueno: 'Dueño',
  recepcion: 'Recepción',
  mecanico: 'Mecánico',
};
