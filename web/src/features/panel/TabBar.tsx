import type { ComponentType } from 'react';
import { NavLink } from 'react-router';

import type { Sesion } from '../../shared/api/client';
import { IconChat, IconGrid, IconHome, IconOrders } from '../../shared/ui/icons';
import { can } from '../auth/api';

export const nuevaOrdenEvento = 'sweveh:nueva-orden';

/** Abre la hoja "Nueva orden" desde cualquier pantalla del celular (la escucha MobileShell). */
export function abrirNuevaOrden() {
  window.dispatchEvent(new Event(nuevaOrdenEvento));
}

type TabDef = { to: string; label: string; icon: ComponentType<{ size?: number }>; end?: boolean };

/**
 * Pestañas de la barra inferior (entrega del 27-nov, V-05 y V-11; Figma "04 Hoy"): Hoy · Órdenes · Asistente · Taller
 * para todos los roles. "Hoy" es el tablero para quien ve los números (kpi:read) y el piso para el resto.
 * La orden nueva se abre desde Órdenes. Ocultar no protege: el servidor responde 403 igual (R-05).
 */
export function tabsFor(session: Sesion): TabDef[] {
  const hoy = can(session, 'kpi:read') ? '/panel/tablero' : '/panel/piso';
  return [
    { to: hoy, label: 'Hoy', icon: IconHome, end: true },
    { to: '/panel/ordenes', label: 'Órdenes', icon: IconOrders },
    { to: '/panel/asistente', label: 'Asistente', icon: IconChat },
    { to: '/panel/mas', label: 'Taller', icon: IconGrid },
  ];
}

const tab = 'flex min-h-[56px] flex-1 flex-col items-center justify-center gap-0.5 text-[11px] font-bold';

/** Barra inferior fija del celular. El activo va en lima (DR-02) con aria-current="page" (DR-14). */
export function TabBar({ session }: { session: Sesion }) {
  return (
    <nav
      aria-label="Secciones"
      className="fixed inset-x-0 bottom-0 z-40 flex items-end border-t border-[#E3EAE8] bg-white px-1.5 pt-1 pb-[calc(10px+env(safe-area-inset-bottom))]"
    >
      {tabsFor(session).map(t => (
        <NavLink key={t.to} to={t.to} end={t.end ?? false} className={({ isActive }) => `${tab} ${isActive ? 'text-ink' : 'text-ink-2'}`}>
          {({ isActive }) => (
            <>
              <i className={`relative grid h-[30px] w-[52px] place-items-center rounded-full not-italic ${isActive ? 'bg-nav' : ''}`}>
                <t.icon size={22} />
              </i>
              {t.label}
            </>
          )}
        </NavLink>
      ))}
    </nav>
  );
}
