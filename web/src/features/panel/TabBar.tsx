import type { ComponentType } from 'react';
import { NavLink } from 'react-router';

import type { Sesion } from '../../shared/api/client';
import { IconGrid, IconHome, IconInbox, IconOrders, IconPlus, IconUser } from '../../shared/ui/icons';
import { can } from '../auth/api';

export const nuevaOrdenEvento = 'sweveh:nueva-orden';

/** Abre la hoja "Nueva orden" desde cualquier pantalla del celular (la escucha MobileShell). */
export function abrirNuevaOrden() {
  window.dispatchEvent(new Event(nuevaOrdenEvento));
}

type TabDef =
  | { kind: 'link'; to: string; label: string; icon: ComponentType<{ size?: number }>; badge?: boolean }
  | { kind: 'new'; label: string };

/**
 * Pestañas de la barra inferior según lo que el rol puede hacer (mockups móviles v1, docs/10 §4.7):
 * - dueño: Hoy · Órdenes · + Nueva · Revisar · Taller
 * - recepción (sin números): Órdenes · + Nueva · Revisar · Más
 * - mecánico (Piso): Órdenes · Yo
 * Ocultar no protege: el servidor responde 403 igual (R-05).
 */
export function tabsFor(session: Sesion): TabDef[] {
  if (!can(session, 'order:create')) {
    return [
      { kind: 'link', to: '/panel/piso', label: 'Órdenes', icon: IconOrders },
      { kind: 'link', to: '/panel/mas', label: 'Yo', icon: IconUser },
    ];
  }
  const tabs: TabDef[] = [];
  const numeros = can(session, 'kpi:read');
  if (numeros) tabs.push({ kind: 'link', to: '/panel/tablero', label: 'Hoy', icon: IconHome });
  tabs.push({ kind: 'link', to: '/panel/ordenes', label: 'Órdenes', icon: IconOrders });
  tabs.push({ kind: 'new', label: 'Nueva' });
  if (can(session, 'inbox:manage')) tabs.push({ kind: 'link', to: '/panel/por-revisar', label: 'Revisar', icon: IconInbox, badge: true });
  tabs.push({ kind: 'link', to: '/panel/mas', label: numeros ? 'Taller' : 'Más', icon: IconGrid });
  return tabs;
}

const tab = 'flex min-h-[56px] flex-1 flex-col items-center justify-center gap-0.5 text-[11px] font-bold';

/** Barra inferior fija del celular. El activo va en lima (DR-02) con aria-current="page" (DR-14). */
export function TabBar({ session, pending }: { session: Sesion; pending: number }) {
  return (
    <nav
      aria-label="Secciones"
      className="fixed inset-x-0 bottom-0 z-40 flex items-end border-t border-[#E3EAE8] bg-white px-1.5 pt-1 pb-[calc(10px+env(safe-area-inset-bottom))]"
    >
      {tabsFor(session).map((t) =>
        t.kind === 'new' ? (
          <button key="nueva" type="button" aria-label="Nueva orden" onClick={abrirNuevaOrden} className={`${tab} text-primary`}>
            <i className="-mt-[22px] grid h-14 w-14 place-items-center rounded-[20px] bg-primary not-italic text-white shadow-[0_6px_16px_rgba(0,95,84,.35)]">
              <IconPlus size={26} strokeWidth={2.6} />
            </i>
            {t.label}
          </button>
        ) : (
          <NavLink key={t.to} to={t.to} className={({ isActive }) => `${tab} ${isActive ? 'text-ink' : 'text-ink-2'}`}>
            {({ isActive }) => (
              <>
                <i className={`relative grid h-[30px] w-[52px] place-items-center rounded-full not-italic ${isActive ? 'bg-nav' : ''}`}>
                  <t.icon size={22} />
                  {t.badge && pending > 0 ? (
                    <span className="absolute -top-1.5 right-1 grid h-5 min-w-5 place-items-center rounded-full border-2 border-white bg-primary px-1 text-[11px] font-extrabold text-white">
                      {pending}
                      <span className="sr-only"> por revisar</span>
                    </span>
                  ) : null}
                </i>
                {t.label}
              </>
            )}
          </NavLink>
        ),
      )}
    </nav>
  );
}
