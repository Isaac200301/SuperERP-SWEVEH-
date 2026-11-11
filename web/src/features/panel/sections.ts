import type { ComponentType } from 'react';

import type { Sesion } from '../../shared/api/client';
import {
  IconCar,
  IconChat,
  IconCheck,
  IconClock,
  IconOrders,
  IconPlus,
  IconTeam,
  IconTool,
  IconWarn,
} from '../../shared/ui/icons';
import { can } from '../auth/api';

export type Section = { to: string; label: string; icon: ComponentType<{ size?: number }>; perm: string; admin?: boolean };

/**
 * Secciones del panel. Solo aparecen las que ya existen y que el rol puede usar
 * (HU-81: el mecánico no ve configuración). Ocultar no protege: el servidor responde 403 igual.
 * Las usan el menú de escritorio (PanelLayout) y la pantalla "Más" del celular (MorePage).
 */
export const sections: Section[] = [
  { to: '/panel/tablero', label: 'Tablero', icon: IconCheck, perm: 'kpi:read' },
  { to: '/panel/piso', label: 'Piso', icon: IconClock, perm: 'order:read' },
  { to: '/panel/ordenes', label: 'Órdenes', icon: IconOrders, perm: 'order:read' },
  // HU-82: dueño y recepción ven los clientes con sus vehículos.
  { to: '/panel/clientes', label: 'Clientes', icon: IconTeam, perm: 'customer:read_all' },
  { to: '/panel/por-revisar', label: 'Por revisar', icon: IconWarn, perm: 'inbox:manage' },
  { to: '/panel/conversaciones', label: 'Conversaciones', icon: IconChat, perm: 'conversation:read' },
  // HU-82: la configuración de los avisos es solo del dueño; recepción y mecánico no la ven.
  { to: '/panel/automatizaciones', label: 'Automatizaciones', icon: IconPlus, perm: 'automation:manage' },
  { to: '/panel/servicios', label: 'Servicios', icon: IconTool, perm: 'service:manage' },
  { to: '/panel/repuestos', label: 'Repuestos', icon: IconOrders, perm: 'part:manage' },
  { to: '/panel/equipo', label: 'Equipo', icon: IconTeam, perm: 'user:manage' },
  { to: '/panel/taller', label: 'Mi taller', icon: IconCar, perm: 'workshop:manage' },
  // HU-74: solo el administrador del ERP; no es un permiso de taller (ADR-0030).
  { to: '/panel/plataforma', label: 'Plataforma', icon: IconCheck, perm: '', admin: true },
];

/** soloYo: el taller dijo "solo yo" en el registro y no ve la gestión de usuarios (HU-83). */
export function sectionsFor(session: Sesion, soloYo: boolean): Section[] {
  return sections.filter(
    (n) => (n.admin ? session.adminPlataforma === true : can(session, n.perm)) && !(soloYo && n.to === '/panel/equipo'),
  );
}
